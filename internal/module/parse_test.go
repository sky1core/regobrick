package module

import (
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
)

// TestAddDefaultFalse_BooleanRule boolean rule에 default가 추가되는지 확인
func TestAddDefaultFalse_BooleanRule(t *testing.T) {
	source := `package test
import data.regobrick.default_false

allow if { input.x == 1 }
`
	mod, err := ParseModule("test.rego", source, nil)
	if err != nil {
		t.Fatalf("ParseModule error: %v", err)
	}

	// default allow = false가 추가되어야 함
	hasDefault := false
	for _, r := range mod.Rules {
		if r.Default && r.Head.Ref().String() == "allow" {
			hasDefault = true
			break
		}
	}

	if !hasDefault {
		t.Error("expected default rule for 'allow', but not found")
	}
}

// TestAddDefaultFalse_ExistingDefault 이미 default가 있으면 중복 추가 안 됨
func TestAddDefaultFalse_ExistingDefault(t *testing.T) {
	source := `package test
import data.regobrick.default_false

default allow = false
allow if { input.x == 1 }
`
	mod, err := ParseModule("test.rego", source, nil)
	if err != nil {
		t.Fatalf("ParseModule error: %v", err)
	}

	// default가 1개만 있어야 함
	defaultCount := 0
	for _, r := range mod.Rules {
		if r.Default && r.Head.Ref().String() == "allow" {
			defaultCount++
		}
	}

	if defaultCount != 1 {
		t.Errorf("expected 1 default rule for 'allow', got %d", defaultCount)
	}
}

// TestAddDefaultFalse_FunctionRule function rule은 skip
func TestAddDefaultFalse_FunctionRule(t *testing.T) {
	source := `package test
import data.regobrick.default_false

is_admin(user) if { user == "admin" }
`
	mod, err := ParseModule("test.rego", source, nil)
	if err != nil {
		t.Fatalf("ParseModule error: %v", err)
	}

	// function rule에는 default가 추가되면 안 됨
	for _, r := range mod.Rules {
		if r.Default {
			t.Errorf("unexpected default rule: %v", r.Head.Ref())
		}
	}
}

// TestAddDefaultFalse_NoImport import 없으면 default_false 미적용
func TestAddDefaultFalse_NoImport(t *testing.T) {
	source := `package test

allow if { input.x == 1 }
`
	mod, err := ParseModule("test.rego", source, nil)
	if err != nil {
		t.Fatalf("ParseModule error: %v", err)
	}

	// import 없으면 default 추가 안 됨
	for _, r := range mod.Rules {
		if r.Default {
			t.Errorf("unexpected default rule without import: %v", r.Head.Ref())
		}
	}
}

// TestAddDefaultFalse_MultipleRules 여러 rule에 각각 default 추가
func TestAddDefaultFalse_MultipleRules(t *testing.T) {
	source := `package test
import data.regobrick.default_false

allow if { input.role == "admin" }
deny if { input.blocked }
`
	mod, err := ParseModule("test.rego", source, nil)
	if err != nil {
		t.Fatalf("ParseModule error: %v", err)
	}

	defaults := make(map[string]bool)
	for _, r := range mod.Rules {
		if r.Default {
			defaults[r.Head.Ref().String()] = true
		}
	}

	if !defaults["allow"] {
		t.Error("expected default rule for 'allow'")
	}
	if !defaults["deny"] {
		t.Error("expected default rule for 'deny'")
	}
}

// =============================================================================
// 기존 동작 보장 테스트 - 수정 후에도 반드시 통과해야 함
// =============================================================================

// TestMustWork_BooleanRuleVariants 다양한 형태의 boolean rule
func TestMustWork_BooleanRuleVariants(t *testing.T) {
	tests := []struct {
		name       string
		source     string
		wantDefault []string // default가 추가되어야 하는 rule 이름들
	}{
		{
			name: "simple_if",
			source: `package test
import data.regobrick.default_false
allow if { input.x }`,
			wantDefault: []string{"allow"},
		},
		{
			name: "multiple_conditions",
			source: `package test
import data.regobrick.default_false
allow if {
    input.role == "admin"
    input.active
}`,
			wantDefault: []string{"allow"},
		},
		{
			name: "multiple_rules_same_name",
			source: `package test
import data.regobrick.default_false
allow if { input.role == "admin" }
allow if { input.role == "superuser" }`,
			wantDefault: []string{"allow"}, // 하나만 추가되어야 함
		},
		{
			name: "different_rules",
			source: `package test
import data.regobrick.default_false
allow if { input.role == "admin" }
deny if { input.blocked }
read_access if { input.level > 0 }`,
			wantDefault: []string{"allow", "deny", "read_access"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mod, err := ParseModule("test.rego", tt.source, nil)
			if err != nil {
				t.Fatalf("ParseModule error: %v", err)
			}

			defaults := make(map[string]int)
			for _, r := range mod.Rules {
				if r.Default {
					defaults[r.Head.Ref().String()]++
				}
			}

			// 기대하는 모든 default가 있는지 확인
			for _, want := range tt.wantDefault {
				if defaults[want] == 0 {
					t.Errorf("expected default for %q, but not found", want)
				}
				if defaults[want] > 1 {
					t.Errorf("duplicate default for %q: count=%d", want, defaults[want])
				}
			}

			// 기대하지 않는 default가 없는지 확인
			wantSet := make(map[string]bool)
			for _, w := range tt.wantDefault {
				wantSet[w] = true
			}
			for name := range defaults {
				if !wantSet[name] {
					t.Errorf("unexpected default for %q", name)
				}
			}
		})
	}
}

