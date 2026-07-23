package module

import (
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
)

// Direct unit tests for branches that the public-API tests (root package) do
// not reach from inside this package: parse failures, fabricated non-ref
// imports, and isImportRef edge shapes.

func TestParseModule_ParseError(t *testing.T) {
	_, err := ParseModule("bad.rego", "package", nil)
	if err == nil {
		t.Fatal("expected parse error, got nil")
	}
	if !strings.Contains(err.Error(), "bad.rego") {
		t.Fatalf("error should name the file: %v", err)
	}
}

func TestValidateRegobrickFeatures_NonRefImportIgnored(t *testing.T) {
	mod := ast.MustParseModule(`package t
import data.foo`)
	// A fabricated import whose path is not a Ref must be skipped, not crash
	// or be misread as a regobrick feature.
	mod.Imports = append(mod.Imports, &ast.Import{Path: ast.StringTerm("not-a-ref")})
	if err := validateRegobrickFeatures(mod); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAddImport_EmptyPathIsNoop(t *testing.T) {
	mod := ast.MustParseModule(`package t`)
	if err := addImport(mod, ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mod.Imports) != 0 {
		t.Fatalf("empty path must not add an import, got %v", mod.Imports)
	}
}

func TestAddImport_SkipsNonRefExistingImport(t *testing.T) {
	mod := ast.MustParseModule(`package t`)
	mod.Imports = append(mod.Imports, &ast.Import{Path: ast.StringTerm("junk")})

	if err := addImport(mod, "data.foo.bar"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	last := mod.Imports[len(mod.Imports)-1]
	ref, ok := last.Path.Value.(ast.Ref)
	if !ok || ref.String() != "data.foo.bar" {
		t.Fatalf("expected data.foo.bar appended, got %v", last)
	}
}

func TestIsImportRef_Edges(t *testing.T) {
	if isImportRef(ast.Ref{}) {
		t.Fatal("empty ref must not be a valid import ref")
	}
	if isImportRef(ast.Ref{ast.StringTerm("data")}) {
		t.Fatal("ref with a non-var head must not be a valid import ref")
	}
	// data.x[y]: tail contains a Var term, not a String.
	if isImportRef(ast.MustParseRef("data.x[y]")) {
		t.Fatal("ref with a non-string tail must not be a valid import ref")
	}
	if !isImportRef(ast.MustParseRef("input.x")) {
		t.Fatal("input-rooted ref must be a valid import ref")
	}
}
