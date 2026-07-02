package regobrick

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/rego"
	"github.com/open-policy-agent/opa/v1/topdown"
)

// toFloat64 converts json.Number or float64 to float64.
// Returns (value, true) on success, (0, false) if type is not numeric.
func toFloat64(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case json.Number:
		f, err := n.Float64()
		if err != nil {
			return 0, false
		}
		return f, true
	case float64:
		return n, true
	default:
		return 0, false
	}
}

// builtin functions for testing
var testCallCount int64
var mutableCategory = []string{"mutable_test_category"}

func resetCallCount() {
	atomic.StoreInt64(&testCallCount, 0)
}

func getCallCount() int64 {
	return atomic.LoadInt64(&testCallCount)
}

func evalBuiltinModule(t *testing.T, queryText, module string, options ...func(*rego.Rego)) rego.ResultSet {
	t.Helper()

	ctx := context.Background()
	args := []func(*rego.Rego){
		rego.Query(queryText),
		rego.Module("test.rego", module),
	}
	args = append(args, options...)

	query, err := rego.New(args...).PrepareForEval(ctx)
	if err != nil {
		t.Fatalf("prepare error: %v", err)
	}

	rs, err := query.Eval(ctx)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	return rs
}

func requireBuiltinSingleValue(t *testing.T, rs rego.ResultSet) interface{} {
	t.Helper()
	if len(rs) == 0 || len(rs[0].Expressions) == 0 {
		t.Fatal("expected single result, got none")
	}
	return rs[0].Expressions[0].Value
}

func init() {
	// register builtins for testing

	// 1 arg string → returns int
	RegisterBuiltin1[string, int](
		"test_strlen",
		func(bctx rego.BuiltinContext, s string) (int, error) {
			atomic.AddInt64(&testCallCount, 1)
			return len(s), nil
		},
		WithCategories("test"),
	)

	// 2 args int, int → returns int
	RegisterBuiltin2[int64, int64, int64](
		"test_add",
		func(bctx rego.BuiltinContext, a, b int64) (int64, error) {
			return a + b, nil
		},
		WithCategories("test"),
	)

	// returns float64
	RegisterBuiltin1[float64, float64](
		"test_double",
		func(bctx rego.BuiltinContext, x float64) (float64, error) {
			return x * 2, nil
		},
		WithCategories("test"),
	)

	// returns bool
	RegisterBuiltin1[int64, bool](
		"test_is_positive",
		func(bctx rego.BuiltinContext, x int64) (bool, error) {
			return x > 0, nil
		},
		WithCategories("test"),
	)

	// returns map
	RegisterBuiltin1[string, map[string]any](
		"test_wrap",
		func(bctx rego.BuiltinContext, s string) (map[string]any, error) {
			return map[string]any{"value": s, "length": len(s)}, nil
		},
		WithCategories("test"),
	)

	// returns slice
	RegisterBuiltin1[int64, []int64](
		"test_range",
		func(bctx rego.BuiltinContext, n int64) ([]int64, error) {
			result := make([]int64, n)
			for i := int64(0); i < n; i++ {
				result[i] = i
			}
			return result, nil
		},
		WithCategories("test"),
	)

	// for Nondeterministic testing
	RegisterBuiltin0[int64](
		"test_counter",
		func(bctx rego.BuiltinContext) (int64, error) {
			return atomic.AddInt64(&testCallCount, 1), nil
		},
		WithCategories("test"),
		WithNondeterministic(),
	)

	// for RegisterBuiltin1_ testing (error only, returns null)
	RegisterBuiltin1_[string](
		"test_log",
		func(bctx rego.BuiltinContext, msg string) error {
			// side effect only, no return value
			atomic.AddInt64(&testCallCount, 1)
			return nil
		},
		WithCategories("test"),
	)

	// for RegisterBuiltin1_ error propagation testing
	RegisterBuiltin1_[string](
		"test_fail_if_empty",
		func(bctx rego.BuiltinContext, msg string) error {
			if msg == "" {
				return fmt.Errorf("message cannot be empty")
			}
			return nil
		},
		WithCategories("test"),
	)
}