// TestMustWork_PartialRules partial rule은 default 추가 안 됨
func TestMustWork_PartialRules(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{
			name: "partial_set",
			source: `package test
import data.regobrick.default_false
items contains x if { x := input.arr[_] }`,
		},
		{
			name: "partial_object",
			source: `package test
import data.regobrick.default_false
obj[k] := v if { k := "a"; v := 1 }`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mod, err := ParseModule("test.rego", tt.source, nil)
			if err != nil {
				t.Fatalf("ParseModule error: %v", err)
			}

			for _, r := range mod.Rules {
				if r.Default {
					t.Errorf("unexpected default rule for partial rule: %v", r.Head.Ref())
				}
			}
		})
	}
}

// TestMustWork_MixedRules boolean과 다른 타입 혼합
func TestMustWork_MixedRules(t *testing.T) {
	source := `package test
import data.regobrick.default_false

allow if { input.role == "admin" }
items contains x if { x := input.arr[_] }
config[k] := v if { k := "key"; v := "value" }
deny if { input.blocked }
`
	mod, err := ParseModule("test.rego", source, nil)
	if err != nil {
		t.Fatalf("ParseModule error: %v", err)
	}

	defaults := make(map[string]bool)
	for _, r := range mod.Rules {
		if r.Default {
			defaults[r.Head.Ref().String()] = true
		}
	}

	// boolean rule만 default 있어야 함
	if !defaults["allow"] {
		t.Error("expected default for 'allow'")
	}
	if !defaults["deny"] {
		t.Error("expected default for 'deny'")
	}

	// partial rule에는 default 없어야 함
	if defaults["items"] {
		t.Error("unexpected default for partial set 'items'")
	}
	if defaults["config"] {
		t.Error("unexpected default for partial object 'config'")
	}
}

// TestMustWork_WithExistingDefault 기존 default 존중
func TestMustWork_WithExistingDefault(t *testing.T) {
	source := `package test
import data.regobrick.default_false

default allow = true
allow if { input.admin }

deny if { input.blocked }
`
	mod, err := ParseModule("test.rego", source, nil)
	if err != nil {
		t.Fatalf("ParseModule error: %v", err)
	}

	allowDefaults := 0
	denyDefaults := 0
	for _, r := range mod.Rules {
		if r.Default {
			switch r.Head.Ref().String() {
			case "allow":
				allowDefaults++
			case "deny":
				denyDefaults++
			}
		}
	}

	if allowDefaults != 1 {
		t.Errorf("expected 1 default for 'allow', got %d", allowDefaults)
	}
	if denyDefaults != 1 {
		t.Errorf("expected 1 default for 'deny', got %d", denyDefaults)
	}
}

// =============================================================================
// AST 구조 검증 테스트 - OPA 버전별 동작 확인용
// =============================================================================

// TestAST_BooleanRuleStructure boolean rule의 AST 구조 확인
// 디버그/조사용 - 검증 로직 없음
func TestAST_BooleanRuleStructure(t *testing.T) {
	t.Skip("디버그용 테스트 - AST 구조 확인 시 Skip 제거")

	source := `package test
allow if { input.x == 1 }
`
	mod, err := ast.ParseModule("test.rego", source)
	if err != nil {
		t.Fatalf("ParseModule error: %v", err)
	}

	if len(mod.Rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(mod.Rules))
	}

	r := mod.Rules[0]
	t.Logf("Boolean rule AST:")
	t.Logf("  Head.Key = %v (type: %T)", r.Head.Key, r.Head.Key)
	t.Logf("  Head.Value = %v", r.Head.Value)
	if r.Head.Value != nil {
		t.Logf("  Head.Value.Value type = %T", r.Head.Value.Value)
	}
	t.Logf("  Head.Args = %v (len: %d)", r.Head.Args, len(r.Head.Args))
	t.Logf("  Body len = %d", len(r.Body))
}

// TestAST_CompleteRuleStructure complete rule의 AST 구조 확인
// 디버그/조사용 - 검증 로직 없음
func TestAST_CompleteRuleStructure(t *testing.T) {
	t.Skip("디버그용 테스트 - AST 구조 확인 시 Skip 제거")

	source := `package test
x := 1
`
	mod, err := ast.ParseModule("test.rego", source)
	if err != nil {
		t.Fatalf("ParseModule error: %v", err)
	}

	if len(mod.Rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(mod.Rules))
	}

	r := mod.Rules[0]
	t.Logf("Complete rule AST:")
	t.Logf("  Head.Key = %v (type: %T)", r.Head.Key, r.Head.Key)
	t.Logf("  Head.Value = %v", r.Head.Value)
	if r.Head.Value != nil {
		t.Logf("  Head.Value.Value type = %T", r.Head.Value.Value)
	}
	t.Logf("  Head.Args = %v (len: %d)", r.Head.Args, len(r.Head.Args))
	t.Logf("  Body len = %d", len(r.Body))
}

