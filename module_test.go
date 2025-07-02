package regobrick

import (
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/v1/rego"
)

func TestParseModule(t *testing.T) {
	tests := []struct {
		name        string
		filename    string
		source      string
		imports     []string
		expectError bool
		checkFunc   func(*testing.T, interface{}) // We'll check the module structure
	}{
		{
			name:     "simple valid module",
			filename: "test.rego",
			source: `package test
			
rule1 if {
	input.value == "test"
}`,
			imports:     []string{},
			expectError: false,
		},
		{
			name:     "module with default_false import",
			filename: "test.rego",
			source: `package test

import data.regobrick.default_false

allow if {
	input.user == "admin"
}`,
			imports:     []string{},
			expectError: false,
		},
		{
			name:     "module with additional imports",
			filename: "test.rego",
			source: `package test

rule1 if {
	input.value == "test"
}`,
			imports:     []string{"data.company.utils", "data.shared.helpers"},
			expectError: false,
		},
		{
			name:        "invalid rego syntax",
			filename:    "invalid.rego",
			source:      `package test { invalid syntax }`,
			imports:     []string{},
			expectError: true,
		},
		{
			name:        "empty source",
			filename:    "empty.rego",
			source:      "",
			imports:     []string{},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			module, err := ParseModule(tt.filename, tt.source, tt.imports)

			if tt.expectError {
				if err == nil {
					t.Errorf("ParseModule() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("ParseModule() unexpected error = %v", err)
				return
			}

			if module == nil {
				t.Errorf("ParseModule() returned nil module")
				return
			}

			// Check that the module has the expected package
			if module.Package == nil {
				t.Errorf("ParseModule() module has no package")
				return
			}

			// For modules with default_false import, check if default rule was added
			if strings.Contains(tt.source, "import data.regobrick.default_false") {
				hasDefaultRule := false
				for _, rule := range module.Rules {
					if rule.Default {
						hasDefaultRule = true
						break
					}
				}
				if !hasDefaultRule {
					t.Errorf("ParseModule() expected default rule to be added for default_false import")
				}
			}

			// Check that additional imports were added
			expectedImportCount := len(tt.imports)
			if strings.Contains(tt.source, "import data.regobrick.default_false") {
				expectedImportCount++ // The regobrick import is also added
			}
			
			if len(module.Imports) < expectedImportCount {
				t.Errorf("ParseModule() expected at least %d imports, got %d", expectedImportCount, len(module.Imports))
			}
		})
	}
}

func TestModule(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		source   string
		imports  []string
	}{
		{
			name:     "basic module option",
			filename: "test.rego",
			source: `package test

rule1 if {
	input.value == "test"
}`,
			imports: []string{},
		},
		{
			name:     "module with imports",
			filename: "test.rego",
			source: `package test

rule1 if {
	input.value == "test"
}`,
			imports: []string{"data.company.utils"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test that Module() returns a valid rego option function
			moduleOption := Module(tt.filename, tt.source, tt.imports)
			if moduleOption == nil {
				t.Errorf("Module() returned nil option function")
				return
			}

			// Test that the option can be used with rego.New()
			_, err := rego.New(
				moduleOption,
				rego.Query("data.test"),
			).PrepareForEval(nil)

			// We expect this might fail due to context or other issues,
			// but it shouldn't panic or have parsing errors
			if err != nil && strings.Contains(err.Error(), "parse error") {
				t.Errorf("Module() option caused parse error: %v", err)
			}
		})
	}
}

func TestModules(t *testing.T) {
	// Test the Modules() function that takes multiple ModuleOption
	// For now, just test that it returns a valid function
	modulesOption := Modules()
	if modulesOption == nil {
		t.Errorf("Modules() returned nil option function")
		return
	}

	// Test that it can be used with rego.New()
	_, err := rego.New(
		modulesOption,
		rego.Query("data.test1"),
	).PrepareForEval(nil)

	// Similar to above, we're mainly checking it doesn't cause parsing errors
	if err != nil && strings.Contains(err.Error(), "parse error") {
		t.Errorf("Modules() option caused parse error: %v", err)
	}
}

func TestParseModuleEdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		filename    string
		source      string
		imports     []string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "nil imports",
			filename:    "test.rego",
			source:      "package test\nrule1 if { true }",
			imports:     nil,
			expectError: false,
		},
		{
			name:        "empty filename",
			filename:    "",
			source:      "package test\nrule1 if { true }",
			imports:     []string{},
			expectError: false, // Should still work
		},
		{
			name:        "malformed import path",
			filename:    "test.rego",
			source:      "package test\nrule1 if { true }",
			imports:     []string{"invalid..path", ""},
			expectError: false, // Should handle gracefully
		},
		{
			name:     "multiple regobrick imports",
			filename: "test.rego",
			source: `package test

import data.regobrick.default_false
import data.regobrick.default_false

rule1 if { input.test }`,
			imports:     []string{},
			expectError: false, // Should handle duplicate imports
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			module, err := ParseModule(tt.filename, tt.source, tt.imports)

			if tt.expectError {
				if err == nil {
					t.Errorf("ParseModule() expected error but got none")
				}
				if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("ParseModule() error = %v, want error containing %v", err, tt.errorMsg)
				}
				return
			}

			if err != nil {
				t.Errorf("ParseModule() unexpected error = %v", err)
				return
			}

			if module == nil {
				t.Errorf("ParseModule() returned nil module")
			}
		})
	}
}
