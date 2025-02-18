package convert

import (
	"fmt"
	"math"
	"reflect"
	"strconv"
	"time"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/shopspring/decimal"
)

// RegoToGo converts a Rego ast.Value to a Go type [T].
// Special handling for time.Time, decimal.Decimal, etc. is done before general struct logic.
func RegoToGo[T any](val ast.Value) (T, error) {
	var zero T
	rt := reflect.TypeOf(zero)
	if rt == nil {
		return zero, fmt.Errorf("no type information available")
	}

	rv := reflect.ValueOf(&zero).Elem()

	// Handle time.Time as a single-value string parse.
	if rt == reflect.TypeOf(time.Time{}) {
		if err := assignRegoValueToGo(val, rv); err != nil {
			return zero, err
		}
		return rv.Interface().(T), nil
	}

	// Handle decimal.Decimal as a single-value numeric parse.
	if rt == reflect.TypeOf(decimal.Decimal{}) {
		if err := assignRegoValueToGo(val, rv); err != nil {
			return zero, err
		}
		return rv.Interface().(T), nil
	}

	// Handle other kinds (general struct, slice, map, etc.).
	switch rt.Kind() {
	case reflect.Struct:
		// Convert ast.Object to a Go struct.
		if err := convertRegoObjectToStruct(val, rv); err != nil {
			return zero, err
		}
		return rv.Interface().(T), nil

	case reflect.Slice, reflect.Array:
		// Slice or array
		if err := convertRegoArrayToSlice(val, rv); err != nil {
			return zero, err
		}
		return rv.Interface().(T), nil
	case reflect.Map:
		// Map
		if err := convertRegoObjectOrSetToMap(val, rv); err != nil {
			return zero, err
		}
		return rv.Interface().(T), nil
	case reflect.Ptr:
		if _, isNull := val.(ast.Null); isNull {
			// => rv.Set(nil)
			rv.Set(reflect.Zero(rt)) // nil pointer
			return rv.Interface().(T), nil
		}

		// If it's a pointer, recursively handle the element
		ptrTarget := rt.Elem()
		newVal := reflect.New(ptrTarget)
		if err := assignRegoValueToGo(val, newVal.Elem()); err != nil {
			return zero, err
		}
		return newVal.Interface().(T), nil
	}

	// Otherwise, handle scalar types, decimal, etc.
	if err := assignRegoValueToGo(val, rv); err != nil {
		return zero, err
	}
	return rv.Interface().(T), nil
}

