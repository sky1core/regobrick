package regobrick

import (
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
)

// Sentinels for silent drift in the OPA version this package is built against.
//
// regobrick couples to OPA's builtin metadata in two places that carry no
// version marker of their own: registerDecimalBuiltins (operator.go) replaces a
// fixed set of builtin implementations, and FilterCapabilities (capabilities.go)
// selects builtins by OPA's own Categories metadata. Both were written against
// OPA v1.11.0. An OPA bump that renames a builtin, changes an arity, or adds a
// builtin to a category regobrick treats as its own semantic area would shift
// the behavior without failing anything. These tests pin that state so a bump
// fails here, at build time, with an explanation of the decision to make.

// overriddenBuiltinArity pins every OPA builtin that registerDecimalBuiltins
// replaces, by literal name, together with the arity its implementation assumes.
//
// The names are string literals rather than ast.Plus.Name and friends on
// purpose: if an OPA release renamed a builtin, an expectation written in terms
// of the ast constants would follow the rename silently, whereas these literals
// break. Arity is pinned because every precision* implementation indexes
// operands positionally (operands[0], operands[1]), so a signature change would
// mean reading a missing operand or ignoring a new one.
//
// Scope limit: this map is maintained by hand alongside registerDecimalBuiltins.
// It catches OPA-side drift — a rename, a removal, an arity change — but cannot
// by itself notice a new override added to registerDecimalBuiltins without a
// matching entry here, because topdown's builtin registry offers no way to
// enumerate what this package registered into it.
var overriddenBuiltinArity = map[string]int{
	// Arithmetic
	"plus":  2,
	"minus": 2,
	"mul":   2,
	"div":   2,
	"rem":   2,

	// Comparison
	"gt":    2,
	"gte":   2,
	"lt":    2,
	"lte":   2,
	"equal": 2,
	"neq":   2,

	// Unary
	"abs":   1,
	"round": 1,
	"ceil":  1,
	"floor": 1,

	// Aggregates
	"sum":     1,
	"product": 1,
	"max":     1,
	"min":     1,
}

// overriddenBuiltinCount is asserted separately from the map so that dropping an
// entry shows up as a count mismatch rather than as a quietly smaller loop.
const overriddenBuiltinCount = 19

func TestOverriddenBuiltinsStillExistWithPinnedArity(t *testing.T) {
	if len(overriddenBuiltinArity) != overriddenBuiltinCount {
		t.Fatalf("overriddenBuiltinArity has %d entries, want %d: it must list exactly the builtins "+
			"registerDecimalBuiltins overrides, so update both together",
			len(overriddenBuiltinArity), overriddenBuiltinCount)
	}

	for name, wantArity := range overriddenBuiltinArity {
		b, ok := ast.BuiltinMap[name]
		if !ok {
			t.Errorf("OPA no longer defines the builtin %q, which registerDecimalBuiltins overrides. "+
				"topdown.RegisterBuiltinFunc for a name OPA does not define is never consulted during "+
				"evaluation, so the override would simply stop applying, with no error anywhere.", name)
			continue
		}

		// ast.BuiltinMap is keyed by both Name and Infix, so a lookup that
		// succeeds does not by itself prove the key is the canonical name.
		if b.Name != name {
			t.Errorf("ast.BuiltinMap[%q] resolves to the builtin named %q; %q is an alias (an infix "+
				"symbol) rather than the canonical name registerDecimalBuiltins must register under",
				name, b.Name, name)
			continue
		}

		if got := b.Decl.Arity(); got != wantArity {
			t.Errorf("OPA builtin %q now has arity %d, pinned at %d. The precision* implementation "+
				"registered for it reads its operands positionally, so it must be reviewed against "+
				"the new signature before this expectation is updated.", name, got, wantArity)
		}
	}
}

// TestOverriddenBuiltinASTVarsStillCarryPinnedNames closes the loop between the
// literal names pinned above and the ast package variables registerDecimalBuiltins
// actually registers through. Without it, an OPA release that repointed
// ast.Plus.Name from "plus" to something else would leave the pinned literals
// passing while the override moved to a different builtin, or to none.
func TestOverriddenBuiltinASTVarsStillCarryPinnedNames(t *testing.T) {
	// Mirrors registerDecimalBuiltins, in the same order.
	decls := []*ast.Builtin{
		ast.Plus, ast.Minus, ast.Multiply, ast.Divide, ast.Rem,
		ast.GreaterThan, ast.GreaterThanEq, ast.LessThan, ast.LessThanEq, ast.Equal, ast.NotEqual,
		ast.Abs, ast.Round, ast.Ceil, ast.Floor,
		ast.Sum, ast.Product, ast.Max, ast.Min,
	}

	if len(decls) != overriddenBuiltinCount {
		t.Fatalf("this test lists %d ast builtin vars, want %d: it must mirror registerDecimalBuiltins",
			len(decls), overriddenBuiltinCount)
	}

	got := make(map[string]int, len(decls))
	for _, b := range decls {
		got[b.Name] = b.Decl.Arity()
	}
	if len(got) != len(decls) {
		t.Fatalf("the %d ast builtin vars registerDecimalBuiltins uses collapsed to %d distinct names, "+
			"so at least two of them now point at the same builtin", len(decls), len(got))
	}

	if !reflect.DeepEqual(got, overriddenBuiltinArity) {
		t.Errorf("the ast builtin vars registerDecimalBuiltins registers through no longer match the "+
			"pinned names/arities.\n  from ast vars: %v\n  pinned:        %v\n"+
			"An OPA release repointed one of these vars; confirm which builtin the override now lands "+
			"on before updating the expectation.", sortedPairs(got), sortedPairs(overriddenBuiltinArity))
	}
}

