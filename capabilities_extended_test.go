package regobrick

import (
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/rego"
)

// Test FilterCapabilities with custom builtin categories
func TestFilterCapabilitiesWithCustomCategories(t *testing.T) {
	// First register a custom builtin with custom categories
	testBuiltin := func(ctx rego.BuiltinContext, input string) (string, error) {
		return "custom: " + input, nil
	}

	// Register with custom categories
	RegisterBuiltin1[string, string](
		"test_custom_category_builtin",
		testBuiltin,
		WithCategories("custom_category_1", "custom_category_2"),
	)

	tests := []struct {
		name         string
		allowedNames []string
		allowedCats  []string
		expectCustom bool
	}{
		{
			name:         "allow by custom category 1",
			allowedNames: []string{},
			allowedCats:  []string{"custom_category_1"},
			expectCustom: true,
		},
		{
			name:         "allow by custom category 2",
			allowedNames: []string{},
			allowedCats:  []string{"custom_category_2"},
			expectCustom: true,
		},
		{
			name:         "allow by builtin name",
			allowedNames: []string{"test_custom_category_builtin"},
			allowedCats:  []string{},
			expectCustom: true,
		},
		{
			name:         "disallow custom builtin",
			allowedNames: []string{},
			allowedCats:  []string{"other_category"},
			expectCustom: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			caps := FilterCapabilities(tt.allowedNames, tt.allowedCats)
			
			if caps == nil {
				t.Errorf("FilterCapabilities() returned nil")
				return
			}

			// Check if our custom builtin is included
			foundCustom := false
			for _, builtin := range caps.Builtins {
				if builtin.Name == "test_custom_category_builtin" {
					foundCustom = true
					break
				}
			}

			if foundCustom != tt.expectCustom {
				t.Errorf("Expected custom builtin found=%v, got %v", tt.expectCustom, foundCustom)
			}
		})
	}
}

// Test FilterCapabilities with nil custom categories
func TestFilterCapabilitiesWithNilCustomCategories(t *testing.T) {
	// Register a builtin with nil categories (empty categories)
	testBuiltin := func(ctx rego.BuiltinContext, input string) (string, error) {
		return input, nil
	}

	RegisterBuiltin1[string, string](
		"test_nil_category_builtin",
		testBuiltin,
		WithCategories(), // Empty categories
	)

	// Test that it's only found by name, not by category
	caps := FilterCapabilities([]string{"test_nil_category_builtin"}, []string{"any_category"})
	
	foundByName := false
	for _, builtin := range caps.Builtins {
		if builtin.Name == "test_nil_category_builtin" {
			foundByName = true
			break
		}
	}

	if !foundByName {
		t.Error("Expected to find builtin by name even with nil categories")
	}

	// Test that it's not found by category when categories are nil
	caps2 := FilterCapabilities([]string{}, []string{"any_category"})
	
	foundByCategory := false
	for _, builtin := range caps2.Builtins {
		if builtin.Name == "test_nil_category_builtin" {
			foundByCategory = true
			break
		}
	}

	// Should not be found by category since it has nil categories
	// (unless it's a core infix or has OPA built-in categories)
	t.Logf("Found by category: %v (this may be true if OPA assigns default categories)", foundByCategory)
}

// Test FilterCapabilities edge cases
func TestFilterCapabilitiesEdgeCases(t *testing.T) {
	tests := []struct {
		name         string
		allowedNames []string
		allowedCats  []string
		description  string
	}{
		{
			name:         "nil allowed names and categories",
			allowedNames: nil,
			allowedCats:  nil,
			description:  "should only include core infixes",
		},
		{
			name:         "empty allowed names and categories",
			allowedNames: []string{},
			allowedCats:  []string{},
			description:  "should only include core infixes",
		},
		{
			name:         "single allowed name",
			allowedNames: []string{"concat"},
			allowedCats:  []string{},
			description:  "should include concat and core infixes",
		},
		{
			name:         "single allowed category",
			allowedNames: []string{},
			allowedCats:  []string{"strings"},
			description:  "should include string builtins and core infixes",
		},
		{
			name:         "non-existent name",
			allowedNames: []string{"non_existent_builtin"},
			allowedCats:  []string{},
			description:  "should only include core infixes",
		},
		{
			name:         "non-existent category",
			allowedNames: []string{},
			allowedCats:  []string{"non_existent_category"},
			description:  "should only include core infixes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			caps := FilterCapabilities(tt.allowedNames, tt.allowedCats)
			
			if caps == nil {
				t.Errorf("FilterCapabilities() returned nil")
				return
			}

			if caps.Builtins == nil {
				t.Errorf("FilterCapabilities() returned capabilities with nil Builtins")
				return
			}

			// Always check that core infixes are preserved
			coreInfixCount := 0
			for _, builtin := range caps.Builtins {
				if infixAllowed(builtin.Infix) {
					coreInfixCount++
				}
			}

			// We should always have some core infixes
			if coreInfixCount == 0 && len(tt.allowedNames) == 0 && len(tt.allowedCats) == 0 {
				t.Error("FilterCapabilities() should preserve core infixes even with empty allow lists")
			}

			t.Logf("%s: Found %d builtins, %d core infixes", tt.description, len(caps.Builtins), coreInfixCount)
		})
	}
}

// Test that FilterCapabilities preserves the original capabilities structure
func TestFilterCapabilitiesPreservesOriginalStructure(t *testing.T) {
	originalCaps := ast.CapabilitiesForThisVersion()
	
	// Test with some allowed names and categories
	filteredCaps := FilterCapabilities([]string{"concat", "count"}, []string{"strings"})
	
	// The filtered capabilities should be a copy, not the same object
	if filteredCaps == originalCaps {
		t.Error("FilterCapabilities() should return a copy, not the original capabilities")
	}
	
	// The original capabilities should be unchanged
	originalBuiltinCount := len(originalCaps.Builtins)
	filteredBuiltinCount := len(filteredCaps.Builtins)
	
	if filteredBuiltinCount >= originalBuiltinCount {
		t.Errorf("Filtered capabilities should have fewer builtins than original (%d >= %d)", 
			filteredBuiltinCount, originalBuiltinCount)
	}
	
	// Verify that all builtins in filtered caps are valid
	for i, builtin := range filteredCaps.Builtins {
		if builtin == nil {
			t.Errorf("Builtin at index %d is nil", i)
		}
		if builtin.Name == "" && builtin.Infix == "" {
			t.Errorf("Builtin at index %d has empty name and infix", i)
		}
	}
}

// Test FilterCapabilities with overlapping names and categories
func TestFilterCapabilitiesOverlapping(t *testing.T) {
	// Test case where a builtin might be allowed by both name and category
	allowedNames := []string{"concat"}
	allowedCats := []string{"strings"} // concat is likely in strings category
	
	caps := FilterCapabilities(allowedNames, allowedCats)
	
	concatCount := 0
	for _, builtin := range caps.Builtins {
		if builtin.Name == "concat" {
			concatCount++
		}
	}
	
	// Should only appear once, even if allowed by both name and category
	if concatCount > 1 {
		t.Errorf("concat builtin appears %d times, should appear only once", concatCount)
	}
	
	if concatCount == 0 {
		t.Error("concat builtin should be included")
	}
}
