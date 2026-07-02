package regobrick_test

import (
	"context"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/rego"
	"github.com/sky1core/regobrick"
)

func TestModule_BasicEvaluation(t *testing.T) {
	ctx := context.Background()

	policy := `
		package test
		allow if {
			input.user == "admin"
		}
	`

	query, err := rego.New(
		regobrick.Module("test.rego", policy, nil),
		rego.Query("data.test.allow"),
	).PrepareForEval(ctx)
	if err != nil {
		t.Fatalf("PrepareForEval failed: %v", err)
	}

	// allow = true when user is admin
	rs, err := query.Eval(ctx, rego.EvalInput(map[string]any{"user": "admin"}))
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	if len(rs) == 0 {
		t.Fatal("expected result, got empty")
	}
	if len(rs[0].Expressions) == 0 {
		t.Fatal("expected expressions, got empty")
	}
	if rs[0].Expressions[0].Value != true {
		t.Errorf("expected true, got %v", rs[0].Expressions[0].Value)
	}

	// allow = undefined when user is not admin (no default_false import)
	// OPA는 undefined 값에 대해 빈 결과를 반환함
	rs, err = query.Eval(ctx, rego.EvalInput(map[string]any{"user": "guest"}))
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	// undefined는 len(rs)==0이어야 함
	if len(rs) != 0 {
		t.Errorf("expected undefined (len(rs)==0), got %d results: %v", len(rs), rs)
	}
}

func TestModule_DefaultFalse(t *testing.T) {
	ctx := context.Background()

	policy := `
		package test
		import data.regobrick.default_false

		allow if {
			input.user == "admin"
		}
	`

	query, err := rego.New(
		regobrick.Module("test.rego", policy, nil),
		rego.Query("data.test.allow"),
	).PrepareForEval(ctx)
	if err != nil {
		t.Fatalf("PrepareForEval failed: %v", err)
	}

	// allow = true when user is admin
	rs, err := query.Eval(ctx, rego.EvalInput(map[string]any{"user": "admin"}))
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	if len(rs) == 0 || rs[0].Expressions[0].Value != true {
		t.Errorf("expected true, got %v", rs)
	}

	// allow = false when user is not admin (default_false applied)
	rs, err = query.Eval(ctx, rego.EvalInput(map[string]any{"user": "guest"}))
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	if len(rs) == 0 || rs[0].Expressions[0].Value != false {
		t.Errorf("expected false, got %v", rs)
	}
}

func TestModules_MultipleModules(t *testing.T) {
	ctx := context.Background()

	helperPolicy := `
		package helper
		is_admin(user) if user == "admin"
	`

	mainPolicy := `
		package main
		import data.regobrick.default_false
		import data.helper

		allow if {
			helper.is_admin(input.user)
		}
	`

	query, err := rego.New(
		regobrick.Modules(
			regobrick.ModuleOption{Filename: "helper.rego", Source: helperPolicy},
			regobrick.ModuleOption{Filename: "main.rego", Source: mainPolicy},
		),
		rego.Query("data.main.allow"),
	).PrepareForEval(ctx)
	if err != nil {
		t.Fatalf("PrepareForEval failed: %v", err)
	}

	// allow = true when user is admin
	rs, err := query.Eval(ctx, rego.EvalInput(map[string]any{"user": "admin"}))
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	if len(rs) == 0 || rs[0].Expressions[0].Value != true {
		t.Errorf("expected true, got %v", rs)
	}

	// allow = false when user is not admin
	rs, err = query.Eval(ctx, rego.EvalInput(map[string]any{"user": "guest"}))
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	if len(rs) == 0 || rs[0].Expressions[0].Value != false {
		t.Errorf("expected false, got %v", rs)
	}
}

func TestModule_WithImports(t *testing.T) {
	ctx := context.Background()

	// helper 모듈 정의
	helperPolicy := `
		package helper
		greeting := "hello from helper"
	`

	// main 모듈: import 없이 helper.greeting 참조 시도
	// imports 파라미터로 data.helper를 주입해야만 동작함
	mainPolicy := `
		package main
		result := helper.greeting
	`

	// imports 파라미터 없이 시도 - 실패해야 함
	t.Run("without_imports_fails", func(t *testing.T) {
		_, err := rego.New(
			rego.Module("helper.rego", helperPolicy),
			regobrick.Module("main.rego", mainPolicy, nil), // imports 없음
			rego.Query("data.main.result"),
		).PrepareForEval(ctx)

		// helper를 import하지 않으면 컴파일 에러 발생
		if err == nil {
			t.Error("expected error without import, but got none")
		}
	})

	// imports 파라미터로 data.helper 주입 - 성공해야 함
	t.Run("with_imports_succeeds", func(t *testing.T) {
		query, err := rego.New(
			rego.Module("helper.rego", helperPolicy),
			regobrick.Module("main.rego", mainPolicy, []string{"data.helper"}),
			rego.Query("data.main.result"),
		).PrepareForEval(ctx)
		if err != nil {
			t.Fatalf("PrepareForEval failed: %v", err)
		}

		rs, err := query.Eval(ctx)
		if err != nil {
			t.Fatalf("Eval failed: %v", err)
		}
		if len(rs) == 0 {
			t.Fatal("expected result, got empty")
		}
		if len(rs[0].Expressions) == 0 {
			t.Fatal("expected expressions, got empty")
		}
		if rs[0].Expressions[0].Value != "hello from helper" {
			t.Errorf("expected 'hello from helper', got %v", rs[0].Expressions[0].Value)
		}
	})
}

