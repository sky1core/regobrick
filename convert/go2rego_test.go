package convert_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/shopspring/decimal"

	. "github.com/sky1core/regobrick/convert"
)

func TestGoToRego(t *testing.T) {
	t.Run("Nil", func(t *testing.T) {
		val, err := GoToRego(nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, ok := val.(ast.Null); !ok {
			t.Fatalf("expected ast.Null, got %T", val)
		}
	})

	t.Run("Bool", func(t *testing.T) {
		val, err := GoToRego(true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		boolVal, ok := val.(ast.Boolean)
		if !ok {
			t.Fatalf("expected ast.Boolean, got %T", val)
		}
		if boolVal != ast.Boolean(true) {
			t.Fatalf("expected true, got %v", boolVal)
		}
	})

	t.Run("String", func(t *testing.T) {
		val, err := GoToRego("hello")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		strVal, ok := val.(ast.String)
		if !ok {
			t.Fatalf("expected ast.String, got %T", val)
		}
		if string(strVal) != "hello" {
			t.Fatalf("expected \"hello\", got %q", strVal)
		}
	})

	t.Run("Int", func(t *testing.T) {
		val, err := GoToRego(42)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		numVal, ok := val.(ast.Number)
		if !ok {
			t.Fatalf("expected ast.Number, got %T", val)
		}
		if numVal.String() != "42" {
			t.Fatalf("expected 42, got %v", numVal)
		}
	})

	t.Run("Float", func(t *testing.T) {
		val, err := GoToRego(3.14)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		numVal, ok := val.(ast.Number)
		if !ok {
			t.Fatalf("expected ast.Number, got %T", val)
		}
		if numVal.String() != "3.14" {
			t.Fatalf("expected 3.14, got %v", numVal)
		}
	})

	t.Run("decimal.Decimal", func(t *testing.T) {
		dec := decimal.NewFromFloat(123.456)
		val, err := GoToRego(dec)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		numVal, ok := val.(ast.Number)
		if !ok {
			t.Fatalf("expected ast.Number, got %T", val)
		}
		if numVal.String() != "123.456" {
			t.Fatalf("expected 123.456, got %s", numVal)
		}
	})

	t.Run("decimal.NullDecimal valid", func(t *testing.T) {
		nd := decimal.NullDecimal{
			Decimal: decimal.NewFromInt(99),
			Valid:   true,
		}
		val, err := GoToRego(nd)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		numVal, ok := val.(ast.Number)
		if !ok {
			t.Fatalf("expected ast.Number, got %T", val)
		}
		if numVal.String() != "99" {
			t.Fatalf("expected 99, got %s", numVal)
		}
	})

	t.Run("decimal.NullDecimal null", func(t *testing.T) {
		nd := decimal.NullDecimal{Valid: false}
		val, err := GoToRego(nd)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, ok := val.(ast.Null); !ok {
			t.Fatalf("expected ast.Null, got %T", val)
		}
	})

	t.Run("time.Time", func(t *testing.T) {
		// convert.DefaultTimeFormat을 가정 (예: RFC3339)
		tm, _ := time.Parse(time.RFC3339, "2025-02-16T12:34:56Z")
		val, err := GoToRego(tm)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		strVal, ok := val.(ast.String)
		if !ok {
			t.Fatalf("expected ast.String, got %T", val)
		}
		// Check the actual string
		if string(strVal) != tm.Format(DefaultTimeFormat) {
			t.Fatalf("expected %q, got %q", tm.Format(DefaultTimeFormat), strVal)
		}
	})

	t.Run("*time.Time nil", func(t *testing.T) {
		var tm *time.Time = nil
		val, err := GoToRego(tm)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, ok := val.(ast.Null); !ok {
			t.Fatalf("expected ast.Null, got %T", val)
		}
	})

	t.Run("*time.Time valid", func(t *testing.T) {
		tmVal := time.Date(2025, 2, 16, 12, 34, 56, 0, time.UTC)
		tmPtr := &tmVal
		val, err := GoToRego(tmPtr)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		strVal, ok := val.(ast.String)
		if !ok {
			t.Fatalf("expected ast.String, got %T", val)
		}
		want := tmVal.Format(DefaultTimeFormat)
		if string(strVal) != want {
			t.Fatalf("expected %q, got %q", want, strVal)
		}
	})

	t.Run("Slice of strings", func(t *testing.T) {
		input := []string{"alpha", "beta", "gamma"}
		val, err := GoToRego(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		arr, ok := val.(*ast.Array)
		if !ok {
			t.Fatalf("expected ast.Array, got %T", val)
		}
		if arr.Len() != 3 {
			t.Fatalf("expected length 3, got %d", arr.Len())
		}
	})

	t.Run("Map[string]string", func(t *testing.T) {
		input := map[string]string{
			"key1": "val1",
			"key2": "val2",
		}
		val, err := GoToRego(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		obj, ok := val.(ast.Object)
		if !ok {
			t.Fatalf("expected ast.Object, got %T", val)
		}
		if obj.Len() != 2 {
			t.Fatalf("expected length 2, got %d", obj.Len())
		}
	})

	t.Run("Map with non-string key should error", func(t *testing.T) {
		input := map[int]string{1: "val1"}
		_, err := GoToRego(input)
		if err == nil {
			t.Fatal("expected error for non-string key, got nil")
		}
	})

	t.Run("Struct", func(t *testing.T) {
		type Person struct {
			Name string
			Age  int
		}
		p := Person{Name: "Alice", Age: 30}
		val, err := GoToRego(p)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		obj, ok := val.(ast.Object)
		if !ok {
			t.Fatalf("expected ast.Object, got %T", val)
		}
		if obj.Len() != 2 {
			t.Fatalf("expected length 2, got %d", obj.Len())
		}
		nameTerm := obj.Get(ast.StringTerm("Name"))
		if nameTerm == nil {
			t.Fatal("expected 'Name' key in object")
		}
		ageTerm := obj.Get(ast.StringTerm("Age"))
		if ageTerm == nil {
			t.Fatal("expected 'Age' key in object")
		}
	})

	t.Run("Pointer to struct", func(t *testing.T) {
		type Person struct {
			Name string
			Age  int
		}
		p := &Person{Name: "Bob", Age: 25}
		val, err := GoToRego(p)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		obj, ok := val.(ast.Object)
		if !ok {
			t.Fatalf("expected ast.Object, got %T", val)
		}
		if obj.Len() != 2 {
			t.Fatalf("expected length 2, got %d", obj.Len())
		}
	})

	t.Run("Unknown struct field keys (unexported) are ignored", func(t *testing.T) {
		type Hidden struct {
			Public  string
			private int
		}
		h := Hidden{Public: "visible", private: 42}
		val, err := GoToRego(h)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		obj, ok := val.(ast.Object)
		if !ok {
			t.Fatalf("expected ast.Object, got %T", val)
		}
		if obj.Len() != 1 {
			t.Fatalf("expected 1 field (Public), got %d", obj.Len())
		}
		if obj.Get(ast.StringTerm("Public")) == nil {
			t.Fatal("expected 'Public' key")
		}
	})

	t.Run("Interface nil", func(t *testing.T) {
		var someVal interface{} = nil
		val, err := GoToRego(someVal)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, ok := val.(ast.Null); !ok {
			t.Fatalf("expected ast.Null, got %T", val)
		}
	})

	t.Run("Interface value", func(t *testing.T) {
		var iface interface{} = "some string"
		val, err := GoToRego(iface)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		strVal, ok := val.(ast.String)
		if !ok {
			t.Fatalf("expected ast.String, got %T", val)
		}
		if strVal != ast.String("some string") {
			t.Fatalf("expected 'some string', got %v", strVal)
		}
	})

	t.Run("map[string]struct{} => ast.Set", func(t *testing.T) {
		input := map[string]struct{}{
			"foo": {},
			"bar": {},
		}
		val, err := GoToRego(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// 변환 결과가 ast.Set인지 확인
		setVal, ok := val.(ast.Set)
		if !ok {
			t.Fatalf("expected ast.Set, got %T", val)
		}

		// Set 내부 요소가 "foo", "bar" 문자열인지 검사
		if setVal.Len() != 2 {
			t.Fatalf("expected 2 elements in set, got %d", setVal.Len())
		}

		for _, want := range []string{"foo", "bar"} {
			if !setVal.Contains(ast.StringTerm(want)) {
				t.Fatalf("expected set to contain %q", want)
			}
		}
	})

	t.Run("map[string]struct{} empty => ast.Set", func(t *testing.T) {
		input := map[string]struct{}{}
		val, err := GoToRego(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// 빈 map => 길이가 0인 ast.Set
		setVal, ok := val.(ast.Set)
		if !ok {
			t.Fatalf("expected ast.Set, got %T", val)
		}
		if setVal.Len() != 0 {
			t.Fatalf("expected empty set, got length %d", setVal.Len())
		}
	})
}

// Example of hooking into time format from convert.DefaultTimeFormat (if needed).
// Ensure DefaultTimeFormat is declared in the same package, or import the correct constant.
func TestGoToRegoTimeFormatExample(t *testing.T) {
	// Only run if you need to test a specific format or custom check
	fmt.Println("DefaultTimeFormat:", DefaultTimeFormat)
}

func TestGoToRego_NestedStruct(t *testing.T) {
	type NestedEvent struct {
		Start time.Time
		Price decimal.Decimal
	}
	type OuterEvent struct {
		ID   int
		Info NestedEvent
	}

	// 테스트를 위한 예시 데이터
	wantTime, _ := time.Parse(DefaultTimeFormat, "2026-01-01T09:00:00Z")
	wantDec, _ := decimal.NewFromString("1999.99")
	input := OuterEvent{
		ID: 123,
		Info: NestedEvent{
			Start: wantTime,
			Price: wantDec,
		},
	}

	// 변환 수행
	val, err := GoToRego(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 최상위는 ast.Object 여야 함
	obj, ok := val.(ast.Object)
	if !ok {
		t.Fatalf("expected ast.Object, got %T", val)
	}

	// 필드 개수 확인 (ID, Info)
	if obj.Len() != 2 {
		t.Fatalf("expected 2 fields, got %d", obj.Len())
	}

	// ID 필드 검사
	idTerm := obj.Get(ast.StringTerm("ID"))
	if idTerm == nil {
		t.Fatal("expected 'ID' key in object")
	}
	idNum, ok := idTerm.Value.(ast.Number)
	if !ok {
		t.Fatalf("expected ast.Number for 'ID', got %T", idTerm.Value)
	}
	if idNum.String() != "123" {
		t.Fatalf("expected ID=123, got %s", idNum)
	}

	// Info 필드 검사 (nested struct)
	infoTerm := obj.Get(ast.StringTerm("Info"))
	if infoTerm == nil {
		t.Fatal("expected 'Info' key in object")
	}
	infoObj, ok := infoTerm.Value.(ast.Object)
	if !ok {
		t.Fatalf("expected ast.Object for 'Info', got %T", infoTerm.Value)
	}
	if infoObj.Len() != 2 {
		t.Fatalf("expected 2 fields inside 'Info', got %d", infoObj.Len())
	}

	// Info.Start 검사
	startTerm := infoObj.Get(ast.StringTerm("Start"))
	if startTerm == nil {
		t.Fatal("expected 'Start' key in Info object")
	}
	startStr, ok := startTerm.Value.(ast.String)
	if !ok {
		t.Fatalf("expected ast.String for 'Start', got %T", startTerm.Value)
	}
	if string(startStr) != wantTime.Format(DefaultTimeFormat) {
		t.Fatalf("expected Start=%q, got %q", wantTime.Format(DefaultTimeFormat), startStr)
	}

	// Info.Price 검사
	priceTerm := infoObj.Get(ast.StringTerm("Price"))
	if priceTerm == nil {
		t.Fatal("expected 'Price' key in Info object")
	}
	priceNum, ok := priceTerm.Value.(ast.Number)
	if !ok {
		t.Fatalf("expected ast.Number for 'Price', got %T", priceTerm.Value)
	}
	if priceNum.String() != "1999.99" {
		t.Fatalf("expected Price=1999.99, got %s", priceNum)
	}
}