// expectedBuiltinsByCategory pins the membership of the OPA builtin categories
// that cover regobrick's semantic area: the numeric, comparison, and aggregate
// operations UseDecimalArithmetic replaces, and that FilterCapabilities selects
// on. Values are the sorted canonical names in OPA v1.11.0.
//
// "minus" carries both "sets" and "numbers"; it is listed here under "numbers"
// only, because the set-difference half of precisionMinus is deliberately left
// on OPA's original behavior and the "sets" category is not regobrick's area.
var expectedBuiltinsByCategory = map[string][]string{
	"numbers":    {"abs", "ceil", "div", "floor", "minus", "mul", "plus", "rand.intn", "rem", "round"},
	"comparison": {"equal", "gt", "gte", "lt", "lte", "neq"},
	"aggregates": {"count", "max", "min", "product", "sort", "sum"},
}

// expectedNumbersNamespaceBuiltins pins the "numbers." namespace by name.
//
// OPA is inconsistent about Categories rather than following a namespacing
// convention: in v1.11.0, 98 of its 201 builtins carry none at all, and the
// split does not track namespacing in either direction — 48 namespaced builtins
// do carry a category (rand.intn is in "numbers", pinned above), while 13
// bare-named ones carry none (eq, assign, re_match, set_diff, the cast_*
// family, and others). numbers.range and numbers.range_step happen to fall in
// the uncategorized group, so a category-only scan would not notice a new
// numeric builtin added beside them.
//
// Scope limit: this prefix check closes that gap for the "numbers." namespace
// and nothing else. Uncategorized builtins elsewhere, bare-named ones included,
// are not pinned by this file — do not read this check as covering them.
var expectedNumbersNamespaceBuiltins = []string{"numbers.range", "numbers.range_step"}

// numbersNamespacePrefix is the name prefix used to reach the numeric builtins
// that carry no Categories of their own.
const numbersNamespacePrefix = "numbers."

func TestNumericBuiltinCategoriesHaveNotDrifted(t *testing.T) {
	// Scanned over ast.DefaultBuiltins rather than
	// ast.CapabilitiesForThisVersion().Builtins, which is the same set plus
	// anything registered at runtime: this test binary alone adds two dozen
	// custom builtins, and a regobrick user's would add their own. Only OPA's
	// own builtins can drift with an OPA bump, which is what is being pinned.
	byCategory := map[string][]string{}
	var numbersNamespace []string

	for _, b := range &ast.DefaultBuiltins {
		for _, cat := range b.Categories {
			if _, tracked := expectedBuiltinsByCategory[cat]; tracked {
				byCategory[cat] = append(byCategory[cat], b.Name)
			}
		}
		if strings.HasPrefix(b.Name, numbersNamespacePrefix) {
			numbersNamespace = append(numbersNamespace, b.Name)
		}
	}

	for cat, want := range expectedBuiltinsByCategory {
		got := byCategory[cat]
		sort.Strings(got)
		assertBuiltinSetPinned(t, "category "+strconv.Quote(cat), got, want)
	}

	sort.Strings(numbersNamespace)
	assertBuiltinSetPinned(t, "name prefix "+strconv.Quote(numbersNamespacePrefix), numbersNamespace, expectedNumbersNamespaceBuiltins)
}

// assertBuiltinSetPinned reports how a pinned builtin group drifted, and what
// decision the drift demands.
func assertBuiltinSetPinned(t *testing.T, group string, got, want []string) {
	t.Helper()

	if reflect.DeepEqual(got, want) {
		return
	}

	added := missingFrom(got, want)
	removed := missingFrom(want, got)

	t.Errorf("OPA's %s no longer holds the builtin set regobrick was written against.\n"+
		"  added by this OPA version:   %v\n"+
		"  removed by this OPA version: %v\n"+
		"  full current set:            %v\n"+
		"This failure is the point of the test, not a bug in it. That group is a semantic area "+
		"UseDecimalArithmetic takes over, so each addition is a decision to make explicitly: does the "+
		"new builtin need a decimal override in registerDecimalBuiltins (operator.go), and does "+
		"FilterCapabilities (capabilities.go) still select the right set? A removal matters for the "+
		"same reason — an override registered for a name OPA no longer defines never runs. Make the "+
		"call, then update the expectation here to match.", group, added, removed, got)
}

// missingFrom returns the elements of a that do not appear in b.
func missingFrom(a, b []string) []string {
	inB := make(map[string]struct{}, len(b))
	for _, s := range b {
		inB[s] = struct{}{}
	}
	out := []string{}
	for _, s := range a {
		if _, ok := inB[s]; !ok {
			out = append(out, s)
		}
	}
	return out
}

// sortedPairs renders a name→arity map deterministically for failure output.
func sortedPairs(m map[string]int) []string {
	names := make([]string, 0, len(m))
	for name := range m {
		names = append(names, name)
	}
	sort.Strings(names)

	out := make([]string, 0, len(names))
	for _, name := range names {
		out = append(out, name+"/"+strconv.Itoa(m[name]))
	}
	return out
}
