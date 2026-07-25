package regobrick

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/topdown"
	"github.com/open-policy-agent/opa/v1/topdown/builtins"
	"github.com/quagmt/udecimal"
)

// ------------------------------------------------------------
// Decimal arithmetic options
// ------------------------------------------------------------

type decimalArithmeticConfig struct {
	stringCoercion bool
}

// stringCoercionEnabled is the process-global string coercion setting that every
// operand conversion on the evaluation path reads.
//
// It is atomic so that a UseDecimalArithmetic call made after evaluation has
// already started is a benign last-writer-wins toggle instead of a data race on
// a plain bool, which is undefined behavior. That is the only guarantee it adds:
// UseDecimalArithmetic also rewrites OPA's builtin registry without
// synchronization, so reconfiguring concurrently with evaluation stays unsafe.
var stringCoercionEnabled atomic.Bool

// DecimalArithmeticOption configures UseDecimalArithmetic behavior.
type DecimalArithmeticOption func(*decimalArithmeticConfig)

// WithStringCoercion enables automatic string-to-number coercion.
//
// Numeric strings (e.g., "0.73", "100") from input or data are
// automatically converted to numbers in arithmetic, comparison,
// unary, and aggregate operations.
//
//   - Applied to: +, -, *, /, %, >, >=, <, <=, abs, round, ceil, floor, sum, product, max, min
//   - NOT applied to: ==, != (different types are always unequal — standard OPA behavior)
//   - Non-numeric strings ("abc"): operation fails (undefined or eval error)
//
// String coercion is primarily intended for runtime values from input/data.
// Arithmetic, unary, and the sum/product aggregates declare numeric operand
// types, so string literals in Rego source code (e.g., "0.73" + 1,
// sum(["0.1"])) are rejected by OPA's compile-time type checker before our
// runtime coercion logic runs. This does not apply to max/min, whose operand is
// an Any collection: such literals reach runtime, where non-numeric or mixed
// collections fall back to the default comparison ordering.
func WithStringCoercion() DecimalArithmeticOption {
	return func(cfg *decimalArithmeticConfig) {
		cfg.stringCoercion = true
	}
}

// UseDecimalArithmetic replaces Rego's numeric operations with precision decimal operations.
//
// # Overloaded operators
//
//   - Arithmetic: +, -, *, /, %
//   - Comparison: >, >=, <, <=, ==, !=
//   - Unary: abs(), round(), ceil(), floor()
//   - Aggregates: sum(), product(), max(), min()
//
// # Standard OPA differences
//
// Standard OPA comparison operators (>, <, >=, <=) support all types
// using type ordering (null < bool < number < string < ...).
// With UseDecimalArithmetic, comparison operators become numeric-only,
// and non-numeric string comparisons ("a" < "b") will not work.
//
// The % (rem) operator accepts decimal operands (e.g. 10.5 % 3), whereas
// standard OPA restricts modulo to integers.
//
// # Precision limits (udecimal)
//
//   - Maximum 19 decimal places; values with more fail to parse
//     (default mode: no result; StrictBuiltinErrors: eval error).
//     Truncation (not rounding) applies only to operation results, e.g. 100/3.
//   - Magnitude: coefficients up to 128 bits
//     (±34,028,236,692,093,846,346.3374607431768211455 at the full 19 decimal
//     places) stay on udecimal's zero-allocation fast path; larger plain-notation
//     values fall back to udecimal's exact big.Int arithmetic instead of
//     failing, up to udecimal's input hard cap of 200 characters per number
//     string, beyond which parsing fails.
//   - Exponent notation is expanded to plain notation before parsing under a
//     64-character (≈62-digit) budget: 1e61 parses, 1e62 and beyond (e.g.
//     1e100) fail, while the same magnitude written out in plain notation
//     parses fine.
//
// # Error handling
//
//   - Default mode: operation failure results in rule not satisfied (no result)
//   - StrictBuiltinErrors(true): returns eval_builtin_error
//
// # Options
//
//   - WithStringCoercion(): auto-convert numeric strings to numbers
//
// # Usage
//
//	// Basic precision arithmetic
//	regobrick.UseDecimalArithmetic()
//
//	// With string-to-number coercion
//	regobrick.UseDecimalArithmetic(regobrick.WithStringCoercion())
//
// # Concurrency
//
// Call this once at application startup, before any evaluation begins.
//
// It writes two pieces of process-global state, with different guarantees. The
// coercion setting is stored atomically, so a late call cannot corrupt the flag
// reads on the evaluation path. OPA's builtin function registry, however, is
// rewritten without synchronization. Calling this concurrently with evaluation
// therefore remains unsafe, and "once at startup" is the contract rather than
// just a recommendation.
//
// # Fail-fast on changed udecimal defaults
//
// The parsing contract documented above relies on udecimal's default global
// settings. This function verifies them on entry: a 19-decimal-place value must
// parse, and a 20-decimal-place value must fail to parse. If either check fails
// — as it would after a udecimal.SetDefaultPrecision or
// udecimal.SetDefaultParseMode call elsewhere in the process — it panics with a
// message starting "regobrick: udecimal default settings have been changed",
// rather than silently degrading precision or truncating.
//
// The check is a snapshot taken at call time. Changing udecimal's defaults after
// this function returns cannot be detected, and would silently change the
// contract.
func UseDecimalArithmetic(opts ...DecimalArithmeticOption) {
	assertUdecimalDefaults()

	cfg := decimalArithmeticConfig{}
	for _, opt := range opts {
		opt(&cfg)
	}
	stringCoercionEnabled.Store(cfg.stringCoercion)
	registerDecimalBuiltins()
}

