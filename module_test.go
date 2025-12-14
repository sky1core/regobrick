package regobrick_test

import (
	"context"
	"testing"

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
