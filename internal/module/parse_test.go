package module

import (
	"strings"
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

func TestParseModule_DeduplicatesExistingImport(t *testing.T) {
	source := `package test
import data.helper

result := helper.value
`
	mod, err := ParseModule("test.rego", source, []string{"data.helper"})
	if err != nil {
		t.Fatalf("ParseModule error: %v", err)
	}

	count := 0
	for _, imp := range mod.Imports {
		if ref, ok := imp.Path.Value.(ast.Ref); ok && ref.String() == "data.helper" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected exactly 1 import for data.helper, got %d", count)
	}
}

func TestParseModule_KeepsDistinctAliasedImport(t *testing.T) {
	source := `package test
import data.helper as h

result := helper.value
`
	mod, err := ParseModule("test.rego", source, []string{"data.helper"})
	if err != nil {
		t.Fatalf("ParseModule error: %v", err)
	}

	count := 0
	hasAliased := false
	hasPlain := false
	for _, imp := range mod.Imports {
		ref, ok := imp.Path.Value.(ast.Ref)
		if !ok || ref.String() != "data.helper" {
			continue
		}
		count++
		if imp.Alias == "h" {
			hasAliased = true
		}
		if imp.Alias == "" {
			hasPlain = true
		}
	}
	if count != 2 {
		t.Fatalf("expected aliased and plain imports for data.helper, got %d", count)
	}
	if !hasAliased || !hasPlain {
		t.Fatalf("expected both aliased and plain imports, got aliased=%v plain=%v", hasAliased, hasPlain)
	}
}

func TestParseModule_DeduplicatesDefaultAliasImport(t *testing.T) {
	source := `package test
import data.helper as helper

result := helper.value
`
	mod, err := ParseModule("test.rego", source, []string{"data.helper"})
	if err != nil {
		t.Fatalf("ParseModule error: %v", err)
	}

	count := 0
	for _, imp := range mod.Imports {
		ref, ok := imp.Path.Value.(ast.Ref)
		if ok && ref.String() == "data.helper" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected default alias import to be deduplicated, got %d imports", count)
	}
}

func TestParseModule_DeduplicatesRepeatedInjectedImports(t *testing.T) {
	source := `package test

result := helper.value
`
	mod, err := ParseModule("test.rego", source, []string{"data.helper", "data.helper"})
	if err != nil {
		t.Fatalf("ParseModule error: %v", err)
	}

	count := 0
	for _, imp := range mod.Imports {
		if ref, ok := imp.Path.Value.(ast.Ref); ok && ref.String() == "data.helper" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected repeated injected imports to be deduplicated, got %d imports", count)
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
		name        string
		source      string
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
// 조용한 실패(silent failure) 수정 재현 테스트
// =============================================================================

// TestParseModule_PreservesAnnotations METADATA 어노테이션이 보존되는지 확인 (수정 A)
func TestParseModule_PreservesAnnotations(t *testing.T) {
	source := `# METADATA
# title: allow rule
# description: sample
package test

allow if { input.x }
`
	mod, err := ParseModule("test.rego", source, nil)
	if err != nil {
		t.Fatalf("ParseModule error: %v", err)
	}
	if len(mod.Annotations) == 0 {
		t.Fatal("expected METADATA annotations to be preserved, got none")
	}
	if mod.Annotations[0].Title != "allow rule" {
		t.Errorf("expected annotation title %q, got %q", "allow rule", mod.Annotations[0].Title)
	}
}

// TestParseModule_RemovesMarkerImport default_false 마커 import가 변환 후 제거되는지 확인 (수정 B)
func TestParseModule_RemovesMarkerImport(t *testing.T) {
	source := `package test
import data.regobrick.default_false

allow if { input.x }
`
	mod, err := ParseModule("test.rego", source, nil)
	if err != nil {
		t.Fatalf("ParseModule error: %v", err)
	}
	for _, imp := range mod.Imports {
		if ref, ok := imp.Path.Value.(ast.Ref); ok && ref.String() == "data.regobrick.default_false" {
			t.Fatalf("expected regobrick marker import to be removed, but it remains: %s", imp.String())
		}
	}
}

// TestParseModule_UnknownFeatureErrors 미지 feature 오타는 조용히 무시되지 않고 에러 (수정 C)
func TestParseModule_UnknownFeatureErrors(t *testing.T) {
	source := `package test
import data.regobrick.default_flase

allow if { input.x }
`
	_, err := ParseModule("test.rego", source, nil)
	if err == nil {
		t.Fatal("expected error for unknown regobrick feature, got nil")
	}
	if !strings.Contains(err.Error(), "data.regobrick.default_flase") {
		t.Errorf("error should mention the offending import, got: %v", err)
	}
	if !strings.Contains(err.Error(), "default_false") {
		t.Errorf("error should list known features, got: %v", err)
	}
}

// TestParseModule_InvalidImportPathErrors 잘못된 주입 import 경로는 에러 (수정 E)
func TestParseModule_InvalidImportPathErrors(t *testing.T) {
	source := `package test
allow if { input.x }
`
	_, err := ParseModule("test.rego", source, []string{"data..x"})
	if err == nil {
		t.Fatal("expected error for invalid injected import path, got nil")
	}
	if !strings.Contains(err.Error(), "data..x") {
		t.Errorf("error should mention the offending path, got: %v", err)
	}
}

// TestParseModule_DeduplicatesBracketNotationImport 브래킷 표기 경로도 Ref 기반으로 dedup (수정 E)
func TestParseModule_DeduplicatesBracketNotationImport(t *testing.T) {
	source := `package test
import data["foo-bar"]

allow if { input.x }
`
	mod, err := ParseModule("test.rego", source, []string{`data["foo-bar"]`})
	if err != nil {
		t.Fatalf("ParseModule error: %v", err)
	}
	count := 0
	target := ast.MustParseRef(`data["foo-bar"]`)
	for _, imp := range mod.Imports {
		if ref, ok := imp.Path.Value.(ast.Ref); ok && ref.Equal(target) {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected bracket-notation import to be deduplicated to 1, got %d", count)
	}
}

// TestParseModule_AliasConflictErrors 기존 import alias와 이름 충돌 시 사전 검사 에러 (수정 E)
func TestParseModule_AliasConflictErrors(t *testing.T) {
	source := `package test
import data.other as helper

result := helper.value
`
	_, err := ParseModule("test.rego", source, []string{"data.helper"})
	if err == nil {
		t.Fatal("expected alias conflict error, got nil")
	}
	if !strings.Contains(err.Error(), "helper") {
		t.Errorf("error should mention conflicting name, got: %v", err)
	}
	if !strings.Contains(err.Error(), "data.other") {
		t.Errorf("error should mention the existing conflicting import, got: %v", err)
	}
}

// TestParseModule_UnknownFeatureViaInjectedImportErrors 주입 imports 경로로 들어온
// feature 오타도 에러가 나야 함 (검증 순서 수정: validate가 import 주입 뒤에 실행)
func TestParseModule_UnknownFeatureViaInjectedImportErrors(t *testing.T) {
	source := `package test
allow if { input.x }
`
	_, err := ParseModule("test.rego", source, []string{"data.regobrick.default_flase"})
	if err == nil {
		t.Fatal("expected error for unknown regobrick feature via injected import, got nil")
	}
	if !strings.Contains(err.Error(), "data.regobrick.default_flase") {
		t.Errorf("error should mention the offending import, got: %v", err)
	}
	if !strings.Contains(err.Error(), "default_false") {
		t.Errorf("error should list known features, got: %v", err)
	}
}

// TestParseModule_FeatureViaInjectedImportApplies 주입 imports로 정상 feature를
// 전달하면 transform이 적용되어야 함
func TestParseModule_FeatureViaInjectedImportApplies(t *testing.T) {
	source := `package test
allow if { input.x }
`
	mod, err := ParseModule("test.rego", source, []string{"data.regobrick.default_false"})
	if err != nil {
		t.Fatalf("ParseModule error: %v", err)
	}

	hasDefault := false
	for _, r := range mod.Rules {
		if r.Default && r.Head.Ref().String() == "allow" {
			hasDefault = true
		}
	}
	if !hasDefault {
		t.Error("expected default_false transform to apply for injected feature import")
	}

	// 마커 import는 최종 AST에서 제거되어야 함
	for _, imp := range mod.Imports {
		if ref, ok := imp.Path.Value.(ast.Ref); ok && ref.String() == "data.regobrick.default_false" {
			t.Errorf("expected injected marker import to be removed, but it remains: %s", imp.String())
		}
	}
}

// TestParseModule_NonDataInputImportPathErrors import 경로는 data 또는 input으로
// 시작해야 함 (isImportRef head 검사)
func TestParseModule_NonDataInputImportPathErrors(t *testing.T) {
	source := `package test
allow if { input.x }
`
	for _, path := range []string{"foo.bar", "abc"} {
		_, err := ParseModule("test.rego", source, []string{path})
		if err == nil {
			t.Errorf("expected error for non data/input import path %q, got nil", path)
			continue
		}
		if !strings.Contains(err.Error(), path) {
			t.Errorf("error should mention the offending path %q, got: %v", path, err)
		}
	}

	// input 루트는 유효해야 함
	mod, err := ParseModule("test.rego", source, []string{"input.user"})
	if err != nil {
		t.Fatalf("expected input-rooted import to be accepted, got: %v", err)
	}
	found := false
	target := ast.MustParseRef("input.user")
	for _, imp := range mod.Imports {
		if ref, ok := imp.Path.Value.(ast.Ref); ok && ref.Equal(target) {
			found = true
		}
	}
	if !found {
		t.Error("expected input.user import to be added")
	}
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
