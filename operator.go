package regobrick

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"

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

var decimalConfig decimalArithmeticConfig

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
//   - Maximum 19 decimal places
//   - Range: ±34,028,236,692,093,846,346.3374607431768211455
//   - Input values with more than 19 decimal places fail to parse
//     (default mode: no result; StrictBuiltinErrors: eval error).
//     Truncation (not rounding) applies only to operation results, e.g. 100/3.
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
// Call this once at application startup, before any evaluation begins.
// It mutates process-global state (OPA's builtin registry and the coercion
// setting) without synchronization, so it is not safe to call concurrently
// with evaluations.
func UseDecimalArithmetic(opts ...DecimalArithmeticOption) {
	cfg := decimalArithmeticConfig{}
	for _, opt := range opts {
		opt(&cfg)
	}
	decimalConfig = cfg
	registerDecimalBuiltins()
}

func registerDecimalBuiltins() {
	// Arithmetic operators
	topdown.RegisterBuiltinFunc(ast.Plus.Name, precisionPlus)
	topdown.RegisterBuiltinFunc(ast.Minus.Name, precisionMinus)
	topdown.RegisterBuiltinFunc(ast.Multiply.Name, precisionMultiply)
	topdown.RegisterBuiltinFunc(ast.Divide.Name, precisionDivide)
	topdown.RegisterBuiltinFunc(ast.Rem.Name, precisionRem)

	// Comparison operators
	topdown.RegisterBuiltinFunc(ast.GreaterThan.Name, precisionGT)
	topdown.RegisterBuiltinFunc(ast.GreaterThanEq.Name, precisionGTE)
	topdown.RegisterBuiltinFunc(ast.LessThan.Name, precisionLT)
	topdown.RegisterBuiltinFunc(ast.LessThanEq.Name, precisionLTE)
	topdown.RegisterBuiltinFunc(ast.Equal.Name, precisionEqual)
	topdown.RegisterBuiltinFunc(ast.NotEqual.Name, precisionNotEqual)

	// Unary operators
	topdown.RegisterBuiltinFunc(ast.Abs.Name, precisionAbs)
	topdown.RegisterBuiltinFunc(ast.Round.Name, precisionRound)
	topdown.RegisterBuiltinFunc(ast.Ceil.Name, precisionCeil)
	topdown.RegisterBuiltinFunc(ast.Floor.Name, precisionFloor)

	// Aggregate operators
	topdown.RegisterBuiltinFunc(ast.Sum.Name, precisionSum)
	topdown.RegisterBuiltinFunc(ast.Product.Name, precisionProduct)
	topdown.RegisterBuiltinFunc(ast.Max.Name, precisionMax)
	topdown.RegisterBuiltinFunc(ast.Min.Name, precisionMin)
}

// maxExpandedLen is the upper bound on the string length of an expanded exponent
// notation result.
//
// The values udecimal can represent have at most ~20 integer digits + 19
// fractional digits (range ±34,028,236,692,093,846,346.3374607431768211455), so
// even including a sign and decimal point, any valid expansion is well under
// this bound. An expansion certain to exceed the bound would be rejected by
// udecimal anyway, so instead of allocating a huge zero string via
// strings.Repeat (e.g. 1e2000000000 from an input path would trigger ~2GB of
// allocation) the original string is returned unchanged. udecimal.Parse then
// rejects the exponent notation as an invalid format, ending in the same
// precision-limit error behavior as before.
const maxExpandedLen = 64