// assignRegoValueToGo maps a Rego ast.Value to a Go value (scalar, decimal, etc.).
func assignRegoValueToGo(aVal ast.Value, rv reflect.Value) error {
	rt := rv.Type()

	// Handle decimal.NullDecimal first
	if rt == reflect.TypeOf(decimal.NullDecimal{}) {
		return convertRegoToNullDecimal(aVal, rv)
	}
	if rt.Kind() == reflect.Ptr && rt.Elem() == reflect.TypeOf(decimal.NullDecimal{}) {
		if rv.IsNil() {
			rv.Set(reflect.New(rt.Elem()))
		}
		return convertRegoToNullDecimal(aVal, rv.Elem())
	}

	// If it's null, set zero values for pointers/slices/maps/interfaces or return an error for value types
	if _, isNull := aVal.(ast.Null); isNull {
		switch rt.Kind() {
		case reflect.Ptr, reflect.Slice, reflect.Map, reflect.Interface:
			rv.Set(reflect.Zero(rt))
			return nil
		default:
			return fmt.Errorf("null is not allowed for type %v", rt)
		}
	}

	// Handle decimal.Decimal (no null allowed)
	if rt == reflect.TypeOf(decimal.Decimal{}) {
		return convertRegoNumberToDecimal(aVal, rv)
	}
	if rt.Kind() == reflect.Ptr && rt.Elem() == reflect.TypeOf(decimal.Decimal{}) {
		if rv.IsNil() {
			rv.Set(reflect.New(rt.Elem()))
		}
		return convertRegoNumberToDecimal(aVal, rv.Elem())
	}

	// Handle time.Time
	if rt == reflect.TypeOf(time.Time{}) {
		s, ok := aVal.(ast.String)
		if !ok {
			return fmt.Errorf("time.Time conversion error: expected ast.String (got %T)", aVal)
		}
		parsed, err := time.Parse(DefaultTimeFormat, string(s))
		if err != nil {
			return fmt.Errorf("time.Time parsing error: %w", err)
		}
		rv.Set(reflect.ValueOf(parsed))
		return nil
	}

	// Handle *time.Time
	if rt.Kind() == reflect.Ptr && rt.Elem() == reflect.TypeOf(time.Time{}) {
		if rv.IsNil() {
			rv.Set(reflect.New(rt.Elem()))
		}
		s, ok := aVal.(ast.String)
		if !ok {
			return fmt.Errorf("*time.Time conversion error: expected ast.String (got %T)", aVal)
		}
		parsed, err := time.Parse(DefaultTimeFormat, string(s))
		if err != nil {
			return fmt.Errorf("*time.Time parsing error: %w", err)
		}
		rv.Elem().Set(reflect.ValueOf(parsed))
		return nil
	}

	// Handle basic scalar types
	switch rt.Kind() {
	case reflect.String:
		s, ok := aVal.(ast.String)
		if !ok {
			return fmt.Errorf("string conversion error: expected ast.String (got %T)", aVal)
		}
		rv.SetString(string(s))
		return nil

	case reflect.Bool:
		b, ok := aVal.(ast.Boolean)
		if !ok {
			return fmt.Errorf("bool conversion error: expected ast.Boolean (got %T)", aVal)
		}
		rv.SetBool(bool(b))
		return nil

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		n, ok := aVal.(ast.Number)
		if !ok {
			return fmt.Errorf("number conversion error: expected ast.Number (got %T)", aVal)
		}
		return parseRegoNumberInto(rv, string(n))

	case reflect.Struct:
		obj, ok := aVal.(ast.Object)
		if !ok {
			return fmt.Errorf("struct conversion error: expected ast.Object for type %v, got %T", rt, aVal)
		}
		return convertRegoObjectToStruct(obj, rv)

	case reflect.Interface:
		val, err := convertRegoValueToInterface(aVal)
		if err != nil {
			return err
		}
		rv.Set(reflect.ValueOf(val))
		return nil
	}

	return fmt.Errorf("assignRegoValueToGo: unsupported type %v", rt)
}

// convertRegoToNullDecimal maps a Rego ast.Value to a decimal.NullDecimal.
func convertRegoToNullDecimal(aVal ast.Value, rv reflect.Value) error {
	if _, isNull := aVal.(ast.Null); isNull {
		var nd decimal.NullDecimal
		nd.Valid = false
		rv.Set(reflect.ValueOf(nd))
		return nil
	}

	numVal, ok := aVal.(ast.Number)
	if !ok {
		return fmt.Errorf("decimal.NullDecimal conversion error: expected ast.Number (got %T)", aVal)
	}
	dec, err := decimal.NewFromString(string(numVal))
	if err != nil {
		return fmt.Errorf("decimal.NullDecimal parse error: %w", err)
	}
	rv.Set(reflect.ValueOf(decimal.NullDecimal{
		Decimal: dec,
		Valid:   true,
	}))
	return nil
}

// convertRegoNumberToDecimal maps a Rego ast.Number to a decimal.Decimal.
func convertRegoNumberToDecimal(aVal ast.Value, rv reflect.Value) error {
	num, ok := aVal.(ast.Number)
	if !ok {
		return fmt.Errorf("decimal.Decimal conversion error: expected ast.Number (got %T)", aVal)
	}
	dec, err := decimal.NewFromString(string(num))
	if err != nil {
		return fmt.Errorf("decimal.Decimal parse error: %w", err)
	}
	rv.Set(reflect.ValueOf(dec))
	return nil
}