func TestRegisterBuiltin1_StringToInt(t *testing.T) {
	module := `package test
result := test_strlen("hello")
`
	ctx := context.Background()
	query, err := rego.New(
		rego.Query("data.test.result"),
		rego.Module("test.rego", module),
	).PrepareForEval(ctx)
	if err != nil {
		t.Fatalf("prepare error: %v", err)
	}

	rs, err := query.Eval(ctx)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}

	if len(rs) == 0 || len(rs[0].Expressions) == 0 {
		t.Fatal("no result")
	}

	result, ok := toFloat64(rs[0].Expressions[0].Value)
	if !ok {
		t.Fatalf("expected numeric result, got %T", rs[0].Expressions[0].Value)
	}
	if result != 5 {
		t.Errorf("expected 5, got %v", result)
	}
}

func TestRegisterBuiltin2_IntAdd(t *testing.T) {
	module := `package test
result := test_add(10, 20)
`
	ctx := context.Background()
	query, err := rego.New(
		rego.Query("data.test.result"),
		rego.Module("test.rego", module),
	).PrepareForEval(ctx)
	if err != nil {
		t.Fatalf("prepare error: %v", err)
	}

	rs, err := query.Eval(ctx)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}

	if len(rs) == 0 || len(rs[0].Expressions) == 0 {
		t.Fatal("no result")
	}

	result, ok := toFloat64(rs[0].Expressions[0].Value)
	if !ok {
		t.Fatalf("expected numeric result, got %T", rs[0].Expressions[0].Value)
	}
	if result != 30 {
		t.Errorf("expected 30, got %v", result)
	}
}

func TestRegisterBuiltin1_Float(t *testing.T) {
	module := `package test
result := test_double(3.5)
`
	ctx := context.Background()
	query, err := rego.New(
		rego.Query("data.test.result"),
		rego.Module("test.rego", module),
	).PrepareForEval(ctx)
	if err != nil {
		t.Fatalf("prepare error: %v", err)
	}

	rs, err := query.Eval(ctx)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}

	if len(rs) == 0 || len(rs[0].Expressions) == 0 {
		t.Fatal("no result")
	}

	result, ok := toFloat64(rs[0].Expressions[0].Value)
	if !ok {
		t.Fatalf("expected numeric result, got %T", rs[0].Expressions[0].Value)
	}
	if result != 7.0 {
		t.Errorf("expected 7.0, got %v", result)
	}
}

func TestRegisterBuiltin1_Bool(t *testing.T) {
	module := `package test
positive := test_is_positive(5)
negative := test_is_positive(-5)
`
	ctx := context.Background()
	query, err := rego.New(
		rego.Query("data.test"),
		rego.Module("test.rego", module),
	).PrepareForEval(ctx)
	if err != nil {
		t.Fatalf("prepare error: %v", err)
	}

	rs, err := query.Eval(ctx)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}

	if len(rs) == 0 || len(rs[0].Expressions) == 0 {
		t.Fatal("no result")
	}

	result := rs[0].Expressions[0].Value.(map[string]interface{})
	if result["positive"] != true {
		t.Errorf("expected positive=true, got %v", result["positive"])
	}
	if result["negative"] != false {
		t.Errorf("expected negative=false, got %v", result["negative"])
	}
}

