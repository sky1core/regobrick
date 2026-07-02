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
	// 산술 연산자
	topdown.RegisterBuiltinFunc(ast.Plus.Name, precisionPlus)
	topdown.RegisterBuiltinFunc(ast.Minus.Name, precisionMinus)
	topdown.RegisterBuiltinFunc(ast.Multiply.Name, precisionMultiply)
	topdown.RegisterBuiltinFunc(ast.Divide.Name, precisionDivide)
	topdown.RegisterBuiltinFunc(ast.Rem.Name, precisionRem)

	// 비교 연산자
	topdown.RegisterBuiltinFunc(ast.GreaterThan.Name, precisionGT)
	topdown.RegisterBuiltinFunc(ast.GreaterThanEq.Name, precisionGTE)
	topdown.RegisterBuiltinFunc(ast.LessThan.Name, precisionLT)
	topdown.RegisterBuiltinFunc(ast.LessThanEq.Name, precisionLTE)
	topdown.RegisterBuiltinFunc(ast.Equal.Name, precisionEqual)
	topdown.RegisterBuiltinFunc(ast.NotEqual.Name, precisionNotEqual)

	// 단항 연산자
	topdown.RegisterBuiltinFunc(ast.Abs.Name, precisionAbs)
	topdown.RegisterBuiltinFunc(ast.Round.Name, precisionRound)
	topdown.RegisterBuiltinFunc(ast.Ceil.Name, precisionCeil)
	topdown.RegisterBuiltinFunc(ast.Floor.Name, precisionFloor)

	// 집계 연산자
	topdown.RegisterBuiltinFunc(ast.Sum.Name, precisionSum)
	topdown.RegisterBuiltinFunc(ast.Product.Name, precisionProduct)
	topdown.RegisterBuiltinFunc(ast.Max.Name, precisionMax)
	topdown.RegisterBuiltinFunc(ast.Min.Name, precisionMin)
}

// maxExpandedLen은 지수 표기 전개 결과 문자열 길이의 상한입니다.
//
// udecimal이 표현 가능한 값은 정수부 최대 20자리 + 소수부 최대 19자리 수준이므로
// (범위 ±34,028,236,692,093,846,346.3374607431768211455), 부호/소수점을 포함해도
// 유효한 전개 결과는 이 상한을 크게 밑돕니다. 상한을 넘을 것이 확실한 전개는
// 어차피 udecimal이 거부할 값이므로, strings.Repeat로 거대한 0 문자열을
// 할당하지 않고(예: input 경로의 1e2000000000은 약 2GB 할당 유발) 원본을
// 그대로 반환합니다. 그러면 udecimal.Parse가 지수 표기를 invalid format으로
// 거부하여 기존 정밀도-한계 에러 동작과 동일하게 끝납니다.
const maxExpandedLen = 64