func TestModule_WithImports_DeduplicatesExistingImport(t *testing.T) {
	ctx := context.Background()

	helperPolicy := `
		package helper
		value := "hello"
	`

	mainPolicy := `
		package main
		import data.helper

		result := helper.value
	`

	query, err := rego.New(
		rego.Module("helper.rego", helperPolicy),
		regobrick.Module("main.rego", mainPolicy, []string{"data.helper"}),
		rego.Query("data.main.result"),
	).PrepareForEval(ctx)
	if err != nil {
		t.Fatalf("PrepareForEval failed: %v", err)
	}

	rs, err := query.Eval(ctx)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	if len(rs) == 0 || len(rs[0].Expressions) == 0 {
		t.Fatal("expected result, got empty")
	}
	if rs[0].Expressions[0].Value != "hello" {
		t.Fatalf("expected hello, got %v", rs[0].Expressions[0].Value)
	}
}

func TestModule_WithImports_PreservesAliasedImportAndAddsPlainImport(t *testing.T) {
	ctx := context.Background()

	helperPolicy := `
		package helper
		value := "hello"
	`

	mainPolicy := `
		package main
		import data.helper as h

		also := h.value
		result := helper.value
	`

	query, err := rego.New(
		rego.Module("helper.rego", helperPolicy),
		regobrick.Module("main.rego", mainPolicy, []string{"data.helper"}),
		rego.Query("data.main"),
	).PrepareForEval(ctx)
	if err != nil {
		t.Fatalf("PrepareForEval failed: %v", err)
	}

	rs, err := query.Eval(ctx)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	if len(rs) == 0 || len(rs[0].Expressions) == 0 {
		t.Fatal("expected result, got empty")
	}

	obj, ok := rs[0].Expressions[0].Value.(map[string]interface{})
	if !ok {
		t.Fatalf("expected object result, got %T", rs[0].Expressions[0].Value)
	}
	if obj["result"] != "hello" || obj["also"] != "hello" {
		t.Fatalf("expected both imports to work, got %v", obj)
	}
}

func TestModule_WithImports_DeduplicatesDefaultAliasImport(t *testing.T) {
	ctx := context.Background()

	helperPolicy := `
		package helper
		value := "hello"
	`

	mainPolicy := `
		package main
		import data.helper as helper

		result := helper.value
	`

	query, err := rego.New(
		rego.Module("helper.rego", helperPolicy),
		regobrick.Module("main.rego", mainPolicy, []string{"data.helper"}),
		rego.Query("data.main.result"),
	).PrepareForEval(ctx)
	if err != nil {
		t.Fatalf("PrepareForEval failed: %v", err)
	}

	rs, err := query.Eval(ctx)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	if len(rs) == 0 || len(rs[0].Expressions) == 0 {
		t.Fatal("expected result, got empty")
	}
	if rs[0].Expressions[0].Value != "hello" {
		t.Fatalf("expected hello, got %v", rs[0].Expressions[0].Value)
	}
}

func TestModule_WithImports_DeduplicatesRepeatedInjectedImports(t *testing.T) {
	ctx := context.Background()

	helperPolicy := `
		package helper
		value := "hello"
	`

	mainPolicy := `
		package main
		result := helper.value
	`

	query, err := rego.New(
		rego.Module("helper.rego", helperPolicy),
		regobrick.Module("main.rego", mainPolicy, []string{"data.helper", "data.helper"}),
		rego.Query("data.main.result"),
	).PrepareForEval(ctx)
	if err != nil {
		t.Fatalf("PrepareForEval failed: %v", err)
	}

	rs, err := query.Eval(ctx)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	if len(rs) == 0 || len(rs[0].Expressions) == 0 {
		t.Fatal("expected result, got empty")
	}
	if rs[0].Expressions[0].Value != "hello" {
		t.Fatalf("expected hello, got %v", rs[0].Expressions[0].Value)
	}
}

