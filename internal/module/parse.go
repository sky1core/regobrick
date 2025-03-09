// Package module provides utilities for parsing and transforming Rego modules,
// particularly for detecting and applying regobrick features like "default_false".
package module

import (
	"fmt"
	"strings"

	"github.com/open-policy-agent/opa/v1/ast"
)

// ParseModule parses the provided Rego source into an AST module.
// It optionally appends additional imports, and applies the "default_false"
// transformation if "import data.regobrick.default_false" is detected.
func ParseModule(filename, source string, imports []string) (*ast.Module, error) {
	// 1) Parse the Rego source.
	mod, err := ast.ParseModule(filename, source)
	if err != nil {
		return nil, fmt.Errorf("parse error in %q: %w", filename, err)
	}
	if mod == nil {
		return nil, fmt.Errorf("got nil module for %q", filename)
	}

	// 2) Add user-specified imports (e.g., "data.xxx.yyy").
	for _, path := range imports {
		addImport(mod, path)
	}

	// 3) If the module imports "data.regobrick.default_false", apply default_false transform.
	if hasRegobrickFeature(mod, "default_false") {
		addDefaultFalse(mod)
	}

	return mod, nil
}

// addImport splits a path like "data.xxx.yyy" and adds it as an import to the module.
func addImport(mod *ast.Module, importPath string) {
	if importPath == "" {
		return
	}

	parts := strings.Split(importPath, ".")
	var ref ast.Ref

	for i, p := range parts {
		// If the first segment is "data", treat it as a VarTerm
		if i == 0 {
			ref = append(ref, ast.VarTerm(p))
		} else {
			ref = append(ref, ast.StringTerm(p))
		}
	}

	mod.Imports = append(mod.Imports, &ast.Import{
		Path: ast.NewTerm(ref),
	})
}

// hasRegobrickFeature returns true if the module imports "data.regobrick.<feature>".
func hasRegobrickFeature(mod *ast.Module, feature string) bool {
	target := "data.regobrick." + feature
	for _, imp := range mod.Imports {
		if ref, ok := imp.Path.Value.(ast.Ref); ok && ref.String() == target {
			return true
		}
	}
	return false
}

// addDefaultFalse inserts a "default <rule> = false" rule for each if-rule without an existing default.
// In Rego, an if-rule is defined when Head.Key == "if" or no key is specified for a boolean rule.
func addDefaultFalse(mod *ast.Module) {
	existing := make(map[string]bool)
	for _, r := range mod.Rules {
		if r.Default {
			existing[r.Head.Ref().String()] = true
		}
	}

	for _, r := range mod.Rules {
		// Skip function rules (those having arguments).
		if len(r.Head.Args) > 0 {
			continue
		}

		isIfRule := false
		if r.Head.Key != nil {
			if s, ok := r.Head.Key.Value.(ast.String); ok && string(s) == "if" {
				isIfRule = true
			}
		} else {
			// Boolean rule => treat as an if-rule.
			isIfRule = true
		}

		if isIfRule {
			refVal := r.Head.Ref()
			if refVal == nil {
				continue
			}
			refStr := refVal.String()

			// Skip if there's already a default rule for this reference.
			if existing[refStr] {
				continue
			}

			// Create a default rule, e.g. "default if = false".
			newHead := &ast.Head{
				Reference: refVal,
				Value:     ast.BooleanTerm(false),
			}
			newRule := &ast.Rule{
				Default: true,
				Head:    newHead,
			}
			mod.Rules = append(mod.Rules, newRule)
			existing[refStr] = true
		}
	}
}