// expandExponent는 지수 표기(scientific notation) 숫자 문자열을 평범한 십진
// 표기로 전개합니다. 예: "1e-8"→"0.00000001", "-2.5E+3"→"-2500", "1.5e0"→"1.5".
//
// float64를 경유하지 않고 순수 문자열 조작으로 소수점 위치만 이동시키므로
// 정밀도 손실이 없습니다. 지수 표기가 아니거나(예: "123.45") 형식이 잘못된
// 문자열은 그대로 반환하여 udecimal.Parse가 판단하도록 넘깁니다.
//
// 전개 결과가 udecimal 범위/정밀도(소수점 이하 19자리)를 벗어나는 경우
// (예: "1e-25"→"0.0000...1", "1e30") udecimal.Parse가 에러를 반환하며,
// 이는 문서화된 정밀도 한계입니다. 전개 결과 길이가 maxExpandedLen을 넘을
// 것이 확실한 거대 지수(예: "1e2000000000")는 할당 폭발을 피하기 위해
// 전개하지 않고 원본을 그대로 반환합니다 (udecimal이 거부 → 동일한 에러 동작).
func expandExponent(s string) string {
	ePos := strings.IndexAny(s, "eE")
	if ePos < 0 {
		return s
	}

	mantissa := s[:ePos]
	exp, err := strconv.Atoi(s[ePos+1:])
	if err != nil {
		return s // 지수부가 정수가 아님(int 범위 초과 포함) → udecimal이 거부하도록 위임
	}

	// 크기 가드 1: 지수 절대값이 상한을 넘으면 전개 결과도 반드시 상한을 넘으므로
	// 전개하지 않는다. (이후 newExp 계산의 정수 오버플로도 함께 차단)
	if exp > maxExpandedLen || exp < -maxExpandedLen {
		return s
	}

	// 가수부 부호 분리
	sign := ""
	if len(mantissa) > 0 && (mantissa[0] == '+' || mantissa[0] == '-') {
		if mantissa[0] == '-' {
			sign = "-"
		}
		mantissa = mantissa[1:]
	}

	// 정수부/소수부 분리
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
			return s // 가수부에 숫자 외 문자 → udecimal이 거부하도록 위임
		}
	}

	// 값 = digits × 10^(exp - len(fracPart))
	newExp := exp - len(fracPart)

	// 크기 가드 2: 전개 결과 길이를 사전 계산해 상한을 넘으면 전개하지 않는다.
	// (newExp >= 0: len(digits)+newExp, newExp < 0: 최대 len(digits)+|newExp|+2)
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

// parseDecimal은 숫자 문자열을 udecimal.Decimal로 파싱합니다.
// 지수 표기를 먼저 평범한 십진 표기로 전개한 뒤 udecimal.Parse에 넘기므로,
// "1e-8"과 같은 지수 표기도 정확히 파싱됩니다.
func parseDecimal(s string) (udecimal.Decimal, error) {
	return udecimal.Parse(expandExponent(s))
}

// toDecimal은 ast.Value를 udecimal.Decimal로 변환합니다.
// ast.Number와 ast.String 모두 파싱을 시도하며, 실패 시 false를 반환합니다.
// 저수준 헬퍼로, stringCoercion 조건은 호출자(isNumericType 등)가 판단합니다.
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

// isNumericType은 값이 숫자 모드에서 처리되어야 하는지 판별합니다.
// ast.Number는 항상 true (파싱 실패해도 숫자 타입).
// ast.String은 stringCoercion이 활성화되고 숫자로 파싱 가능한 경우에만 true.
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

// operandToDecimal은 연산자 피연산자를 udecimal.Decimal로 변환합니다.
// ast.Number의 파싱 에러(예: 정밀도를 벗어나는 "1e-25")는 원래 에러를 그대로
// 반환하여 eval_builtin_error가 되도록 합니다.
// ast.String은 stringCoercion이 활성화된 경우에만 변환을 시도합니다.
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

// elementToDecimal은 배열/셋 요소를 udecimal.Decimal로 변환합니다.
// ast.Number의 파싱 에러는 원래 에러를 그대로 반환합니다.
// ast.String은 stringCoercion이 활성화된 경우에만 변환을 시도합니다.
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

// parseOperands는 두 피연산자를 udecimal.Decimal로 파싱합니다.
// stringCoercion 활성화 시, 숫자 형식 문자열도 자동 변환됩니다.
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

// numberResult는 udecimal 결과를 ast.Term으로 변환
func numberResult(d udecimal.Decimal, iter func(*ast.Term) error) error {
	return iter(ast.NumberTerm(json.Number(d.String())))
}

// boolResult는 bool 결과를 ast.Term으로 변환
func boolResult(b bool, iter func(*ast.Term) error) error {
	return iter(ast.BooleanTerm(b))
}

// === 산술 연산 ===

func precisionPlus(_ topdown.BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	d1, d2, err := parseOperands(operands)
	if err != nil {
		return err
	}
	return numberResult(d1.Add(d2), iter)
}