// convertRegoObjectToStruct maps a Rego ast.Object to a Go struct.
func convertRegoObjectToStruct(aVal ast.Value, rv reflect.Value) error {
	obj, ok := aVal.(ast.Object)
	if !ok {
		return fmt.Errorf("struct conversion error: expected ast.Object (got %T)", aVal)
	}
	rt := rv.Type()
	if rt.Kind() != reflect.Struct {
		return fmt.Errorf("target is not a struct")
	}

	nFields := rt.NumField()
	for i := 0; i < nFields; i++ {
		sf := rt.Field(i)
		if !sf.IsExported() {
			continue
		}
		key := getStructFieldKey(sf)

		term := obj.Get(ast.StringTerm(key))
		if term == nil {
			continue
		}

		fv := rv.Field(i)
		if !fv.CanSet() {
			continue
		}
		if err := assignRegoValueToGo(term.Value, fv); err != nil {
			return fmt.Errorf("error converting field '%s': %w", sf.Name, err)
		}
	}
	return nil
}

// convertRegoArrayToSlice maps a Rego ast.Array to a Go slice or array.
func convertRegoArrayToSlice(aVal ast.Value, rv reflect.Value) error {
	arr, ok := aVal.(*ast.Array)
	if !ok {
		return fmt.Errorf("slice/array conversion error: expected ast.Array (got %T)", aVal)
	}
	length := arr.Len()

	switch rv.Kind() {
	case reflect.Array:
		if length > rv.Len() {
			return fmt.Errorf("Rego array length %d exceeds Go array length %d", length, rv.Len())
		}
	case reflect.Slice:
		rv.Set(reflect.MakeSlice(rv.Type(), length, length))
	default:
		return fmt.Errorf("target is not a slice or array")
	}

	var iterationErr error
	i := 0
	arr.Foreach(func(elem *ast.Term) {
		if iterationErr != nil {
			return
		}
		if err := assignRegoValueToGo(elem.Value, rv.Index(i)); err != nil {
			iterationErr = fmt.Errorf("error converting array element %d: %w", i, err)
			return
		}
		i++
	})
	if iterationErr != nil {
		return iterationErr
	}

	return nil
}

// convertRegoObjectOrSetToMap converts a Rego ast.Object or ast.Set to a Go map.
// If aVal is an ast.Set, it only converts to map[string]struct{}.
// Otherwise, aVal must be an ast.Object.
func convertRegoObjectOrSetToMap(aVal ast.Value, rv reflect.Value) error {

	if setVal, ok := aVal.(ast.Set); ok {
		// map[string]struct{} 변환 로직
		if rv.IsNil() {
			rv.Set(reflect.MakeMap(rv.Type()))
		}
		keyType := rv.Type().Key()
		elemType := rv.Type().Elem()

		if keyType.Kind() != reflect.String || elemType != reflect.TypeOf(struct{}{}) {
			return fmt.Errorf("ast.Set can only convert to map[string]struct{}, got map[%v]%v", keyType, elemType)
		}

		var iterationErr error
		setVal.Foreach(func(elem *ast.Term) {
			if iterationErr != nil {
				return
			}
			s, ok := elem.Value.(ast.String)
			if !ok {
				iterationErr = fmt.Errorf("ast.Set element is not ast.String (got %T)", elem.Value)
				return
			}
			rv.SetMapIndex(reflect.ValueOf(string(s)), reflect.ValueOf(struct{}{}))
		})
		return iterationErr
	}

	obj, ok := aVal.(ast.Object)
	if !ok {
		return fmt.Errorf("map conversion error: expected ast.Object (got %T)", aVal)
	}
	if rv.IsNil() {
		rv.Set(reflect.MakeMap(rv.Type()))
	}
	keyType := rv.Type().Key()
	elemType := rv.Type().Elem()

	var iterationErr error
	obj.Foreach(func(k, val *ast.Term) {
		if iterationErr != nil {
			return
		}
		if _, ok := k.Value.(ast.String); !ok {
			iterationErr = fmt.Errorf("map key is not ast.String (got %T)", k.Value)
			return
		}

		newKey := reflect.New(keyType).Elem()
		if err := assignRegoValueToGo(k.Value, newKey); err != nil {
			iterationErr = fmt.Errorf("error converting map key: %w", err)
			return
		}

		newVal := reflect.New(elemType).Elem()
		if err := assignRegoValueToGo(val.Value, newVal); err != nil {
			iterationErr = fmt.Errorf("error converting map value (key=%v): %w", newKey.Interface(), err)
			return
		}
		rv.SetMapIndex(newKey, newVal)
	})
	if iterationErr != nil {
		return iterationErr
	}

	return nil
}