// Probe values for assertUdecimalDefaults, chosen to straddle udecimal's default
// precision of 19 fractional digits.
const (
	udecimalProbeAtPrecision   = "0.0000000000000000001"  // 19 dp: must parse
	udecimalProbeOverPrecision = "0.00000000000000000001" // 20 dp: must fail to parse
)

// assertUdecimalDefaults panics unless udecimal's process-global defaults still
// match the parsing contract documented on UseDecimalArithmetic.
//
// Two probes cover the two ways that contract can be broken. A lowered
// SetDefaultPrecision makes the 19-decimal-place probe stop parsing, silently
// shrinking the supported range. SetDefaultParseMode(ParseModeTrunc) makes the
// 20-decimal-place probe start parsing, silently truncating out-of-precision
// input instead of rejecting it — the failure mode that matters most, because it
// turns a loud parse error into a wrong number.
func assertUdecimalDefaults() {
	if _, err := udecimal.Parse(udecimalProbeAtPrecision); err != nil {
		panic(fmt.Sprintf("regobrick: udecimal default settings have been changed "+
			"(SetDefaultPrecision/SetDefaultParseMode?): parsing %q, whose 19 decimal "+
			"places the documented contract requires to be supported, failed with %v",
			udecimalProbeAtPrecision, err))
	}
	if _, err := udecimal.Parse(udecimalProbeOverPrecision); err == nil {
		panic(fmt.Sprintf("regobrick: udecimal default settings have been changed "+
			"(SetDefaultPrecision/SetDefaultParseMode?): parsing %q, whose 20 decimal "+
			"places exceed the documented 19-place limit, succeeded instead of failing, "+
			"so out-of-precision values would be silently truncated",
			udecimalProbeOverPrecision))
	}
}