func precisionMinus(_ topdown.BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	// minus는 숫자 뿐만 아니라 set에도 사용되므로, 숫자가 아닌 경우 원래 동작으로 폴백
	// stringCoercion 활성화 시, 숫자 형식 문자열도 숫자로 취급
	numLike1 := isNumericType(operands[0].Value)
	numLike2 := isNumericType(operands[1].Value)

	if numLike1 && numLike2 {
		// operandToDecimal로 파싱하여 ast.Number의 파싱 에러(예: 정밀도 초과 "1e-25") 보존
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

	// set 연산의 경우 원래 동작
	s1, ok3 := operands[0].Value.(ast.Set)
	s2, ok4 := operands[1].Value.(ast.Set)
	if ok3 && ok4 {
		return iter(ast.NewTerm(s1.Diff(s2)))
	}

	// 타입 불일치 에러: lhs 타입에 맞는 기대 타입을 표시
	if numLike1 {
		// lhs가 숫자인데 rhs가 숫자가 아님
		return builtins.NewOperandTypeErr(2, operands[1].Value, "number")
	}
	if ok3 {
		// lhs가 set인데 rhs가 set이 아님
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
			// 표준 OPA와 동일하게 plain error를 반환하여 eval_builtin_error 코드로 처리
			// (OPA v1.11.0 topdown/arithmetic.go: errors.New("divide by zero")).
			return errors.New("divide by zero")
		}
		return err
	}
	return numberResult(result, iter)
}

// === 비교 연산 ===

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
	// equal은 숫자 외에도 다양한 타입에 사용되므로, 숫자인 경우만 정밀 비교
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

	// 숫자가 아닌 경우 기본 동등 비교
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

	// 숫자가 아닌 경우 기본 비교
	return boolResult(operands[0].Value.Compare(operands[1].Value) != 0, iter)
}

// === 나머지 연산 ===

func precisionRem(_ topdown.BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	d1, d2, err := parseOperands(operands)
	if err != nil {
		return err
	}
	if d2.IsZero() {
		// 표준 OPA와 동일하게 plain error를 반환하여 eval_builtin_error 코드로 처리
		// (OPA v1.11.0 topdown/arithmetic.go: errors.New("modulo by zero")).
		return errors.New("modulo by zero")
	}
	result, err := d1.Mod(d2)
	if err != nil {
		return err
	}
	return numberResult(result, iter)
}

// === 단항 연산 ===

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
	// Round half away from zero (OPA 기본 동작과 동일)
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

// === 집계 연산 ===

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
			// 숫자가 아닌 요소가 있으면 기본 비교 사용
			max := a.Elem(0).Value
			a.Foreach(func(x *ast.Term) {
				if ast.Compare(max, x.Value) < 0 {
					max = x.Value
				}
			})
			return iter(ast.NewTerm(max))
		}
		// 모두 숫자인 경우 정밀 비교
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
			// 숫자가 아닌 요소가 있으면 기본 비교 사용
			var max ast.Value
			a.Foreach(func(x *ast.Term) {
				if max == nil || ast.Compare(max, x.Value) < 0 {
					max = x.Value
				}
			})
			return iter(ast.NewTerm(max))
		}
		// 모두 숫자인 경우 정밀 비교
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
			// 숫자가 아닌 요소가 있으면 기본 비교 사용
			min := a.Elem(0).Value
			a.Foreach(func(x *ast.Term) {
				if ast.Compare(min, x.Value) > 0 {
					min = x.Value
				}
			})
			return iter(ast.NewTerm(min))
		}
		// 모두 숫자인 경우 정밀 비교
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
			// 숫자가 아닌 요소가 있으면 기본 비교 사용
			var min ast.Value
			a.Foreach(func(x *ast.Term) {
				if min == nil || ast.Compare(min, x.Value) > 0 {
					min = x.Value
				}
			})
			return iter(ast.NewTerm(min))
		}
		// 모두 숫자인 경우 정밀 비교
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