// expandExponent expands a scientific-notation number string into plain decimal
// notation. E.g. "1e-8"→"0.00000001", "-2.5E+3"→"-2500", "1.5e0"→"1.5".
//
// It shifts only the decimal point via pure string manipulation without going
// through float64, so there is no precision loss. Non-exponent (e.g. "123.45")
// or malformed strings are returned unchanged for udecimal.Parse to decide on.
//
// When the expanded result exceeds udecimal's range/precision (19 fractional
// digits) (e.g. "1e-25"→"0.0000...1", "1e30"), udecimal.Parse returns an error;
// this is the documented precision limit. A huge exponent whose expansion is
// certain to exceed maxExpandedLen (e.g. "1e2000000000") is not expanded and the
// original string is returned to avoid an allocation blowup (udecimal rejects it
// → same error behavior).
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

	// Separate the integer and fractional parts.
	intPart := mantissa
	fracPart := ""
	if dot := strings.IndexByte(mantissa, '.'); dot >= 0 {
		intPart = mantissa[:dot]
		fracPart = mantissa[dot+1:]
	}

	digits := intPart + fracPart
	if digits == "" {
		return s
	}
	for i := 0; i < len(digits); i++ {
		if digits[i] < '0' || digits[i] > '9' {
			return s // non-digit character in the mantissa → let udecimal reject it
		}
	}

	// value = digits × 10^(exp - len(fracPart))
	newExp := exp - len(fracPart)

	// Size guard 2: precompute the expansion length and do not expand if it
	// exceeds the bound.
	// (newExp >= 0: len(digits)+newExp, newExp < 0: at most len(digits)+|newExp|+2)
	absNewExp := newExp
	if absNewExp < 0 {
		absNewExp = -absNewExp
	}
	if len(digits)+absNewExp+2 > maxExpandedLen {
		return s
	}

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
		if !decimalConfig.stringCoercion {
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
		if !decimalConfig.stringCoercion {
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
		if !decimalConfig.stringCoercion {
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
func boolResult(b bool, iter func(*ast.Term) error) error {
	return iter(ast.BooleanTerm(b))
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

// shouldUseNumericExtrema reports whether max/min should use precise numeric
// comparison. Numeric mode is used only when every element is numeric-like;
// otherwise the callers fall back to the default ast.Compare ordering, matching
// standard OPA (e.g. max(["apple","banana"]) == "banana", max([1,"2",true])
// resolves via type ordering).
//
// "numeric-like" is decided by isNumericType: ast.Number always qualifies, and
// ast.String qualifies only when WithStringCoercion() is enabled and the string
// parses as a number. This makes the fallback policy identical whether or not
// coercion is enabled.
func shouldUseNumericExtrema(foreach func(func(*ast.Term))) bool {
	allNumericLike := true
	foreach(func(x *ast.Term) {
		if !isNumericType(x.Value) {
			allNumericLike = false
		}
	})
	return allNumericLike
}

func precisionMax(_ topdown.BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	switch a := operands[0].Value.(type) {
	case *ast.Array:
		if a.Len() == 0 {
			return nil
		}
		useNumeric := shouldUseNumericExtrema(func(fn func(*ast.Term)) {
			a.Foreach(fn)
		})
		if !useNumeric {
			// Use the default comparison when a non-numeric element is present.
			max := a.Elem(0).Value
			a.Foreach(func(x *ast.Term) {
				if ast.Compare(max, x.Value) < 0 {
					max = x.Value
				}
			})
			return iter(ast.NewTerm(max))
		}
		// Precise comparison when all elements are numeric.
		var maxVal udecimal.Decimal
		var maxTerm *ast.Term
		var numericErr error
		first := true
		a.Foreach(func(x *ast.Term) {
			if numericErr != nil {
				return
			}
			d, parseErr := elementToDecimal(a, x)
			if parseErr != nil {
				numericErr = parseErr
				return
			}
			if first || d.Cmp(maxVal) > 0 {
				maxVal = d
				maxTerm = x
				first = false
			}
		})
		if numericErr != nil {
			return numericErr
		}
		return iter(maxTerm)
	case ast.Set:
		if a.Len() == 0 {
			return nil
		}
		useNumeric := shouldUseNumericExtrema(func(fn func(*ast.Term)) {
			a.Foreach(fn)
		})
		if !useNumeric {
			// Use the default comparison when a non-numeric element is present.
			var max ast.Value
			a.Foreach(func(x *ast.Term) {
				if max == nil || ast.Compare(max, x.Value) < 0 {
					max = x.Value
				}
			})
			return iter(ast.NewTerm(max))
		}
		// Precise comparison when all elements are numeric.
		var maxVal udecimal.Decimal
		var maxTerm *ast.Term
		var numericErr error
		first := true
		a.Foreach(func(x *ast.Term) {
			if numericErr != nil {
				return
			}
			d, parseErr := elementToDecimal(a, x)
			if parseErr != nil {
				numericErr = parseErr
				return
			}
			if first || d.Cmp(maxVal) > 0 {
				maxVal = d
				maxTerm = x
				first = false
			}
		})
		if numericErr != nil {
			return numericErr
		}
		return iter(maxTerm)
	default:
		return builtins.NewOperandTypeErr(1, operands[0].Value, "set", "array")
	}
}

func precisionMin(_ topdown.BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	switch a := operands[0].Value.(type) {
	case *ast.Array:
		if a.Len() == 0 {
			return nil
		}
		useNumeric := shouldUseNumericExtrema(func(fn func(*ast.Term)) {
			a.Foreach(fn)
		})
		if !useNumeric {
			// Use the default comparison when a non-numeric element is present.
			min := a.Elem(0).Value
			a.Foreach(func(x *ast.Term) {
				if ast.Compare(min, x.Value) > 0 {
					min = x.Value
				}
			})
			return iter(ast.NewTerm(min))
		}
		// Precise comparison when all elements are numeric.
		var minVal udecimal.Decimal
		var minTerm *ast.Term
		var numericErr error
		first := true
		a.Foreach(func(x *ast.Term) {
			if numericErr != nil {
				return
			}
			d, parseErr := elementToDecimal(a, x)
			if parseErr != nil {
				numericErr = parseErr
				return
			}
			if first || d.Cmp(minVal) < 0 {
				minVal = d
				minTerm = x
				first = false
			}
		})
		if numericErr != nil {
			return numericErr
		}
		return iter(minTerm)
	case ast.Set:
		if a.Len() == 0 {
			return nil
		}
		useNumeric := shouldUseNumericExtrema(func(fn func(*ast.Term)) {
			a.Foreach(fn)
		})
		if !useNumeric {
			// Use the default comparison when a non-numeric element is present.
			var min ast.Value
			a.Foreach(func(x *ast.Term) {
				if min == nil || ast.Compare(min, x.Value) > 0 {
					min = x.Value
				}
			})
			return iter(ast.NewTerm(min))
		}
		// Precise comparison when all elements are numeric.
		var minVal udecimal.Decimal
		var minTerm *ast.Term
		var numericErr error
		first := true
		a.Foreach(func(x *ast.Term) {
			if numericErr != nil {
				return
			}
			d, parseErr := elementToDecimal(a, x)
			if parseErr != nil {
				numericErr = parseErr
				return
			}
			if first || d.Cmp(minVal) < 0 {
				minVal = d
				minTerm = x
				first = false
			}
		})
		if numericErr != nil {
			return numericErr
		}
		return iter(minTerm)
	default:
		return builtins.NewOperandTypeErr(1, operands[0].Value, "set", "array")
	}
}
