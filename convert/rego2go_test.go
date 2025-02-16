package convert_test

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/shopspring/decimal"

	// "github.com/sky1core/regobrick/convert" (실제 import 경로에 맞게 변경하세요)
	. "github.com/sky1core/regobrick/convert"
)

// 예시로 사용할 간단한 구조체
type Person struct {
	Name string
	Age  int
}

func TestRegoToGo(t *testing.T) {

	t.Run("Scalar: int", func(t *testing.T) {
		term := ast.IntNumberTerm(42)
		got, err := RegoToGo[int](term.Value)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != 42 {
			t.Fatalf("expected 42, got %v", got)
		}
	})

	t.Run("Scalar: string", func(t *testing.T) {
		term := ast.StringTerm("hello")
		got, err := RegoToGo[string](term.Value)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "hello" {
			t.Fatalf("expected \"hello\", got %q", got)
		}
	})

	t.Run("Decimal", func(t *testing.T) {
		// OPA에서는 숫자는 기본적으로 Int로, 소수점을 표현하려면 ast.NumberTerm 사용(또는 float형으로 파싱)
		// 여기서는 decimal.Decimal로 직접 변환 가능하도록 가정
		numStr := "123.456"
		decVal, _ := decimal.NewFromString(numStr)

		term := ast.NumberTerm(json.Number(numStr)) // "123.456"
		got, err := RegoToGo[decimal.Decimal](term.Value)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !got.Equal(decVal) {
			t.Fatalf("expected %v, got %v", decVal, got)
		}
	})

	t.Run("Struct", func(t *testing.T) {
		// OPA object: { "Name": "Alice", "Age": 30 }
		obj := ast.ObjectTerm(
			ast.Item(ast.StringTerm("Name"), ast.StringTerm("Alice")),
			ast.Item(ast.StringTerm("Age"), ast.IntNumberTerm(30)),
		)

		got, err := RegoToGo[Person](obj.Value)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := Person{Name: "Alice", Age: 30}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("expected %+v, got %+v", want, got)
		}
	})

	t.Run("Slice", func(t *testing.T) {
		// OPA array: ["a", "b", "c"]
		arr := ast.ArrayTerm(ast.StringTerm("a"), ast.StringTerm("b"), ast.StringTerm("c"))

		got, err := RegoToGo[[]string](arr.Value)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := []string{"a", "b", "c"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("expected %v, got %v", want, got)
		}
	})

	t.Run("Map", func(t *testing.T) {
		// OPA object: { "key1": "val1", "key2": "val2" }
		obj := ast.ObjectTerm(
			ast.Item(ast.StringTerm("key1"), ast.StringTerm("val1")),
			ast.Item(ast.StringTerm("key2"), ast.StringTerm("val2")),
		)

		got, err := RegoToGo[map[string]string](obj.Value)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := map[string]string{"key1": "val1", "key2": "val2"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("expected %v, got %v", want, got)
		}
	})

	t.Run("Pointer", func(t *testing.T) {
		// OPA string to *string
		strTerm := ast.StringTerm("pointer test")

		got, err := RegoToGo[*string](strTerm.Value)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got == nil || *got != "pointer test" {
			t.Fatalf("expected \"pointer test\", got %v", got)
		}
	})

	t.Run("Error case: mismatch type", func(t *testing.T) {
		// Expecting an int, but the term is a string
		strTerm := ast.StringTerm("not an int")

		_, err := RegoToGo[int](strTerm.Value)
		if err == nil {
			t.Fatal("expected error due to type mismatch, got nil")
		}
	})

}