// decimalBuiltinOverrides is the set of OPA builtins UseDecimalArithmetic
// replaces, each paired with the implementation that replaces it.
//
// It is the only place that set is written down. registerDecimalBuiltins just
// walks it, and the drift sentinel in opa_drift_sentinel_test.go derives its
// expectations from it, so adding or dropping an entry here fails that test until
// its pinned names are updated to match — which is the point, because changing
// this set changes which OPA behavior regobrick takes over.
//
// A slice rather than a map, so registration order is fixed rather than varying
// per run. The order carries no meaning today: the names are distinct, so no
// entry can overwrite another's registration.
var decimalBuiltinOverrides = []struct {
	decl *ast.Builtin
	impl topdown.BuiltinFunc
}{
	// Arithmetic operators
	{ast.Plus, precisionPlus},
	{ast.Minus, precisionMinus},
	{ast.Multiply, precisionMultiply},
	{ast.Divide, precisionDivide},
	{ast.Rem, precisionRem},

	// Comparison operators
	{ast.GreaterThan, precisionGT},
	{ast.GreaterThanEq, precisionGTE},
	{ast.LessThan, precisionLT},
	{ast.LessThanEq, precisionLTE},
	{ast.Equal, precisionEqual},
	{ast.NotEqual, precisionNotEqual},

	// Unary operators
	{ast.Abs, precisionAbs},
	{ast.Round, precisionRound},
	{ast.Ceil, precisionCeil},
	{ast.Floor, precisionFloor},

	// Aggregate operators
	{ast.Sum, precisionSum},
	{ast.Product, precisionProduct},
	{ast.Max, precisionMax},
	{ast.Min, precisionMin},
}

// registerDecimalBuiltins installs every decimalBuiltinOverrides entry into
// OPA's process-global builtin registry, under the name OPA declares for the
// builtin, replacing the stock implementation registered there.
func registerDecimalBuiltins() {
	for _, o := range decimalBuiltinOverrides {
		topdown.RegisterBuiltinFunc(o.decl.Name, o.impl)
	}
}

// maxExpandedLen is the upper bound on the string length of an expanded exponent
// notation result. It is an allocation guard, not a udecimal limit: without it a
// huge exponent from an input path (e.g. 1e2000000000) would trigger ~2GB of
// zero-string allocation via strings.Repeat. When the bound is exceeded the
// original string is returned unchanged and udecimal.Parse rejects the exponent
// notation as an invalid format.
//
// Consequence: exponent-notation inputs are capped at a ~62-digit expansion
// (1e61 parses; 1e62 and beyond, e.g. 1e100, fail) even though udecimal itself
// could represent them via its big.Int fallback. The same magnitude written in
// plain notation is unaffected by this cap. The bound comfortably covers the
// u128 fast path (~39 significant digits, up to 19 of them fractional).
const maxExpandedLen = 64

// expandExponent expands a scientific-notation number string into plain decimal
// notation. E.g. "1e-8"→"0.00000001", "-2.5E+3"→"-2500", "1.5e0"→"1.5".
//
// It shifts only the decimal point via pure string manipulation without going
// through float64, so there is no precision loss. Non-exponent (e.g. "123.45")
// or malformed strings are returned unchanged for udecimal.Parse to decide on.
//
// When the expanded result exceeds udecimal's precision (19 fractional digits,
// e.g. "1e-25"→"0.0000...1"), udecimal.Parse returns an error; this is the
// documented precision limit. An exponent whose expansion would exceed
// maxExpandedLen (e.g. "1e100", "1e2000000000") is not expanded and the
// original string is returned to avoid an allocation blowup; udecimal then
// rejects the un-expanded exponent notation as an invalid format.
func expandExponent(s string) string {
	ePos := strings.IndexAny(s, "eE")
	if ePos < 0 {
		return s
	}

	mantissa := s[:ePos]
	exp, err := strconv.Atoi(s[ePos+1:])
	if err != nil {
		return s // exponent is not an integer (including out-of-int-range) → let udecimal reject it
	}

	// Size guard 1: if the absolute exponent exceeds the bound, the expansion is
	// guaranteed to exceed it too, so do not expand. (This also blocks integer
	// overflow in the later newExp computation.)
	if exp > maxExpandedLen || exp < -maxExpandedLen {
		return s
	}

	// Separate the sign of the mantissa.
	sign := ""
	if len(mantissa) > 0 && (mantissa[0] == '+' || mantissa[0] == '-') {
		if mantissa[0] == '-' {
			sign = "-"
		}
		mantissa = mantissa[1:]
	}

	// Separate the integer and fractional parts (substrings of s, not copies).
	intPart := mantissa
	fracPart := ""
	if dot := strings.IndexByte(mantissa, '.'); dot >= 0 {
		intPart = mantissa[:dot]
		fracPart = mantissa[dot+1:]
	}

	digitsLen := len(intPart) + len(fracPart)
	if digitsLen == 0 {
		return s
	}

	// value = digits × 10^(exp - len(fracPart))
	newExp := exp - len(fracPart)

	// Size guard 2: precompute the expansion length from substring lengths and
	// bail out BEFORE concatenating, so an oversized mantissa never triggers a
	// large intermediate allocation.
	// (newExp >= 0: digitsLen+newExp, newExp < 0: at most digitsLen+|newExp|+2)
	absNewExp := newExp
	if absNewExp < 0 {
		absNewExp = -absNewExp
	}
	if digitsLen+absNewExp+2 > maxExpandedLen {
		return s
	}

	for _, part := range [2]string{intPart, fracPart} {
		for i := 0; i < len(part); i++ {
			if part[i] < '0' || part[i] > '9' {
				return s // non-digit character in the mantissa → let udecimal reject it
			}
		}
	}

	// Bounded by the size guard above.
	digits := intPart + fracPart

	var out string
	if newExp >= 0 {
		out = digits + strings.Repeat("0", newExp)
	} else {
		k := -newExp
		if len(digits) > k {
			out = digits[:len(digits)-k] + "." + digits[len(digits)-k:]
		} else {
			out = "0." + strings.Repeat("0", k-len(digits)) + digits
		}
	}
	return sign + out
}