func TestRegisterBuiltin1_Map(t *testing.T) {
	module := `package test
result := test_wrap("abc")
`
	ctx := context.Background()
	query, err := rego.New(
		rego.Query("data.test.result"),
		rego.Module("test.rego", module),
	).PrepareForEval(ctx)
	if err != nil {
		t.Fatalf("prepare error: %v", err)
	}

	rs, err := query.Eval(ctx)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}

	if len(rs) == 0 || len(rs[0].Expressions) == 0 {
		t.Fatal("no result")
	}

	result := rs[0].Expressions[0].Value.(map[string]interface{})
	if result["value"] != "abc" {
		t.Errorf("expected value=abc, got %v", result["value"])
	}
	// length is returned as json.Number
	length, ok := toFloat64(result["length"])
	if !ok {
		t.Fatalf("expected numeric length, got %T", result["length"])
	}
	if length != 3.0 {
		t.Errorf("expected length=3, got %v", length)
	}
}

func TestRegisterBuiltin1_Slice(t *testing.T) {
	module := `package test
result := test_range(3)
`
	ctx := context.Background()
	query, err := rego.New(
		rego.Query("data.test.result"),
		rego.Module("test.rego", module),
	).PrepareForEval(ctx)
	if err != nil {
		t.Fatalf("prepare error: %v", err)
	}

	rs, err := query.Eval(ctx)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}

	if len(rs) == 0 || len(rs[0].Expressions) == 0 {
		t.Fatal("no result")
	}

	result := rs[0].Expressions[0].Value.([]interface{})
	if len(result) != 3 {
		t.Errorf("expected len=3, got %d", len(result))
	}
}

func TestWithCategories(t *testing.T) {
	// verify category filtering behavior via FilterCapabilities
	caps := FilterCapabilities(nil, []string{"test"})

	found := false
	for _, b := range caps.Builtins {
		if b.Name == "test_strlen" {
			found = true
			break
		}
	}
	if !found {
		t.Error("test_strlen should be included when filtering by 'test' category")
	}
}

func TestWithNondeterministic(t *testing.T) {
	// verify that a Nondeterministic function is called multiple times
	resetCallCount()

	module := `package test
r1 := test_counter()
r2 := test_counter()
r3 := test_counter()
result := [r1, r2, r3]
`
	ctx := context.Background()
	query, err := rego.New(
		rego.Query("data.test.result"),
		rego.Module("test.rego", module),
	).PrepareForEval(ctx)
	if err != nil {
		t.Fatalf("prepare error: %v", err)
	}

	_, err = query.Eval(ctx)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}

	// since it is Nondeterministic, it must not be memoized (at least 3 calls)
	// OPA may make additional calls, so verify with >= 3
	count := getCallCount()
	if count < 3 {
		t.Errorf("expected at least 3 calls (nondeterministic), got %d", count)
	}
}

// test_memoized_strlen: builtin for Memoize testing
var memoizedCallCount int64