// parseRegoNumberInto parses the string s into the numeric type of rv.
func parseRegoNumberInto(rv reflect.Value, s string) error {
	kind := rv.Kind()
	switch kind {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		i64, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return fmt.Errorf("error parsing integer: %w", err)
		}
		switch kind {
		case reflect.Int:
			rv.SetInt(i64)
		case reflect.Int8:
			if i64 < math.MinInt8 || i64 > math.MaxInt8 {
				return fmt.Errorf("int8 out of range %d", i64)
			}
			rv.SetInt(i64)
		case reflect.Int16:
			if i64 < math.MinInt16 || i64 > math.MaxInt16 {
				return fmt.Errorf("int16 out of range %d", i64)
			}
			rv.SetInt(i64)
		case reflect.Int32:
			if i64 < math.MinInt32 || i64 > math.MaxInt32 {
				return fmt.Errorf("int32 out of range %d", i64)
			}
			rv.SetInt(i64)
		case reflect.Int64:
			rv.SetInt(i64)
		}
		return nil

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		u64, err := strconv.ParseUint(s, 10, 64)
		if err != nil {
			return fmt.Errorf("error parsing unsigned integer: %w", err)
		}
		switch kind {
		case reflect.Uint:
			rv.SetUint(u64)
		case reflect.Uint8:
			if u64 > math.MaxUint8 {
				return fmt.Errorf("uint8 out of range %d", u64)
			}
			rv.SetUint(u64)
		case reflect.Uint16:
			if u64 > math.MaxUint16 {
				return fmt.Errorf("uint16 out of range %d", u64)
			}
			rv.SetUint(u64)
		case reflect.Uint32:
			if u64 > math.MaxUint32 {
				return fmt.Errorf("uint32 out of range %d", u64)
			}
			rv.SetUint(u64)
		case reflect.Uint64:
			rv.SetUint(u64)
		}
		return nil

	case reflect.Float32, reflect.Float64:
		f64, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return fmt.Errorf("error parsing float: %w", err)
		}
		if kind == reflect.Float32 {
			rv.SetFloat(float64(float32(f64)))
		} else {
			rv.SetFloat(f64)
		}
		return nil
	}
	return fmt.Errorf("unsupported numeric type %v", kind)
}

// convertRegoValueToInterface converts a Rego ast.Value to an interface{}.
func convertRegoValueToInterface(aVal ast.Value) (interface{}, error) {
	switch v := aVal.(type) {
	case ast.Null:
		return nil, nil
	case ast.Boolean:
		return bool(v), nil
	case ast.String:
		return string(v), nil
	case ast.Number:
		f, ok := v.Float64()
		if !ok {
			return nil, fmt.Errorf("error converting ast.Number to float64")
		}
		return f, nil
	case *ast.Array:
		result := make([]interface{}, 0, v.Len())
		var iterationErr error
		v.Foreach(func(elem *ast.Term) {
			if iterationErr != nil {
				return
			}
			val, err := convertRegoValueToInterface(elem.Value)
			if err != nil {
				iterationErr = err
				return
			}
			result = append(result, val)
		})
		if iterationErr != nil {
			return nil, iterationErr
		}
		return result, nil
	case ast.Object:
		result := make(map[string]interface{})
		var iterationErr error
		v.Foreach(func(k, val *ast.Term) {
			if iterationErr != nil {
				return
			}
			strKey, ok := k.Value.(ast.String)
			if !ok {
				iterationErr = fmt.Errorf("object key is not ast.String (got %T)", k.Value)
				return
			}
			mVal, err := convertRegoValueToInterface(val.Value)
			if err != nil {
				iterationErr = fmt.Errorf("object value conversion error (key=%s): %w", strKey, err)
				return
			}
			result[string(strKey)] = mVal
		})
		if iterationErr != nil {
			return nil, iterationErr
		}
		return result, nil
	}
	return nil, fmt.Errorf("unsupported ast.Value type %T", aVal)
}
