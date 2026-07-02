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

// 테스트용 builtin 함수들
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
	// 테스트용 builtin 등록

	// 1인자 string → int 반환
	RegisterBuiltin1[string, int](
		"test_strlen",
		func(bctx rego.BuiltinContext, s string) (int, error) {
			atomic.AddInt64(&testCallCount, 1)
			return len(s), nil
		},
		WithCategories("test"),
	)

	// 2인자 int, int → int 반환
	RegisterBuiltin2[int64, int64, int64](
		"test_add",
		func(bctx rego.BuiltinContext, a, b int64) (int64, error) {
			return a + b, nil
		},
		WithCategories("test"),
	)

	// float64 반환
	RegisterBuiltin1[float64, float64](
		"test_double",
		func(bctx rego.BuiltinContext, x float64) (float64, error) {
			return x * 2, nil
		},
		WithCategories("test"),
	)

	// bool 반환
	RegisterBuiltin1[int64, bool](
		"test_is_positive",
		func(bctx rego.BuiltinContext, x int64) (bool, error) {
			return x > 0, nil
		},
		WithCategories("test"),
	)

	// map 반환
	RegisterBuiltin1[string, map[string]any](
		"test_wrap",
		func(bctx rego.BuiltinContext, s string) (map[string]any, error) {
			return map[string]any{"value": s, "length": len(s)}, nil
		},
		WithCategories("test"),
	)

	// slice 반환
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

	// Nondeterministic 테스트용
	RegisterBuiltin0[int64](
		"test_counter",
		func(bctx rego.BuiltinContext) (int64, error) {
			return atomic.AddInt64(&testCallCount, 1), nil
		},
		WithCategories("test"),
		WithNondeterministic(),
	)

	// RegisterBuiltin1_ (에러만 반환, null 리턴) 테스트용
	RegisterBuiltin1_[string](
		"test_log",
		func(bctx rego.BuiltinContext, msg string) error {
			// 사이드 이펙트만 있고 반환값 없음
			atomic.AddInt64(&testCallCount, 1)
			return nil
		},
		WithCategories("test"),
	)

	// RegisterBuiltin1_ 에러 전파 테스트용
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
	// length는 json.Number로 반환됨
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
	// FilterCapabilities로 카테고리 필터링 동작 검증
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
	// Nondeterministic 함수가 여러 번 호출되는지 확인
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

	// Nondeterministic이므로 memoize되면 안 됨 (최소 3번 호출)
	// OPA가 추가 호출을 할 수 있으므로 >= 3으로 검증
	count := getCallCount()
	if count < 3 {
		t.Errorf("expected at least 3 calls (nondeterministic), got %d", count)
	}
}

// test_memoized_strlen: Memoize 테스트용 builtin
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

	// ConfigureFunction(nil) 테스트용 - nil은 no-op이어야 함
	RegisterBuiltin1[string, int](
		"test_nil_configurator",
		func(bctx rego.BuiltinContext, s string) (int, error) {
			return len(s), nil
		},
		ConfigureFunction(nil),
	)

	// RegisterBuiltin2_ 테스트용 (2인자, 에러만 반환)
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

	// RegisterBuiltin3 테스트용 (3인자, 값 반환)
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

	// RegisterBuiltin0_ 테스트용 (0인자, 에러만 반환)
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
}

func TestConfigureFunction_Nil(t *testing.T) {
	// ConfigureFunction(nil)이 등록된 builtin이 정상 동작하는지 확인
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
	// Memoize 옵션이 설정된 함수는 같은 인자로 호출 시 1번만 실행됨
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

	// Memoize가 설정되어 있으므로 1번만 호출됨
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

// === RegisterBuiltinX_ (에러만 반환) 테스트 ===

func TestRegisterBuiltin1__SideEffect(t *testing.T) {
	// RegisterBuiltin1_는 void 함수 (반환값 없음, 사이드 이펙트만)
	// 값으로 사용할 수 없으므로 조건절에서 호출
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

	// 규칙이 true로 평가되어야 함
	result := rs[0].Expressions[0].Value.(bool)
	if !result {
		t.Error("expected true, got false")
	}

	// 함수는 호출되어야 함
	if getCallCount() != 1 {
		t.Errorf("expected 1 call, got %d", getCallCount())
	}
}

func TestRegisterBuiltin1__ErrorPropagation(t *testing.T) {
	// void 함수가 에러를 반환하면 규칙이 실패해야 함
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

		// 기본 모드: 에러로 인해 규칙 실패 → false
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
		// Strict 모드: eval error 발생
		if err == nil {
			t.Error("expected eval error, but got none")
		}
	})
}

// === FilterCapabilities 추가 테스트 ===

func TestFilterCapabilities_Exclusion(t *testing.T) {
	// 특정 카테고리로 필터링하면 다른 카테고리는 제외됨
	caps := FilterCapabilities(nil, []string{"test"})

	// strings 카테고리의 builtin은 제외되어야 함
	for _, b := range caps.Builtins {
		if b.Name == "sprintf" {
			t.Error("sprintf (strings category) should be excluded when filtering by 'test' category only")
		}
	}
}

func TestFilterCapabilities_CoreInfixPreserved(t *testing.T) {
	// coreInfixes에 있는 연산자(=, :=, in)는 어떤 필터를 적용해도 보존되어야 함
	caps := FilterCapabilities(nil, []string{"test"})

	// coreInfixes에 해당하는 builtin들의 Infix 확인
	// "=" → eq, ":=" → assign, "in" → internal.member_2 등
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
	// 산술 연산자는 coreInfixes에 없으므로 카테고리 필터링 시 제외될 수 있음
	// 이는 의도된 동작임을 문서화하는 테스트
	caps := FilterCapabilities(nil, []string{"test"})

	// plus, minus 등은 coreInfixes에 없으므로 제외됨
	for _, b := range caps.Builtins {
		if b.Name == "plus" || b.Name == "minus" || b.Name == "mul" || b.Name == "div" {
			t.Errorf("arithmetic operator %q should be excluded when filtering by 'test' category only (not in coreInfixes)", b.Name)
		}
	}
}

func TestFilterCapabilities_ByName(t *testing.T) {
	// 이름으로 필터링
	caps := FilterCapabilities([]string{"test_strlen"}, nil)

	found := false
	for _, b := range caps.Builtins {
		if b.Name == "test_strlen" {
			found = true
		}
		// test_add는 이름 목록에 없으므로 제외되어야 함
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

// === 추가 RegisterBuiltinX_ 테스트 ===

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

	// 기본 모드에서 에러는 규칙 실패로 이어짐
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

// === 타입 변환 에러 테스트 ===

func TestBuiltin_TypeMismatch_CompileTime(t *testing.T) {
	// OPA는 타입 체크를 컴파일 타임에 수행함
	// 잘못된 타입은 PrepareForEval에서 에러 발생
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

			// 컴파일 타임 타입 에러 발생
			if err == nil {
				t.Fatal("expected compile-time type error, got none")
			}

			// 1차: typed error 확인 (ast.Errors)
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
				// 2차: 문자열 fallback (ast.Errors가 아닌 경우)
				if !strings.Contains(err.Error(), "rego_type_error") {
					t.Errorf("expected rego_type_error, got: %v", err)
				}
			}
		})
	}
}