// parseDecimal parses a numeric string into a udecimal.Decimal.
// It first expands exponent notation into plain decimal notation before passing
// it to udecimal.Parse, so exponent notation like "1e-8" is parsed accurately.
func parseDecimal(s string) (udecimal.Decimal, error) {
	return udecimal.Parse(expandExponent(s))
}

// toDecimal converts an ast.Value into a udecimal.Decimal.
// It attempts to parse both ast.Number and ast.String, returning false on
// failure. As a low-level helper, the stringCoercion condition is decided by the
// caller (isNumericType, etc.).
func toDecimal(v ast.Value) (udecimal.Decimal, bool) {
	switch val := v.(type) {
	case ast.Number:
		d, err := parseDecimal(string(val))
		if err != nil {
			return udecimal.Decimal{}, false
		}
		return d, true
	case ast.String:
		d, err := parseDecimal(string(val))
		if err != nil {
			return udecimal.Decimal{}, false
		}
		return d, true
	default:
		return udecimal.Decimal{}, false
	}
}

// isNumericType reports whether a value should be handled in numeric mode.
// ast.Number is always true (it is a numeric type even if parsing fails).
// ast.String is true only when stringCoercion is enabled and it parses as a
// number.
func isNumericType(v ast.Value) bool {
	switch v.(type) {
	case ast.Number:
		return true
	case ast.String:
		if !stringCoercionEnabled.Load() {
			return false
		}
		_, ok := toDecimal(v)
		return ok
	default:
		return false
	}
}

// operandToDecimal converts an operator operand into a udecimal.Decimal.
// A parse error on an ast.Number (e.g. the out-of-precision "1e-25") is returned
// as-is so that it becomes an eval_builtin_error.
// An ast.String is converted only when stringCoercion is enabled.
func operandToDecimal(v ast.Value, pos int) (udecimal.Decimal, error) {
	switch val := v.(type) {
	case ast.Number:
		d, err := parseDecimal(string(val))
		if err != nil {
			return udecimal.Decimal{}, err
		}
		return d, nil
	case ast.String:
		if !stringCoercionEnabled.Load() {
			return udecimal.Decimal{}, builtins.NewOperandTypeErr(pos, v, "number")
		}
		d, err := parseDecimal(string(val))
		if err != nil {
			return udecimal.Decimal{}, builtins.NewOperandTypeErr(pos, v, "number")
		}
		return d, nil
	default:
		return udecimal.Decimal{}, builtins.NewOperandTypeErr(pos, v, "number")
	}
}