// TestAST_PartialSetRuleStructure partial set rule의 AST 구조 확인
// 디버그/조사용 - 검증 로직 없음
func TestAST_PartialSetRuleStructure(t *testing.T) {
	t.Skip("디버그용 테스트 - AST 구조 확인 시 Skip 제거")

	source := `package test
items contains x if { x := input.arr[_] }
`
	mod, err := ast.ParseModule("test.rego", source)
	if err != nil {
		t.Fatalf("ParseModule error: %v", err)
	}

	if len(mod.Rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(mod.Rules))
	}

	r := mod.Rules[0]
	t.Logf("Partial set rule AST:")
	t.Logf("  Head.Key = %v (type: %T)", r.Head.Key, r.Head.Key)
	if r.Head.Key != nil {
		t.Logf("  Head.Key.Value type = %T", r.Head.Key.Value)
	}
	t.Logf("  Head.Value = %v", r.Head.Value)
	t.Logf("  Head.Args = %v (len: %d)", r.Head.Args, len(r.Head.Args))
}

// TestAST_PartialObjectRuleStructure partial object rule의 AST 구조 확인
// 디버그/조사용 - 검증 로직 없음
func TestAST_PartialObjectRuleStructure(t *testing.T) {
	t.Skip("디버그용 테스트 - AST 구조 확인 시 Skip 제거")

	source := `package test
obj[k] := v if { k := "a"; v := 1 }
`
	mod, err := ast.ParseModule("test.rego", source)
	if err != nil {
		t.Fatalf("ParseModule error: %v", err)
	}

	if len(mod.Rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(mod.Rules))
	}

	r := mod.Rules[0]
	t.Logf("Partial object rule AST:")
	t.Logf("  Head.Key = %v (type: %T)", r.Head.Key, r.Head.Key)
	if r.Head.Key != nil {
		t.Logf("  Head.Key.Value type = %T", r.Head.Key.Value)
	}
	t.Logf("  Head.Value = %v", r.Head.Value)
	t.Logf("  Head.Args = %v (len: %d)", r.Head.Args, len(r.Head.Args))
}

// =============================================================================
// 버그 검증 테스트 - 현재 동작을 기록 (수정 전/후 비교용)
// =============================================================================

// TestBugFix_CompleteRuleNoDefault complete rule에는 default가 추가되면 안 됨
// 버그: 현재는 x := 1에도 default x = false가 추가됨
func TestBugFix_CompleteRuleNoDefault(t *testing.T) {
	source := `package test
import data.regobrick.default_false

x := 1
`
	mod, err := ParseModule("test.rego", source, nil)
	if err != nil {
		t.Fatalf("ParseModule error: %v", err)
	}

	for _, r := range mod.Rules {
		if r.Default && r.Head.Ref().String() == "x" {
			t.Errorf("complete rule 'x := 1' should NOT have default, but got: default %v = %v",
				r.Head.Ref(), r.Head.Value)
		}
	}
}

// TestBugFix_ConditionalCompleteRuleNoDefault 조건부 complete rule에도 default 추가되면 안 됨
// 버그: 현재는 x := 1 if {...}에도 default x = false가 추가됨
func TestBugFix_ConditionalCompleteRuleNoDefault(t *testing.T) {
	source := `package test
import data.regobrick.default_false

x := 1 if { input.y }
`
	mod, err := ParseModule("test.rego", source, nil)
	if err != nil {
		t.Fatalf("ParseModule error: %v", err)
	}

	for _, r := range mod.Rules {
		if r.Default && r.Head.Ref().String() == "x" {
			t.Errorf("conditional complete rule 'x := 1 if {...}' should NOT have default, but got: default %v = %v",
				r.Head.Ref(), r.Head.Value)
		}
	}
}

// TestBugFix_BooleanAssignmentNeedsDefault 명시적 boolean 할당에는 default 필요
// 이건 정상 동작 - x := true if {...}에는 default가 있어야 함
func TestBugFix_BooleanAssignmentNeedsDefault(t *testing.T) {
	source := `package test
import data.regobrick.default_false

x := true if { input.y }
`
	mod, err := ParseModule("test.rego", source, nil)
	if err != nil {
		t.Fatalf("ParseModule error: %v", err)
	}

	hasDefault := false
	for _, r := range mod.Rules {
		if r.Default && r.Head.Ref().String() == "x" {
			hasDefault = true
		}
	}

	if !hasDefault {
		t.Error("boolean assignment 'x := true if {...}' SHOULD have default")
	}
}