func init() {
	RegisterBuiltin1[string, int](
		"test_memoized_strlen",
		func(bctx rego.BuiltinContext, s string) (int, error) {
			atomic.AddInt64(&memoizedCallCount, 1)
			return len(s), nil
		},
		WithCategories("test"),
		WithMemoize(),
	)

	// for ConfigureFunction(nil) testing - nil should be a no-op
	RegisterBuiltin1[string, int](
		"test_nil_configurator",
		func(bctx rego.BuiltinContext, s string) (int, error) {
			return len(s), nil
		},
		ConfigureFunction(nil),
	)

	// for RegisterBuiltin2_ testing (2 args, error only)
	RegisterBuiltin2_[string, string](
		"test_assert_equal",
		func(bctx rego.BuiltinContext, a, b string) error {
			if a != b {
				return fmt.Errorf("not equal: %q != %q", a, b)
			}
			return nil
		},
		WithCategories("test"),
	)

	// for RegisterBuiltin3 testing (3 args, value return)
	RegisterBuiltin3[int64, int64, int64, int64](
		"test_clamp",
		func(bctx rego.BuiltinContext, val, min, max int64) (int64, error) {
			if val < min {
				return min, nil
			}
			if val > max {
				return max, nil
			}
			return val, nil
		},
		WithCategories("test"),
	)

	// for RegisterBuiltin0_ testing (0 args, error only)
	RegisterBuiltin0_(
		"test_always_succeed",
		func(bctx rego.BuiltinContext) error {
			return nil
		},
		WithCategories("test"),
	)

	RegisterBuiltin0[[]string](
		"test_nil_slice",
		func(bctx rego.BuiltinContext) ([]string, error) {
			return nil, nil
		},
		WithCategories("test"),
	)

	RegisterBuiltin0[int64](
		"test_mutable_category_builtin",
		func(bctx rego.BuiltinContext) (int64, error) {
			return 1, nil
		},
		WithCategories(mutableCategory...),
	)
	mutableCategory[0] = "mutated_after_register"

	RegisterBuiltin0[time.Time](
		"test_fixed_time",
		func(bctx rego.BuiltinContext) (time.Time, error) {
			return time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC), nil
		},
		WithCategories("test"),
	)

	// RegisterBuiltin4 test builtin (4 args, value return): sum of four integers.
	RegisterBuiltin4[int64, int64, int64, int64, int64](
		"test_sum4",
		func(bctx rego.BuiltinContext, a, b, c, d int64) (int64, error) {
			return a + b + c + d, nil
		},
		WithCategories("test"),
	)

	// RegisterBuiltin5 test builtin (5 args, value return): sum of five integers.
	RegisterBuiltin5[int64, int64, int64, int64, int64, int64](
		"test_sum5",
		func(bctx rego.BuiltinContext, a, b, c, d, e int64) (int64, error) {
			return a + b + c + d + e, nil
		},
		WithCategories("test"),
	)

	// RegisterBuiltin4_ test builtin (4 args, error only): all four must be equal.
	RegisterBuiltin4_[string, string, string, string](
		"test_assert_equal4",
		func(bctx rego.BuiltinContext, a, b, c, d string) error {
			if a == b && b == c && c == d {
				return nil
			}
			return fmt.Errorf("not all equal: %q %q %q %q", a, b, c, d)
		},
		WithCategories("test"),
	)

	// RegisterBuiltin5_ test builtin (5 args, error only): all five must be equal.
	RegisterBuiltin5_[string, string, string, string, string](
		"test_assert_equal5",
		func(bctx rego.BuiltinContext, a, b, c, d, e string) error {
			if a == b && b == c && c == d && d == e {
				return nil
			}
			return fmt.Errorf("not all equal: %q %q %q %q %q", a, b, c, d, e)
		},
		WithCategories("test"),
	)
}

func TestConfigureFunction_Nil(t *testing.T) {
	// verify that a builtin registered with ConfigureFunction(nil) works correctly
	module := `package test
result := test_nil_configurator("hello")
`
	rs := evalBuiltinModule(t, "data.test.result", module)
	result, ok := toFloat64(requireBuiltinSingleValue(t, rs))
	if !ok {
		t.Fatalf("expected numeric result, got %T", requireBuiltinSingleValue(t, rs))
	}
	if result != 5 {
		t.Errorf("expected 5, got %v", result)
	}
}

func TestWithMemoize(t *testing.T) {
	// a function with the Memoize option runs only once when called with the same arguments
	atomic.StoreInt64(&memoizedCallCount, 0)

	module := `package test
r1 := test_memoized_strlen("hello")
r2 := test_memoized_strlen("hello")
r3 := test_memoized_strlen("hello")
result := [r1, r2, r3]
`
	ctx := context.Background()
	query, err := rego.New(
		rego.Query("data.test.result"),
		rego.Module("test.rego", module),
	).PrepareForEval(ctx)
	if err != nil {
		t.Fatalf("prepare error: %v", err)
	}

	_, err = query.Eval(ctx)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}

	// since Memoize is set, it is called only once
	count := atomic.LoadInt64(&memoizedCallCount)
	t.Logf("test_memoized_strlen called %d times", count)
	if count != 1 {
		t.Errorf("expected 1 call (memoized), got %d", count)
	}
}