// elementToDecimal converts an array/set element into a udecimal.Decimal.
// A parse error on an ast.Number is returned as-is.
// An ast.String is converted only when stringCoercion is enabled.
func elementToDecimal(container ast.Value, elem *ast.Term) (udecimal.Decimal, error) {
	switch val := elem.Value.(type) {
	case ast.Number:
		d, err := parseDecimal(string(val))
		if err != nil {
			return udecimal.Decimal{}, err
		}
		return d, nil
	case ast.String:
		if !stringCoercionEnabled.Load() {
			return udecimal.Decimal{}, builtins.NewOperandElementErr(1, container, elem.Value, "number")
		}
		d, err := parseDecimal(string(val))
		if err != nil {
			return udecimal.Decimal{}, builtins.NewOperandElementErr(1, container, elem.Value, "number")
		}
		return d, nil
	default:
		return udecimal.Decimal{}, builtins.NewOperandElementErr(1, container, elem.Value, "number")
	}
}

// parseOperands parses two operands into udecimal.Decimal values.
// When stringCoercion is enabled, numeric-format strings are auto-converted.
func parseOperands(operands []*ast.Term) (udecimal.Decimal, udecimal.Decimal, error) {
	d1, err := operandToDecimal(operands[0].Value, 1)
	if err != nil {
		return udecimal.Decimal{}, udecimal.Decimal{}, err
	}
	d2, err := operandToDecimal(operands[1].Value, 2)
	if err != nil {
		return udecimal.Decimal{}, udecimal.Decimal{}, err
	}
	return d1, d2, nil
}

// numberResult converts a udecimal result into an ast.Term.
func numberResult(d udecimal.Decimal, iter func(*ast.Term) error) error {
	return iter(ast.NumberTerm(json.Number(d.String())))
}

// boolResult converts a bool result into an ast.Term.
// The two boolean terms are interned, so this allocates nothing — the same
// approach OPA's own comparison builtins take
// (OPA v1.11.0 topdown/comparison.go: iter(ast.InternedTerm(...))).
func boolResult(b bool, iter func(*ast.Term) error) error {
	return iter(ast.InternedTerm(b))
}

// === Arithmetic operations ===

func precisionPlus(_ topdown.BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	d1, d2, err := parseOperands(operands)
	if err != nil {
		return err
	}
	return numberResult(d1.Add(d2), iter)
}

func precisionMinus(_ topdown.BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	// minus is used for sets as well as numbers, so fall back to the original
	// behavior when the operands are not numeric.
	// When stringCoercion is enabled, numeric-format strings are treated as numbers.
	numLike1 := isNumericType(operands[0].Value)
	numLike2 := isNumericType(operands[1].Value)

	if numLike1 && numLike2 {
		// Parse via operandToDecimal to preserve ast.Number parse errors (e.g. the
		// out-of-precision "1e-25").
		d1, err := operandToDecimal(operands[0].Value, 1)
		if err != nil {
			return err
		}
		d2, err := operandToDecimal(operands[1].Value, 2)
		if err != nil {
			return err
		}
		return numberResult(d1.Sub(d2), iter)
	}

	// Original behavior for set operations.
	s1, ok3 := operands[0].Value.(ast.Set)
	s2, ok4 := operands[1].Value.(ast.Set)
	if ok3 && ok4 {
		return iter(ast.NewTerm(s1.Diff(s2)))
	}

	// Type mismatch error: report the expected type based on the lhs type.
	if numLike1 {
		// lhs is a number but rhs is not.
		return builtins.NewOperandTypeErr(2, operands[1].Value, "number")
	}
	if ok3 {
		// lhs is a set but rhs is not.
		return builtins.NewOperandTypeErr(2, operands[1].Value, "set")
	}
	return builtins.NewOperandTypeErr(1, operands[0].Value, "number", "set")
}

