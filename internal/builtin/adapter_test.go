package builtin

import (
	"errors"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/rego"
)

func TestIsNil(t *testing.T) {
	var nilPtr *int
	var nilSlice []string
	var nilMap map[string]int
	var nilFunc func()
	var nilChan chan int
	num := 7

	cases := []struct {
		name string
		v    any
		want bool
	}{
		{"nil interface", nil, true},
		{"nil pointer", nilPtr, true},
		{"nil slice", nilSlice, true},
		{"nil map", nilMap, true},
		{"nil func", nilFunc, true},
		{"nil chan", nilChan, true},
		{"non-nil pointer", &num, false},
		{"empty slice", []string{}, false},
		{"empty map", map[string]int{}, false},
		{"int zero", 0, false},
		{"empty string", "", false},
		{"zero struct", struct{}{}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isNil(tc.v); got != tc.want {
				t.Fatalf("isNil(%#v) = %v, want %v", tc.v, got, tc.want)
			}
		})
	}
}

func TestToTerm(t *testing.T) {
	t.Run("nil becomes null", func(t *testing.T) {
		term, err := toTerm(nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, ok := term.Value.(ast.Null); !ok {
			t.Fatalf("expected ast.Null, got %T", term.Value)
		}
	})

	t.Run("nil slice becomes null", func(t *testing.T) {
		term, err := toTerm(([]string)(nil))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, ok := term.Value.(ast.Null); !ok {
			t.Fatalf("expected ast.Null, got %T", term.Value)
		}
	})

	t.Run("empty slice becomes empty array", func(t *testing.T) {
		term, err := toTerm([]string{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		arr, ok := term.Value.(*ast.Array)
		if !ok {
			t.Fatalf("expected *ast.Array, got %T", term.Value)
		}
		if arr.Len() != 0 {
			t.Fatalf("expected empty array, got %v", arr)
		}
	})

	t.Run("struct becomes object via json tags", func(t *testing.T) {
		type payload struct {
			X int    `json:"x"`
			Y string `json:"y"`
		}
		term, err := toTerm(payload{X: 1, Y: "a"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := ast.MustParseTerm(`{"x": 1, "y": "a"}`)
		if term.Value.Compare(want.Value) != 0 {
			t.Fatalf("got %v, want %v", term, want)
		}
	})

	t.Run("unconvertible value returns error", func(t *testing.T) {
		if _, err := toTerm(func() {}); err == nil {
			t.Fatal("expected error for func value, got nil")
		}
	})
}

func terms(vals ...string) []*ast.Term {
	out := make([]*ast.Term, len(vals))
	for i, v := range vals {
		out[i] = ast.MustParseTerm(v)
	}
	return out
}

func TestConvertArgs_Arity(t *testing.T) {
	if _, err := convertArgs1[string](nil); err == nil || !strings.Contains(err.Error(), "expected 1 argument") {
		t.Fatalf("convertArgs1 arity error missing, got %v", err)
	}
	if _, _, err := convertArgs2[string, string](terms(`"a"`)); err == nil || !strings.Contains(err.Error(), "expected 2 arguments") {
		t.Fatalf("convertArgs2 arity error missing, got %v", err)
	}
	if _, _, _, err := convertArgs3[string, string, string](terms(`"a"`, `"b"`)); err == nil || !strings.Contains(err.Error(), "expected 3 arguments") {
		t.Fatalf("convertArgs3 arity error missing, got %v", err)
	}
	if _, _, _, _, err := convertArgs4[string, string, string, string](terms(`"a"`, `"b"`, `"c"`)); err == nil || !strings.Contains(err.Error(), "expected 4 arguments") {
		t.Fatalf("convertArgs4 arity error missing, got %v", err)
	}
	if _, _, _, _, _, err := convertArgs5[string, string, string, string, string](terms(`"a"`, `"b"`, `"c"`, `"d"`)); err == nil || !strings.Contains(err.Error(), "expected 5 arguments") {
		t.Fatalf("convertArgs5 arity error missing, got %v", err)
	}
}

func TestConvertArgs_TypeMismatch(t *testing.T) {
	// A string term cannot convert into an int target.
	if _, err := convertArgs1[int](terms(`"not a number"`)); err == nil {
		t.Fatal("expected conversion error, got nil")
	}
	// Position matters: the second argument fails.
	if _, _, err := convertArgs2[string, int](terms(`"ok"`, `"bad"`)); err == nil {
		t.Fatal("expected conversion error on second arg, got nil")
	}
}

func TestConvertArgs_Success(t *testing.T) {
	v1, v2, v3, v4, v5, err := convertArgs5[string, int, float64, bool, []string](
		terms(`"a"`, `42`, `1.5`, `true`, `["x", "y"]`),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v1 != "a" || v2 != 42 || v3 != 1.5 || v4 != true || len(v5) != 2 || v5[0] != "x" || v5[1] != "y" {
		t.Fatalf("converted values wrong: %v %v %v %v %v", v1, v2, v3, v4, v5)
	}
}

func TestAdapter_ValueAndError(t *testing.T) {
	bctx := rego.BuiltinContext{}

	t.Run("Adapter1 happy path", func(t *testing.T) {
		fn := Adapter1[string, string](func(_ rego.BuiltinContext, s string) (string, error) {
			return s + "!", nil
		})
		term, err := fn(bctx, terms(`"hi"`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if want := ast.MustParseTerm(`"hi!"`); term.Value.Compare(want.Value) != 0 {
			t.Fatalf("got %v, want %v", term, want)
		}
	})

	t.Run("Adapter2 propagates fn error", func(t *testing.T) {
		wantErr := errors.New("boom")
		fn := Adapter2[string, string, string](func(rego.BuiltinContext, string, string) (string, error) {
			return "", wantErr
		})
		if _, err := fn(bctx, terms(`"a"`, `"b"`)); !errors.Is(err, wantErr) {
			t.Fatalf("expected fn error, got %v", err)
		}
	})

	t.Run("Adapter1 propagates conversion error without calling fn", func(t *testing.T) {
		called := false
		fn := Adapter1[int, int](func(_ rego.BuiltinContext, n int) (int, error) {
			called = true
			return n, nil
		})
		if _, err := fn(bctx, terms(`"not an int"`)); err == nil {
			t.Fatal("expected conversion error, got nil")
		}
		if called {
			t.Fatal("fn must not be called when conversion fails")
		}
	})

	t.Run("Adapter0_ returns null", func(t *testing.T) {
		fn := Adapter0_(func(rego.BuiltinContext) error { return nil })
		term, err := fn(bctx, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, ok := term.Value.(ast.Null); !ok {
			t.Fatalf("expected null term, got %v", term)
		}
	})

	t.Run("Adapter3_ error-only propagates error", func(t *testing.T) {
		wantErr := errors.New("side effect failed")
		fn := Adapter3_[string, string, string](func(rego.BuiltinContext, string, string, string) error {
			return wantErr
		})
		if _, err := fn(bctx, terms(`"a"`, `"b"`, `"c"`)); !errors.Is(err, wantErr) {
			t.Fatalf("expected fn error, got %v", err)
		}
	})

	t.Run("Adapter0 nil-typed result becomes null", func(t *testing.T) {
		fn := Adapter0[[]string](func(rego.BuiltinContext) ([]string, error) {
			return nil, nil
		})
		term, err := fn(bctx, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, ok := term.Value.(ast.Null); !ok {
			t.Fatalf("expected null term, got %v", term)
		}
	})
}

// TestAdapter_AllArities exercises every generated adapter shape once: the
// value forms return the concatenation of their arguments, the error-only
// forms record that they were called and return null.
func TestAdapter_AllArities(t *testing.T) {
	bctx := rego.BuiltinContext{}
	args := terms(`"a"`, `"b"`, `"c"`, `"d"`, `"e"`)

	valueForms := []struct {
		name  string
		arity int
		fn    func(rego.BuiltinContext, []*ast.Term) (*ast.Term, error)
		want  string
	}{
		{"Adapter0", 0, Adapter0[string](func(rego.BuiltinContext) (string, error) {
			return "", nil
		}), ``},
		{"Adapter1", 1, Adapter1[string, string](func(_ rego.BuiltinContext, a string) (string, error) {
			return a, nil
		}), `a`},
		{"Adapter2", 2, Adapter2[string, string, string](func(_ rego.BuiltinContext, a, b string) (string, error) {
			return a + b, nil
		}), `ab`},
		{"Adapter3", 3, Adapter3[string, string, string, string](func(_ rego.BuiltinContext, a, b, c string) (string, error) {
			return a + b + c, nil
		}), `abc`},
		{"Adapter4", 4, Adapter4[string, string, string, string, string](func(_ rego.BuiltinContext, a, b, c, d string) (string, error) {
			return a + b + c + d, nil
		}), `abcd`},
		{"Adapter5", 5, Adapter5[string, string, string, string, string, string](func(_ rego.BuiltinContext, a, b, c, d, e string) (string, error) {
			return a + b + c + d + e, nil
		}), `abcde`},
	}
	for _, tc := range valueForms {
		t.Run(tc.name, func(t *testing.T) {
			term, err := tc.fn(bctx, args[:tc.arity])
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			got, ok := term.Value.(ast.String)
			if !ok || string(got) != tc.want {
				t.Fatalf("got %v, want %q", term, tc.want)
			}
		})
	}

	errOnlyForms := []struct {
		name  string
		arity int
		make  func(called *bool) func(rego.BuiltinContext, []*ast.Term) (*ast.Term, error)
	}{
		{"Adapter0_", 0, func(called *bool) func(rego.BuiltinContext, []*ast.Term) (*ast.Term, error) {
			return Adapter0_(func(rego.BuiltinContext) error { *called = true; return nil })
		}},
		{"Adapter1_", 1, func(called *bool) func(rego.BuiltinContext, []*ast.Term) (*ast.Term, error) {
			return Adapter1_[string](func(rego.BuiltinContext, string) error { *called = true; return nil })
		}},
		{"Adapter2_", 2, func(called *bool) func(rego.BuiltinContext, []*ast.Term) (*ast.Term, error) {
			return Adapter2_[string, string](func(rego.BuiltinContext, string, string) error { *called = true; return nil })
		}},
		{"Adapter3_", 3, func(called *bool) func(rego.BuiltinContext, []*ast.Term) (*ast.Term, error) {
			return Adapter3_[string, string, string](func(rego.BuiltinContext, string, string, string) error { *called = true; return nil })
		}},
		{"Adapter4_", 4, func(called *bool) func(rego.BuiltinContext, []*ast.Term) (*ast.Term, error) {
			return Adapter4_[string, string, string, string](func(rego.BuiltinContext, string, string, string, string) error { *called = true; return nil })
		}},
		{"Adapter5_", 5, func(called *bool) func(rego.BuiltinContext, []*ast.Term) (*ast.Term, error) {
			return Adapter5_[string, string, string, string, string](func(rego.BuiltinContext, string, string, string, string, string) error { *called = true; return nil })
		}},
	}
	for _, tc := range errOnlyForms {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			term, err := tc.make(&called)(bctx, args[:tc.arity])
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !called {
				t.Fatal("wrapped fn was not called")
			}
			if _, ok := term.Value.(ast.Null); !ok {
				t.Fatalf("expected null term, got %v", term)
			}
		})
	}
}