func TestWithMemoize_DifferentArgumentsNotShared(t *testing.T) {
	atomic.StoreInt64(&memoizedCallCount, 0)

	module := `package test
result := [
	test_memoized_strlen("hello"),
	test_memoized_strlen("world"),
	test_memoized_strlen("hello"),
]
`
	_ = evalBuiltinModule(t, "data.test.result", module)

	if count := atomic.LoadInt64(&memoizedCallCount); count != 2 {
		t.Fatalf("expected 2 calls for two distinct arguments, got %d", count)
	}
}

func TestWithMemoize_ResetPerEvaluation(t *testing.T) {
	atomic.StoreInt64(&memoizedCallCount, 0)

	module := `package test
result := test_memoized_strlen("hello")
`
	rs := evalBuiltinModule(t, "data.test.result", module)
	if got, ok := toFloat64(requireBuiltinSingleValue(t, rs)); !ok || got != 5 {
		t.Fatalf("expected first eval result 5, got %v", requireBuiltinSingleValue(t, rs))
	}

	rs = evalBuiltinModule(t, "data.test.result", module)
	if got, ok := toFloat64(requireBuiltinSingleValue(t, rs)); !ok || got != 5 {
		t.Fatalf("expected second eval result 5, got %v", requireBuiltinSingleValue(t, rs))
	}

	if count := atomic.LoadInt64(&memoizedCallCount); count != 2 {
		t.Fatalf("expected memoization to reset per evaluation, got %d calls", count)
	}
}

// === RegisterBuiltinX_ (error only) tests ===

func TestRegisterBuiltin1__SideEffect(t *testing.T) {
	// RegisterBuiltin1_ is a void function (no return value, side effect only)
	// it cannot be used as a value, so call it in a condition
	resetCallCount()

	module := `package test
default result := false
result if {
	test_log("hello")
}
`
	ctx := context.Background()
	query, err := rego.New(
		rego.Query("data.test.result"),
		rego.Module("test.rego", module),
	).PrepareForEval(ctx)
	if err != nil {
		t.Fatalf("prepare error: %v", err)
	}

	rs, err := query.Eval(ctx)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}

	if len(rs) == 0 || len(rs[0].Expressions) == 0 {
		t.Fatal("no result")
	}

	// the rule should evaluate to true
	result := rs[0].Expressions[0].Value.(bool)
	if !result {
		t.Error("expected true, got false")
	}

	// the function should be called
	if getCallCount() != 1 {
		t.Errorf("expected 1 call, got %d", getCallCount())
	}
}

func TestRegisterBuiltin1__ErrorPropagation(t *testing.T) {
	// if a void function returns an error, the rule should fail
	module := `package test
default result := false
result if {
	test_fail_if_empty("")
}
`
	ctx := context.Background()

	t.Run("default_mode", func(t *testing.T) {
		query, err := rego.New(
			rego.Query("data.test.result"),
			rego.Module("test.rego", module),
		).PrepareForEval(ctx)
		if err != nil {
			t.Fatalf("prepare error: %v", err)
		}

		rs, err := query.Eval(ctx)
		if err != nil {
			t.Fatalf("unexpected eval error: %v", err)
		}

		if len(rs) == 0 || len(rs[0].Expressions) == 0 {
			t.Fatal("no result")
		}

		// default mode: the error fails the rule → false
		result := rs[0].Expressions[0].Value.(bool)
		if result {
			t.Error("expected false due to error, but got true")
		}
	})

	t.Run("strict_builtin_errors", func(t *testing.T) {
		query, err := rego.New(
			rego.Query("data.test.result"),
			rego.Module("test.rego", module),
			rego.StrictBuiltinErrors(true),
		).PrepareForEval(ctx)
		if err != nil {
			t.Fatalf("prepare error: %v", err)
		}

		_, err = query.Eval(ctx)
		// Strict mode: the builtin error propagates as an eval error.
		if err == nil {
			t.Fatal("expected eval error, but got none")
		}

		// A plain error returned by a custom builtin surfaces as a topdown
		// BuiltinErr in strict mode, carrying the original message.
		var topdownErr *topdown.Error
		if !errors.As(err, &topdownErr) {
			t.Fatalf("expected topdown.Error, got %T: %v", err, err)
		}
		if topdownErr.Code != topdown.BuiltinErr {
			t.Errorf("expected error code %s, got %s", topdown.BuiltinErr, topdownErr.Code)
		}
		if !strings.Contains(err.Error(), "message cannot be empty") {
			t.Errorf("expected message to contain %q, got: %v", "message cannot be empty", err)
		}
	})
}

