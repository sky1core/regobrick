// Package module provides utilities for parsing and transforming Rego modules,
// particularly for detecting and applying regobrick features like "default_false".
package module

import (
	"fmt"
	"slices"
	"strings"

	"github.com/open-policy-agent/opa/v1/ast"
)

// featureDefaultFalse is the regobrick feature that inserts "default <rule> = false"
// rules for boolean rules.
const featureDefaultFalse = "default_false"

// regobrickImportPrefix is the marker import prefix used to enable regobrick
// features, e.g. "import data.regobrick.default_false".
const regobrickImportPrefix = "data.regobrick."

// knownFeatures lists every feature that may appear under regobrickImportPrefix.
// Any import under that prefix that is not in this list is rejected by ParseModule.
var knownFeatures = []string{featureDefaultFalse}

func isKnownFeature(feature string) bool {
	return slices.Contains(knownFeatures, feature)
}

// ParseModule parses the provided Rego source into an AST module.
// It optionally appends additional imports, and applies the "default_false"
// transformation if "import data.regobrick.default_false" is detected.
//
// ParseModule never panics: any failure (parse error, invalid injected import
// path, import name conflict, or an unknown regobrick feature) is returned as an
// error.
func ParseModule(filename, source string, imports []string) (*ast.Module, error) {
	// 1) Parse the Rego source. ProcessAnnotation keeps "# METADATA" annotations
	//    attached to the AST instead of silently dropping them.
	mod, err := ast.ParseModuleWithOpts(filename, source, ast.ParserOptions{ProcessAnnotation: true})
	if err != nil {
		return nil, fmt.Errorf("parse error in %q: %w", filename, err)
	}
	if mod == nil {
		return nil, fmt.Errorf("got nil module for %q", filename)
	}

	// 2) Add user-specified imports (e.g., "data.xxx.yyy").
	for _, path := range imports {
		if err := addImport(mod, path); err != nil {
			return nil, err
		}
	}

	// 3) Reject unknown regobrick feature imports so a typo like
	//    "data.regobrick.default_flase" fails loudly instead of silently doing
	//    nothing. This runs after import injection so both source imports and
	//    injected imports are validated.
	if err := validateRegobrickFeatures(mod); err != nil {
		return nil, err
	}

	// 4) If the module imports "data.regobrick.default_false", apply default_false transform.
	if hasRegobrickFeature(mod, featureDefaultFalse) {
		addDefaultFalse(mod)
	}

	// 5) Drop the regobrick marker imports. They only trigger transforms and are
	//    unused in the resulting AST, which would otherwise break rego.Strict(true).
	removeRegobrickImports(mod)

	return mod, nil
}

// validateRegobrickFeatures returns an error if the module imports anything under
// "data.regobrick." that is not a known feature.
func validateRegobrickFeatures(mod *ast.Module) error {
	for _, imp := range mod.Imports {
		ref, ok := imp.Path.Value.(ast.Ref)
		if !ok {
			continue
		}
		s := ref.String()
		if !strings.HasPrefix(s, regobrickImportPrefix) {
			continue
		}
		feature := strings.TrimPrefix(s, regobrickImportPrefix)
		if !isKnownFeature(feature) {
			return fmt.Errorf(
				"regobrick: unknown feature import %q (known features: %s)",
				s, strings.Join(knownFeatures, ", "),
			)
		}
	}
	return nil
}

// removeRegobrickImports strips every "data.regobrick.*" marker import from the module.
func removeRegobrickImports(mod *ast.Module) {
	kept := mod.Imports[:0]
	for _, imp := range mod.Imports {
		if ref, ok := imp.Path.Value.(ast.Ref); ok && strings.HasPrefix(ref.String(), regobrickImportPrefix) {
			continue
		}
		kept = append(kept, imp)
	}
	mod.Imports = kept
}

// addImport parses importPath into an ast.Ref and appends it as an import to the
// module. It deduplicates against existing imports using Ref equality, and reports
// an error when the path is invalid or when it would shadow an existing import that
// binds the same local name to a different reference.
func addImport(mod *ast.Module, importPath string) error {
	if importPath == "" {
		return nil
	}

	ref, err := ast.ParseRef(importPath)
	if err != nil {
		return fmt.Errorf("regobrick: invalid import path %q: %w", importPath, err)
	}
	if !isImportRef(ref) {
		return fmt.Errorf("regobrick: invalid import path %q: import must be a reference to a data path", importPath)
	}

	newImp := &ast.Import{Path: ast.NewTerm(ref)}
	newName := newImp.Name()

	for _, imp := range mod.Imports {
		existingRef, ok := imp.Path.Value.(ast.Ref)
		if !ok {
			continue
		}
		if existingRef.Equal(ref) {
			// Same target. Deduplicate when the existing import binds the default
			// local name (no alias, or an alias equal to the default name).
			if imp.Alias == "" || imp.Alias.Equal(newName) {
				return nil
			}
			// Existing import is aliased to a different name (e.g. "as h"); keep both.
			continue
		}
		// Different target but the same local binding name -> unresolvable shadow.
		if imp.Name().Equal(newName) {
			return fmt.Errorf(
				"regobrick: injected import %q conflicts with existing import %q (both bind name %q)",
				importPath, imp.String(), string(newName),
			)
		}
	}

	mod.Imports = append(mod.Imports, newImp)
	return nil
}

// isImportRef reports whether ref has the shape of a valid import path: a "data"
// or "input" head followed by String terms (e.g. data.foo, data["foo-bar"].baz).
// Rego imports must be rooted at data or input.
func isImportRef(ref ast.Ref) bool {
	if len(ref) == 0 {
		return false
	}
	head, ok := ref[0].Value.(ast.Var)
	if !ok || (!head.Equal(ast.DefaultRootDocument.Value) && !head.Equal(ast.InputRootDocument.Value)) {
		return false
	}
	for _, t := range ref[1:] {
		if _, ok := t.Value.(ast.String); !ok {
			return false
		}
	}
	return true
}

// hasRegobrickFeature returns true if the module imports "data.regobrick.<feature>".
func hasRegobrickFeature(mod *ast.Module, feature string) bool {
	target := regobrickImportPrefix + feature
	for _, imp := range mod.Imports {
		if ref, ok := imp.Path.Value.(ast.Ref); ok && ref.String() == target {
			return true
		}
	}
	return false
}

// addDefaultFalse inserts a "default <rule> = false" rule for each boolean rule without an existing default.
// A boolean rule is one where Head.Key is nil and Head.Value is nil or Boolean type.
// Complete rules (e.g., x := 1) are excluded since their Head.Value is a non-boolean type.
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

		// Boolean rule 판별:
		// - Head.Key가 nil (partial rule이 아님)
		// - Head.Value가 nil이거나 Boolean 타입
		// Complete rule (x := 1)은 Head.Value가 Number/String 등이므로 제외
		isBooleanRule := false
		if r.Head.Key == nil {
			if r.Head.Value == nil {
				isBooleanRule = true
			} else if _, ok := r.Head.Value.Value.(ast.Boolean); ok {
				isBooleanRule = true
			}
		}

		if isBooleanRule {
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