func precisionMultiply(_ topdown.BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	d1, d2, err := parseOperands(operands)
	if err != nil {
		return err
	}
	return numberResult(d1.Mul(d2), iter)
}

func precisionDivide(_ topdown.BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	d1, d2, err := parseOperands(operands)
	if err != nil {
		return err
	}
	result, err := d1.Div(d2)
	if err != nil {
		if errors.Is(err, udecimal.ErrDivideByZero) {
			// Return a plain error like standard OPA so it is handled with the
			// eval_builtin_error code
			// (OPA v1.11.0 topdown/arithmetic.go: errors.New("divide by zero")).
			return errors.New("divide by zero")
		}
		return err
	}
	return numberResult(result, iter)
}

// === Comparison operations ===

func precisionGT(_ topdown.BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	d1, d2, err := parseOperands(operands)
	if err != nil {
		return err
	}
	return boolResult(d1.Cmp(d2) > 0, iter)
}

func precisionGTE(_ topdown.BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	d1, d2, err := parseOperands(operands)
	if err != nil {
		return err
	}
	return boolResult(d1.Cmp(d2) >= 0, iter)
}

func precisionLT(_ topdown.BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	d1, d2, err := parseOperands(operands)
	if err != nil {
		return err
	}
	return boolResult(d1.Cmp(d2) < 0, iter)
}

func precisionLTE(_ topdown.BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	d1, d2, err := parseOperands(operands)
	if err != nil {
		return err
	}
	return boolResult(d1.Cmp(d2) <= 0, iter)
}

func precisionEqual(_ topdown.BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	// equal is used for many types besides numbers, so use precise comparison only when both are numbers.
	n1, ok1 := operands[0].Value.(ast.Number)
	n2, ok2 := operands[1].Value.(ast.Number)

	if ok1 && ok2 {
		d1, err := parseDecimal(string(n1))
		if err != nil {
			return err
		}
		d2, err := parseDecimal(string(n2))
		if err != nil {
			return err
		}
		return boolResult(d1.Cmp(d2) == 0, iter)
	}

	// Default equality comparison for non-numbers.
	return boolResult(operands[0].Value.Compare(operands[1].Value) == 0, iter)
}

func precisionNotEqual(_ topdown.BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	n1, ok1 := operands[0].Value.(ast.Number)
	n2, ok2 := operands[1].Value.(ast.Number)

	if ok1 && ok2 {
		d1, err := parseDecimal(string(n1))
		if err != nil {
			return err
		}
		d2, err := parseDecimal(string(n2))
		if err != nil {
			return err
		}
		return boolResult(d1.Cmp(d2) != 0, iter)
	}

	// Default comparison for non-numbers.
	return boolResult(operands[0].Value.Compare(operands[1].Value) != 0, iter)
}

// === Remainder operation ===

func precisionRem(_ topdown.BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	d1, d2, err := parseOperands(operands)
	if err != nil {
		return err
	}
	if d2.IsZero() {
		// Return a plain error like standard OPA so it is handled with the
		// eval_builtin_error code
		// (OPA v1.11.0 topdown/arithmetic.go: errors.New("modulo by zero")).
		return errors.New("modulo by zero")
	}
	result, err := d1.Mod(d2)
	if err != nil {
		return err
	}
	return numberResult(result, iter)
}

// === Unary operations ===

func parseUnaryOperand(operands []*ast.Term) (udecimal.Decimal, error) {
	return operandToDecimal(operands[0].Value, 1)
}

func precisionAbs(_ topdown.BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	d, err := parseUnaryOperand(operands)
	if err != nil {
		return err
	}
	return numberResult(d.Abs(), iter)
}

func precisionRound(_ topdown.BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	d, err := parseUnaryOperand(operands)
	if err != nil {
		return err
	}
	// Round half away from zero (same as OPA's default behavior).
	return numberResult(d.RoundHAZ(0), iter)
}

func precisionCeil(_ topdown.BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	d, err := parseUnaryOperand(operands)
	if err != nil {
		return err
	}
	return numberResult(d.Ceil(), iter)
}