// === Additional FilterCapabilities tests ===

func TestFilterCapabilities_Exclusion(t *testing.T) {
	// filtering by a specific category excludes other categories
	caps := FilterCapabilities(nil, []string{"test"})

	// builtins in the strings category should be excluded
	for _, b := range caps.Builtins {
		if b.Name == "sprintf" {
			t.Error("sprintf (strings category) should be excluded when filtering by 'test' category only")
		}
	}
}

func TestFilterCapabilities_CoreInfixPreserved(t *testing.T) {
	// operators in coreInfixes (=, :=, in) must be preserved regardless of the filter applied
	caps := FilterCapabilities(nil, []string{"test"})

	// check the Infix of builtins that belong to coreInfixes
	// "=" → eq, ":=" → assign, "in" → internal.member_2, etc.
	coreInfixValues := []string{"=", ":=", "in"}

	for _, infix := range coreInfixValues {
		found := false
		for _, b := range caps.Builtins {
			if b.Infix == infix {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("core infix %q should be preserved", infix)
		}
	}
}

func TestFilterCapabilities_ArithmeticNotCore(t *testing.T) {
	// arithmetic operators are not in coreInfixes, so they can be excluded during category filtering
	// this test documents that this is intended behavior
	caps := FilterCapabilities(nil, []string{"test"})

	// plus, minus, etc. are not in coreInfixes, so they are excluded
	for _, b := range caps.Builtins {
		if b.Name == "plus" || b.Name == "minus" || b.Name == "mul" || b.Name == "div" {
			t.Errorf("arithmetic operator %q should be excluded when filtering by 'test' category only (not in coreInfixes)", b.Name)
		}
	}
}

func TestFilterCapabilities_ByName(t *testing.T) {
	// filter by name
	caps := FilterCapabilities([]string{"test_strlen"}, nil)

	found := false
	for _, b := range caps.Builtins {
		if b.Name == "test_strlen" {
			found = true
		}
		// test_add is not in the name list, so it should be excluded
		if b.Name == "test_add" {
			t.Error("test_add should be excluded when filtering by name 'test_strlen' only")
		}
	}
	if !found {
		t.Error("test_strlen should be included")
	}
}

func TestWithCategories_CopiesSliceInput(t *testing.T) {
	caps := FilterCapabilities(nil, []string{"mutable_test_category"})

	foundOriginal := false
	foundMutated := false
	for _, b := range caps.Builtins {
		if b.Name == "test_mutable_category_builtin" {
			foundOriginal = true
		}
	}

	caps = FilterCapabilities(nil, []string{"mutated_after_register"})
	for _, b := range caps.Builtins {
		if b.Name == "test_mutable_category_builtin" {
			foundMutated = true
		}
	}

	if !foundOriginal {
		t.Fatal("expected builtin to remain in original category after caller slice mutation")
	}
	if foundMutated {
		t.Fatal("builtin should not move to mutated category after registration")
	}
}

// === Additional RegisterBuiltinX_ tests ===

func TestRegisterBuiltin2__Success(t *testing.T) {
	module := `package test
default result := false
result if {
	test_assert_equal("hello", "hello")
}
`
	ctx := context.Background()
	query, err := rego.New(
		rego.Query("data.test.result"),
		rego.Module("test.rego", module),
	).PrepareForEval(ctx)
	if err != nil {
		t.Fatalf("prepare error: %v", err)
	}

	rs, err := query.Eval(ctx)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}

	if len(rs) == 0 || len(rs[0].Expressions) == 0 {
		t.Fatal("no result")
	}

	result := rs[0].Expressions[0].Value.(bool)
	if !result {
		t.Error("expected true (equal strings), got false")
	}
}