func TestRegoToGo_Time(t *testing.T) {

	t.Run("time.Time valid", func(t *testing.T) {
		// 예: RFC3339 포맷 문자열
		timeStr := "2025-02-16T12:34:56Z"
		term := ast.StringTerm(timeStr)
		got, err := RegoToGo[time.Time](term.Value)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want, _ := time.Parse(DefaultTimeFormat, timeStr)
		if !got.Equal(want) {
			t.Fatalf("expected %v, got %v", want, got)
		}
	})

	t.Run("time.Time invalid format", func(t *testing.T) {
		// 유효하지 않은 시간 문자열
		timeStr := "invalid-time-format"
		term := ast.StringTerm(timeStr)
		_, err := RegoToGo[time.Time](term.Value)
		if err == nil {
			t.Fatal("expected error due to invalid time format, got nil")
		}
	})

	t.Run("time.Time null", func(t *testing.T) {
		// time.Time은 null을 받으면 에러가 나야 함 (assignRegoValueToGo 로직 상)
		nullTerm := ast.NullTerm()
		_, err := RegoToGo[time.Time](nullTerm.Value)
		if err == nil {
			t.Fatal("expected error for null time.Time, got nil")
		}
	})

	t.Run("*time.Time valid", func(t *testing.T) {
		timeStr := "2025-02-16T12:34:56Z"
		term := ast.StringTerm(timeStr)
		got, err := RegoToGo[*time.Time](term.Value)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got == nil {
			t.Fatal("expected non-nil pointer, got nil")
		}
		want, _ := time.Parse(DefaultTimeFormat, timeStr)
		if !got.Equal(want) {
			t.Fatalf("expected %v, got %v", want, got)
		}
	})

	t.Run("*time.Time null", func(t *testing.T) {
		// 포인터 타입은 null을 받으면 nil 포인터가 됨
		nullTerm := ast.NullTerm()
		got, err := RegoToGo[*time.Time](nullTerm.Value)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != nil {
			t.Fatalf("expected nil pointer, got %v", got)
		}
	})

	t.Run("*time.Time invalid format", func(t *testing.T) {
		term := ast.StringTerm("nope")
		_, err := RegoToGo[*time.Time](term.Value)
		if err == nil {
			t.Fatal("expected error due to invalid time format, got nil")
		}
	})
}
func TestRegoToGo_MixedTypes(t *testing.T) {
	t.Run("time in a struct", func(t *testing.T) {
		type Event struct {
			Name string
			When time.Time
		}
		// 예: {"Name":"Concert","When":"2025-12-31T23:59:59Z"}
		obj := ast.ObjectTerm(
			ast.Item(ast.StringTerm("Name"), ast.StringTerm("Concert")),
			ast.Item(ast.StringTerm("When"), ast.StringTerm("2025-12-31T23:59:59Z")),
		)

		got, err := RegoToGo[Event](obj.Value)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		wantTime, _ := time.Parse(DefaultTimeFormat, "2025-12-31T23:59:59Z")
		want := Event{Name: "Concert", When: wantTime}

		if !reflect.DeepEqual(got, want) {
			t.Fatalf("expected %+v, got %+v", want, got)
		}
	})
}

func TestRegoToGo_NestedStructures(t *testing.T) {
	type NestedEvent struct {
		Start time.Time
		Price decimal.Decimal
	}

	type OuterEvent struct {
		ID   int
		Info NestedEvent
	}

	// 예: {
	//   "ID": 123,
	//   "Info": {
	//     "Start": "2026-01-01T09:00:00Z",
	//     "Price": 1999.99
	//   }
	// }
	obj := ast.ObjectTerm(
		ast.Item(ast.StringTerm("ID"), ast.IntNumberTerm(123)),
		ast.Item(ast.StringTerm("Info"), ast.ObjectTerm(
			ast.Item(ast.StringTerm("Start"), ast.StringTerm("2026-01-01T09:00:00Z")),
			ast.Item(ast.StringTerm("Price"), ast.NumberTerm("1999.99")),
		)),
	)

	got, err := RegoToGo[OuterEvent](obj.Value)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 기대값
	wantTime, _ := time.Parse(DefaultTimeFormat, "2026-01-01T09:00:00Z")
	wantDecimal, _ := decimal.NewFromString("1999.99")
	want := OuterEvent{
		ID: 123,
		Info: NestedEvent{
			Start: wantTime,
			Price: wantDecimal,
		},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %+v, got %+v", want, got)
	}
}