func precisionFloor(_ topdown.BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	d, err := parseUnaryOperand(operands)
	if err != nil {
		return err
	}
	return numberResult(d.Floor(), iter)
}

// === Aggregate operations ===

func precisionSum(_ topdown.BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	sum := udecimal.Zero

	switch a := operands[0].Value.(type) {
	case *ast.Array:
		var err error
		a.Foreach(func(x *ast.Term) {
			if err != nil {
				return
			}
			d, parseErr := elementToDecimal(a, x)
			if parseErr != nil {
				err = parseErr
				return
			}
			sum = sum.Add(d)
		})
		if err != nil {
			return err
		}
	case ast.Set:
		var err error
		a.Foreach(func(x *ast.Term) {
			if err != nil {
				return
			}
			d, parseErr := elementToDecimal(a, x)
			if parseErr != nil {
				err = parseErr
				return
			}
			sum = sum.Add(d)
		})
		if err != nil {
			return err
		}
	default:
		return builtins.NewOperandTypeErr(1, operands[0].Value, "set", "array")
	}

	return numberResult(sum, iter)
}

func precisionProduct(_ topdown.BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	product := udecimal.One

	switch a := operands[0].Value.(type) {
	case *ast.Array:
		var err error
		a.Foreach(func(x *ast.Term) {
			if err != nil {
				return
			}
			d, parseErr := elementToDecimal(a, x)
			if parseErr != nil {
				err = parseErr
				return
			}
			product = product.Mul(d)
		})
		if err != nil {
			return err
		}
	case ast.Set:
		var err error
		a.Foreach(func(x *ast.Term) {
			if err != nil {
				return
			}
			d, parseErr := elementToDecimal(a, x)
			if parseErr != nil {
				err = parseErr
				return
			}
			product = product.Mul(d)
		})
		if err != nil {
			return err
		}
	default:
		return builtins.NewOperandTypeErr(1, operands[0].Value, "set", "array")
	}

	return numberResult(product, iter)
}

// extremaDir selects which end of the ordering max/min keep. Scaling a
// comparison result by the direction lets a single scan serve both operators.
type extremaDir int

const (
	extremaMax extremaDir = 1
	extremaMin extremaDir = -1
)

func precisionMax(_ topdown.BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	return precisionExtrema(operands[0].Value, extremaMax, iter)
}

func precisionMin(_ topdown.BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	return precisionExtrema(operands[0].Value, extremaMin, iter)
}

// precisionExtrema implements max/min over an array or set operand.
//
// Precise numeric comparison is used only when EVERY element is numeric-like:
// an ast.Number always qualifies (even one that fails to parse), an ast.String
// only when WithStringCoercion() is enabled and the string parses as a number.
// Any other element makes the whole collection fall back to the default
// ast.Compare ordering, matching standard OPA (e.g. max(["apple","banana"]) ==
// "banana", max([1,"2",true]) resolves via type ordering). The fallback policy
// is therefore identical whether or not coercion is enabled.
//
// Each operand type gets its own driver rather than a shared iteration
// interface: the per-element logic is shared through extremaScan, while
// iterating an array by index keeps the scan state on the stack, which a
// callback passed to an interface method would not.
func precisionExtrema(operand ast.Value, dir extremaDir, iter func(*ast.Term) error) error {
	switch c := operand.(type) {
	case *ast.Array:
		return arrayExtrema(c, dir, iter)
	case ast.Set:
		return setExtrema(c, dir, iter)
	default:
		return builtins.NewOperandTypeErr(1, operand, "set", "array")
	}
}

func arrayExtrema(a *ast.Array, dir extremaDir, iter func(*ast.Term) error) error {
	n := a.Len()
	if n == 0 {
		// max/min of an empty collection is undefined.
		return nil
	}

	scan := extremaScan{dir: dir}
	for i := 0; i < n; i++ {
		if scan.step(a.Elem(i)) {
			break
		}
	}
	if !scan.useCompare {
		return scan.result(iter)
	}

	var best ast.Value
	for i := 0; i < n; i++ {
		best = pickByCompare(best, a.Elem(i).Value, dir)
	}
	return iter(ast.NewTerm(best))
}