func TestRegisterBuiltin2__Failure(t *testing.T) {
	module := `package test
default result := false
result if {
	test_assert_equal("hello", "world")
}
`
	ctx := context.Background()
	query, err := rego.New(
		rego.Query("data.test.result"),
		rego.Module("test.rego", module),
	).PrepareForEval(ctx)
	if err != nil {
		t.Fatalf("prepare error: %v", err)
	}

	rs, err := query.Eval(ctx)
	if err != nil {
		t.Fatalf("unexpected eval error: %v", err)
	}

	if len(rs) == 0 || len(rs[0].Expressions) == 0 {
		t.Fatal("no result")
	}

	// in default mode, an error leads to rule failure
	result := rs[0].Expressions[0].Value.(bool)
	if result {
		t.Error("expected false (unequal strings cause error), got true")
	}
}

func TestRegisterBuiltin3_Clamp(t *testing.T) {
	tests := []struct {
		name     string
		expr     string
		expected float64
	}{
		{"within_range", "test_clamp(5, 0, 10)", 5},
		{"below_min", "test_clamp(-5, 0, 10)", 0},
		{"above_max", "test_clamp(15, 0, 10)", 10},
		{"at_min", "test_clamp(0, 0, 10)", 0},
		{"at_max", "test_clamp(10, 0, 10)", 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			module := "package test\nresult := " + tt.expr
			ctx := context.Background()
			query, err := rego.New(
				rego.Query("data.test.result"),
				rego.Module("test.rego", module),
			).PrepareForEval(ctx)
			if err != nil {
				t.Fatalf("prepare error: %v", err)
			}

			rs, err := query.Eval(ctx)
			if err != nil {
				t.Fatalf("eval error: %v", err)
			}

			if len(rs) == 0 || len(rs[0].Expressions) == 0 {
				t.Fatal("no result")
			}

			result, ok := toFloat64(rs[0].Expressions[0].Value)
			if !ok {
				t.Fatalf("expected numeric result, got %T", rs[0].Expressions[0].Value)
			}
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestRegisterBuiltin4_Sum(t *testing.T) {
	module := `package test
result := test_sum4(1, 2, 3, 4)
`
	rs := evalBuiltinModule(t, "data.test.result", module)
	result, ok := toFloat64(requireBuiltinSingleValue(t, rs))
	if !ok {
		t.Fatalf("expected numeric result, got %T", requireBuiltinSingleValue(t, rs))
	}
	if result != 10 {
		t.Errorf("expected 10, got %v", result)
	}
}

func TestRegisterBuiltin5_Sum(t *testing.T) {
	module := `package test
result := test_sum5(1, 2, 3, 4, 5)
`
	rs := evalBuiltinModule(t, "data.test.result", module)
	result, ok := toFloat64(requireBuiltinSingleValue(t, rs))
	if !ok {
		t.Fatalf("expected numeric result, got %T", requireBuiltinSingleValue(t, rs))
	}
	if result != 15 {
		t.Errorf("expected 15, got %v", result)
	}
}

func TestRegisterBuiltin4__SuccessAndFailure(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		module := `package test
default result := false
result if {
	test_assert_equal4("a", "a", "a", "a")
}
`
		rs := evalBuiltinModule(t, "data.test.result", module)
		if got := requireBuiltinSingleValue(t, rs); got != true {
			t.Errorf("expected true for all-equal args, got %v", got)
		}
	})

	t.Run("failure_default_mode", func(t *testing.T) {
		// An unequal arg makes the builtin return an error; in default mode the
		// error fails the rule, so result falls back to the default false.
		module := `package test
default result := false
result if {
	test_assert_equal4("a", "a", "a", "b")
}
`
		rs := evalBuiltinModule(t, "data.test.result", module)
		if got := requireBuiltinSingleValue(t, rs); got != false {
			t.Errorf("expected false when args unequal, got %v", got)
		}
	})
}

func TestRegisterBuiltin5__SuccessAndFailure(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		module := `package test
default result := false
result if {
	test_assert_equal5("a", "a", "a", "a", "a")
}
`
		rs := evalBuiltinModule(t, "data.test.result", module)
		if got := requireBuiltinSingleValue(t, rs); got != true {
			t.Errorf("expected true for all-equal args, got %v", got)
		}
	})

	t.Run("failure_default_mode", func(t *testing.T) {
		module := `package test
default result := false
result if {
	test_assert_equal5("a", "a", "a", "a", "b")
}
`
		rs := evalBuiltinModule(t, "data.test.result", module)
		if got := requireBuiltinSingleValue(t, rs); got != false {
			t.Errorf("expected false when args unequal, got %v", got)
		}
	})
}

