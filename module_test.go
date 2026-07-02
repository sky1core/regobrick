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
	// OPA returns an empty result for undefined values
	rs, err = query.Eval(ctx, rego.EvalInput(map[string]any{"user": "guest"}))
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	// undefined should result in len(rs)==0
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

	// helper module definition
	helperPolicy := `
		package helper
		greeting := "hello from helper"
	`

	// main module: attempts to reference helper.greeting without an import
	// it only works if data.helper is injected via the imports parameter
	mainPolicy := `
		package main
		result := helper.greeting
	`

	// attempt without the imports parameter - should fail
	t.Run("without_imports_fails", func(t *testing.T) {
		_, err := rego.New(
			rego.Module("helper.rego", helperPolicy),
			regobrick.Module("main.rego", mainPolicy, nil), // no imports
			rego.Query("data.main.result"),
		).PrepareForEval(ctx)

		// a compile error occurs if helper is not imported
		if err == nil {
			t.Error("expected error without import, but got none")
		}
	})

	// inject data.helper via the imports parameter - should succeed
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

	// policy with a syntax error - Module delegates the error to rego.Module
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

	// a syntax error should occur
	if err == nil {
		t.Error("expected syntax error, got nil")
	}
}

// TestModule_DefaultFalse_StrictMode verifies that a default_false-transformed module
// compiles under rego.Strict(true) (fix B: the marker import is removed, so there should be no unused import error).
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

// TestModule_FallbackForPlainV0Source when the source uses v0 syntax + no imports + no regobrick usage,
// it should fall back to rego.Module as before (fix D's fallback path).
func TestModule_FallbackForPlainV0Source(t *testing.T) {
	ctx := context.Background()

	// v0 syntax (= true { ... } without if) - fails to parse with the v1 parser.
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

// TestModule_PanicOnV0WithImports when imports are requested but parsing fails (v0 syntax), it should panic
// (fix D's fail-fast path).
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

	// since imports is not empty, it should panic without falling back.
	regobrick.Module("test.rego", policy, []string{"data.helper"})(rego.New())
}

// TestModule_PanicOnUnknownFeature a typo in a regobrick feature should lead to a panic
// because the source references regobrick (fix C + D).
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

	// verify that the default rule was added
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