func TestModule_SyntaxError(t *testing.T) {
	ctx := context.Background()

	// 문법 에러가 있는 정책 - Module은 에러를 rego.Module로 위임
	policy := `
		package test
		allow if {
			invalid syntax here
		}
	`

	_, err := rego.New(
		regobrick.Module("test.rego", policy, nil),
		rego.Query("data.test.allow"),
	).PrepareForEval(ctx)

	// 문법 에러가 발생해야 함
	if err == nil {
		t.Error("expected syntax error, got nil")
	}
}

// TestModule_DefaultFalse_StrictMode default_false 변환된 모듈이 rego.Strict(true)에서
// 컴파일되는지 확인 (수정 B: 마커 import가 제거되어 unused import 에러가 없어야 함).
func TestModule_DefaultFalse_StrictMode(t *testing.T) {
	ctx := context.Background()

	policy := `
		package test
		import data.regobrick.default_false

		allow if { input.user == "admin" }
	`

	query, err := rego.New(
		rego.Strict(true),
		regobrick.Module("test.rego", policy, nil),
		rego.Query("data.test.allow"),
	).PrepareForEval(ctx)
	if err != nil {
		t.Fatalf("expected strict-mode compilation to succeed, got: %v", err)
	}

	rs, err := query.Eval(ctx, rego.EvalInput(map[string]any{"user": "guest"}))
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	if len(rs) == 0 || rs[0].Expressions[0].Value != false {
		t.Errorf("expected false, got %v", rs)
	}
}

// TestModule_FallbackForPlainV0Source v0 문법 소스 + imports 없음 + regobrick 미사용인 경우
// 기존처럼 rego.Module 폴백으로 동작해야 한다 (수정 D의 폴백 경로).
func TestModule_FallbackForPlainV0Source(t *testing.T) {
	ctx := context.Background()

	// v0 문법 (if 없이 = true { ... }) - v1 파서로는 파싱 실패한다.
	policy := `package test
allow = true { input.x == 1 }
`

	query, err := rego.New(
		rego.SetRegoVersion(ast.RegoV0),
		regobrick.Module("test.rego", policy, nil),
		rego.Query("data.test.allow"),
	).PrepareForEval(ctx)
	if err != nil {
		t.Fatalf("expected v0 fallback to compile, got: %v", err)
	}

	rs, err := query.Eval(ctx, rego.EvalInput(map[string]any{"x": 1}))
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	if len(rs) == 0 || rs[0].Expressions[0].Value != true {
		t.Errorf("expected true from v0 fallback, got %v", rs)
	}
}

// TestModule_PanicOnV0WithImports imports를 요청했는데 파싱이 실패하면(v0 문법) panic해야 한다
// (수정 D의 fail-fast 경로).
func TestModule_PanicOnV0WithImports(t *testing.T) {
	policy := `package test
allow = true { input.x == 1 }
`

	defer func() {
		rec := recover()
		if rec == nil {
			t.Fatal("expected panic when imports requested but parse failed, got none")
		}
		msg, _ := rec.(string)
		if !strings.Contains(msg, "cannot process module") || !strings.Contains(msg, "test.rego") {
			t.Errorf("panic message should mention cause and filename, got: %v", rec)
		}
	}()

	// imports가 비어있지 않으므로 폴백 없이 panic해야 한다.
	regobrick.Module("test.rego", policy, []string{"data.helper"})(rego.New())
}

// TestModule_PanicOnUnknownFeature regobrick feature 오타는 소스가 regobrick을 참조하므로
// panic으로 이어져야 한다 (수정 C + D).
func TestModule_PanicOnUnknownFeature(t *testing.T) {
	policy := `package test
import data.regobrick.default_flase

allow if { input.x }
`

	defer func() {
		rec := recover()
		if rec == nil {
			t.Fatal("expected panic for unknown regobrick feature, got none")
		}
		msg, _ := rec.(string)
		if !strings.Contains(msg, "cannot process module") {
			t.Errorf("panic message should mention cause, got: %v", rec)
		}
	}()

	regobrick.Module("typo.rego", policy, nil)(rego.New())
}

func TestParseModule_Direct(t *testing.T) {
	policy := `
		package test
		import data.regobrick.default_false

		allow if {
			input.ok
		}
	`

	module, err := regobrick.ParseModule("test.rego", policy, nil)
	if err != nil {
		t.Fatalf("ParseModule failed: %v", err)
	}

	if module.Package.Path.String() != "data.test" {
		t.Errorf("expected package data.test, got %s", module.Package.Path.String())
	}

	// default rule이 추가되었는지 확인
	hasDefaultAllow := false
	for _, rule := range module.Rules {
		if rule.Default {
			ref := rule.Head.Ref()
			if ref != nil && ref.String() == "allow" {
				hasDefaultAllow = true
				break
			}
		}
	}
	if !hasDefaultAllow {
		t.Error("expected default allow rule to be added")
	}
}