func TestRegisterBuiltin0__Success(t *testing.T) {
	module := `package test
default result := false
result if {
	test_always_succeed()
}
`
	ctx := context.Background()
	query, err := rego.New(
		rego.Query("data.test.result"),
		rego.Module("test.rego", module),
	).PrepareForEval(ctx)
	if err != nil {
		t.Fatalf("prepare error: %v", err)
	}

	rs, err := query.Eval(ctx)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}

	if len(rs) == 0 || len(rs[0].Expressions) == 0 {
		t.Fatal("no result")
	}

	result := rs[0].Expressions[0].Value.(bool)
	if !result {
		t.Error("expected true, got false")
	}
}

func TestRegisterBuiltin_ReturnsNilSliceAsNull(t *testing.T) {
	module := `package test
result := test_nil_slice()
`
	rs := evalBuiltinModule(t, "data.test.result", module)
	if got := requireBuiltinSingleValue(t, rs); got != nil {
		t.Fatalf("expected null result for nil slice, got %v (%T)", got, got)
	}
}

func TestRegisterBuiltin_ReturnsTimeAsRFC3339String(t *testing.T) {
	module := `package test
result := test_fixed_time()
`
	rs := evalBuiltinModule(t, "data.test.result", module)
	if got := requireBuiltinSingleValue(t, rs); got != "2024-01-02T03:04:05Z" {
		t.Fatalf("expected RFC3339 string, got %v (%T)", got, got)
	}
}

// === Type conversion error tests ===

func TestBuiltin_TypeMismatch_CompileTime(t *testing.T) {
	// OPA performs type checking at compile time
	// an incorrect type raises an error in PrepareForEval
	tests := []struct {
		name   string
		module string
	}{
		{
			"string_to_int_builtin",
			`package test
result := test_add("hello", 1)`,
		},
		{
			"bool_to_int_builtin",
			`package test
result := test_clamp(true, 0, 10)`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			_, err := rego.New(
				rego.Query("data.test.result"),
				rego.Module("test.rego", tt.module),
			).PrepareForEval(ctx)

			// a compile-time type error occurs
			if err == nil {
				t.Fatal("expected compile-time type error, got none")
			}

			// first: check for a typed error (ast.Errors)
			var astErrs ast.Errors
			if errors.As(err, &astErrs) {
				hasTypeError := false
				for _, e := range astErrs {
					if e.Code == ast.TypeErr {
						hasTypeError = true
						break
					}
				}
				if !hasTypeError {
					t.Errorf("expected ast.TypeErr in errors, got: %v", astErrs)
				}
			} else {
				// second: string fallback (when it is not ast.Errors)
				if !strings.Contains(err.Error(), "rego_type_error") {
					t.Errorf("expected rego_type_error, got: %v", err)
				}
			}
		})
	}
}
