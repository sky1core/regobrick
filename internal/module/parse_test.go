package module

import (
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
)

// TestAddDefaultFalse_BooleanRule verifies that a default is added to a boolean rule
func TestAddDefaultFalse_BooleanRule(t *testing.T) {
	source := `package test
import data.regobrick.default_false

allow if { input.x == 1 }
`
	mod, err := ParseModule("test.rego", source, nil)
	if err != nil {
		t.Fatalf("ParseModule error: %v", err)
	}

	// default allow = false should be added
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

// TestAddDefaultFalse_ExistingDefault no duplicate is added when a default already exists
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

	// there should be exactly 1 default
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

// TestAddDefaultFalse_FunctionRule function rules are skipped
func TestAddDefaultFalse_FunctionRule(t *testing.T) {
	source := `package test
import data.regobrick.default_false

is_admin(user) if { user == "admin" }
`
	mod, err := ParseModule("test.rego", source, nil)
	if err != nil {
		t.Fatalf("ParseModule error: %v", err)
	}

	// a default should not be added to a function rule
	for _, r := range mod.Rules {
		if r.Default {
			t.Errorf("unexpected default rule: %v", r.Head.Ref())
		}
	}
}

// TestAddDefaultFalse_NoImport default_false is not applied without the import
func TestAddDefaultFalse_NoImport(t *testing.T) {
	source := `package test

allow if { input.x == 1 }
`
	mod, err := ParseModule("test.rego", source, nil)
	if err != nil {
		t.Fatalf("ParseModule error: %v", err)
	}

	// a default is not added without the import
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

// TestAddDefaultFalse_MultipleRules a default is added to each of several rules
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
// Existing-behavior guarantee tests - must still pass after modifications
// =============================================================================

// TestMustWork_BooleanRuleVariants various forms of boolean rules
func TestMustWork_BooleanRuleVariants(t *testing.T) {
	tests := []struct {
		name        string
		source      string
		wantDefault []string // names of rules that should have a default added
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
			wantDefault: []string{"allow"}, // only one should be added
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

			// verify that all expected defaults are present
			for _, want := range tt.wantDefault {
				if defaults[want] == 0 {
					t.Errorf("expected default for %q, but not found", want)
				}
				if defaults[want] > 1 {
					t.Errorf("duplicate default for %q: count=%d", want, defaults[want])
				}
			}

			// verify that no unexpected defaults are present
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

// TestMustWork_PartialRules no default is added to partial rules
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

// TestMustWork_MixedRules a mix of boolean and other rule types
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

	// only boolean rules should have a default
	if !defaults["allow"] {
		t.Error("expected default for 'allow'")
	}
	if !defaults["deny"] {
		t.Error("expected default for 'deny'")
	}

	// partial rules should not have a default
	if defaults["items"] {
		t.Error("unexpected default for partial set 'items'")
	}
	if defaults["config"] {
		t.Error("unexpected default for partial object 'config'")
	}
}

// TestMustWork_WithExistingDefault respects an existing default
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
// Regression tests reproducing the silent-failure fixes
// =============================================================================

// TestParseModule_PreservesAnnotations verifies that METADATA annotations are preserved (fix A)
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

// TestParseModule_RemovesMarkerImport verifies the default_false marker import is removed after transformation (fix B)
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

// TestParseModule_UnknownFeatureErrors a typo in an unknown feature errors instead of being silently ignored (fix C)
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

// TestParseModule_InvalidImportPathErrors an invalid injected import path errors (fix E)
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

// TestParseModule_DeduplicatesBracketNotationImport bracket-notation paths are also deduplicated based on Ref (fix E)
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

// TestParseModule_AliasConflictErrors a pre-check error occurs on a name conflict with an existing import alias (fix E)
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

// TestParseModule_UnknownFeatureViaInjectedImportErrors a feature typo coming in through the
// injected imports path must also error (validation order fix: validate runs after import injection)
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

// TestParseModule_FeatureViaInjectedImportApplies passing a valid feature via the injected imports
// should apply the transform
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

	// the marker import should be removed from the final AST
	for _, imp := range mod.Imports {
		if ref, ok := imp.Path.Value.(ast.Ref); ok && ref.String() == "data.regobrick.default_false" {
			t.Errorf("expected injected marker import to be removed, but it remains: %s", imp.String())
		}
	}
}

// TestParseModule_NonDataInputImportPathErrors an import path must start with data or
// input (isImportRef head check)
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

	// an input root should be valid
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
// Bug verification tests - record current behavior (for before/after comparison)
// =============================================================================

// TestBugFix_CompleteRuleNoDefault a default should not be added to a complete rule
// Bug: currently default x = false is added even to x := 1
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

// TestBugFix_ConditionalCompleteRuleNoDefault a default should not be added to a conditional complete rule either
// Bug: currently default x = false is added even to x := 1 if {...}
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

// TestBugFix_BooleanAssignmentNeedsDefault an explicit boolean assignment needs a default
// This is correct behavior - x := true if {...} should have a default
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
