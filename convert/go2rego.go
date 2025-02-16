package convert

import (
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/shopspring/decimal"
)

// GoToRego converts a Go interface{} value to a Rego ast.Value.
func GoToRego(x interface{}) (ast.Value, error) {
	if x == nil {
		return ast.NullTerm().Value, nil
	}
	rv := reflect.ValueOf(x)
	return goValueToAst(rv)
}

// goValueToAst recursively converts a reflect.Value to a Rego ast.Value.
func goValueToAst(rv reflect.Value) (ast.Value, error) {
	if !rv.IsValid() {
		return ast.NullTerm().Value, nil
	}

	rt := rv.Type()

	// Handle decimal.NullDecimal
	if rt == reflect.TypeOf(decimal.NullDecimal{}) {
		nd := rv.Interface().(decimal.NullDecimal)
		if !nd.Valid {
			return ast.NullTerm().Value, nil
		}
		return ast.NumberTerm(json.Number(nd.Decimal.String())).Value, nil
	}

	// Handle decimal.Decimal
	if rt == reflect.TypeOf(decimal.Decimal{}) {
		dec := rv.Interface().(decimal.Decimal)
		return ast.NumberTerm(json.Number(dec.String())).Value, nil
	}

	// Handle time.Time
	if rt == reflect.TypeOf(time.Time{}) {
		t := rv.Interface().(time.Time)
		return ast.StringTerm(t.Format(DefaultTimeFormat)).Value, nil
	}

	// Handle basic scalar types
	switch rt.Kind() {
	case reflect.Bool:
		return ast.BooleanTerm(rv.Bool()).Value, nil
	case reflect.String:
		return ast.StringTerm(rv.String()).Value, nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return ast.IntNumberTerm(int(rv.Int())).Value, nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return ast.UIntNumberTerm(rv.Uint()).Value, nil
	case reflect.Float32, reflect.Float64:
		return ast.FloatNumberTerm(rv.Float()).Value, nil
	case reflect.Interface:
		if rv.IsNil() {
			return ast.NullTerm().Value, nil
		}
		return goValueToAst(rv.Elem())
	}

	// Handle pointers
	if rt.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return ast.NullTerm().Value, nil
		}
		return goValueToAst(rv.Elem())
	}

	// Handle slices and arrays
	if rt.Kind() == reflect.Slice || rt.Kind() == reflect.Array {
		arr := ast.NewArray()
		length := rv.Len()
		for i := 0; i < length; i++ {
			elemVal, err := goValueToAst(rv.Index(i))
			if err != nil {
				return nil, err
			}
			arr = arr.Append(ast.NewTerm(elemVal))
		}
		return arr, nil
	}

	// Handle maps
	if rt.Kind() == reflect.Map {
		obj := ast.NewObject()
		iter := rv.MapRange()
		for iter.Next() {
			kVal := iter.Key()
			vVal := iter.Value()

			if kVal.Kind() != reflect.String {
				return nil, fmt.Errorf("map key must be string type (got %v)", kVal.Kind())
			}
			convertedVal, err := goValueToAst(vVal)
			if err != nil {
				return nil, err
			}
			obj.Insert(ast.StringTerm(kVal.String()), ast.NewTerm(convertedVal))
		}
		return obj, nil
	}

	// Handle structs
	if rt.Kind() == reflect.Struct {
		obj := ast.NewObject()
		nFields := rt.NumField()

		for i := 0; i < nFields; i++ {
			sf := rt.Field(i)
			if !sf.IsExported() {
				continue
			}
			fieldRV := rv.Field(i)
			key := getStructFieldKey(sf)
			if key == "" {
				continue
			}
			valAst, err := goValueToAst(fieldRV)
			if err != nil {
				return nil, fmt.Errorf("error converting field '%s': %w", sf.Name, err)
			}
			obj.Insert(ast.StringTerm(key), ast.NewTerm(valAst))
		}
		return obj, nil
	}

	return nil, fmt.Errorf("unsupported Go type %v", rt)
}