// setExtrema mirrors arrayExtrema. ast.Set exposes iteration only through
// callbacks, so unlike the array driver its scan state does escape to the heap.
func setExtrema(s ast.Set, dir extremaDir, iter func(*ast.Term) error) error {
	if s.Len() == 0 {
		return nil
	}

	scan := extremaScan{dir: dir}
	s.Until(scan.step)
	if !scan.useCompare {
		return scan.result(iter)
	}

	var best ast.Value
	s.Foreach(func(x *ast.Term) {
		best = pickByCompare(best, x.Value, dir)
	})
	return iter(ast.NewTerm(best))
}

// extremaScan folds a collection into a running max/min, parsing each element
// exactly once.
//
// The trade-off of parsing while classifying: a collection whose only
// non-numeric-like element sits at the end (e.g. max([1000 numbers…, "abc"]))
// parses every element before learning that the fallback ordering applies and
// that none of that work was needed. Deciding numeric-likeness first instead
// would skip those parses, but at the cost of parsing every string twice on
// the common numeric path.
//
// A parse failure on an ast.Number does not end the scan, because the remaining
// elements still decide whether the collection is numeric-only. The error is
// held until the scan completes: returned as-is if every element turned out to
// be numeric-like (so it surfaces as eval_builtin_error), dropped if the
// collection needs the fallback ordering instead. That is what separates
// max([1e-25, 2]), an error, from max([1e-25, {}]), which is not.
type extremaScan struct {
	dir        extremaDir
	bestTerm   *ast.Term
	bestVal    udecimal.Decimal
	pendingErr error
	useCompare bool
}

// step folds one element into the scan and reports whether the caller can stop
// iterating, which it can as soon as the collection is known not to be
// numeric-only.
func (s *extremaScan) step(x *ast.Term) bool {
	var d udecimal.Decimal

	switch v := x.Value.(type) {
	case ast.Number:
		if s.pendingErr != nil {
			// A Number is numeric-like whether or not it parses and only the
			// first parse failure is reported, so this element cannot change the
			// outcome any more.
			return false
		}
		parsed, err := parseDecimal(string(v))
		if err != nil {
			// Held rather than returned: a later non-numeric-like element would
			// switch the call to the fallback ordering, which never converts this
			// element and so never fails on it.
			s.pendingErr = err
			return false
		}
		d = parsed
	case ast.String:
		if !stringCoercionEnabled.Load() {
			s.useCompare = true
			return true
		}
		parsed, err := parseDecimal(string(v))
		if err != nil {
			// Not numeric-like: the collection is ordered by ast.Compare.
			s.useCompare = true
			return true
		}
		if s.pendingErr != nil {
			return false
		}
		d = parsed
	default:
		s.useCompare = true
		return true
	}

	// max keeps a candidate comparing greater than the best so far, min one
	// comparing smaller. Ties keep the element met first, which is what
	// preserves its exact term representation ("1.0" vs "1").
	if s.bestTerm == nil || d.Cmp(s.bestVal)*int(s.dir) > 0 {
		s.bestVal, s.bestTerm = d, x
	}
	return false
}

// result yields the numeric extremum the scan settled on. Only valid once the
// scan has run without requiring the fallback ordering.
func (s *extremaScan) result(iter func(*ast.Term) error) error {
	if s.pendingErr != nil {
		return s.pendingErr
	}
	return iter(s.bestTerm)
}

// pickByCompare folds one element into an extremum taken under OPA's default
// value ordering, the fallback for collections that are not numeric-only.
func pickByCompare(best, candidate ast.Value, dir extremaDir) ast.Value {
	// max replaces the best when it compares less than the candidate, min when
	// it compares greater. Ties keep the element met first.
	if best == nil || ast.Compare(best, candidate)*int(dir) < 0 {
		return candidate
	}
	return best
}
