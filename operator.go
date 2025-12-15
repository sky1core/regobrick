package regobrick

import (
	"encoding/json"
	"errors"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/topdown"
	"github.com/open-policy-agent/opa/v1/topdown/builtins"
	"github.com/quagmt/udecimal"
)

// UseDecimalArithmetic Rego의 숫자 연산을 정밀 decimal 연산으로 대체합니다.
//
// 오버로딩되는 연산자:
//   - 산술: +, -, *, /, %
//   - 비교: >, >=, <, <=, ==, !=
//   - 단항: abs(), round(), ceil(), floor()
//   - 집계: sum(), product(), max(), min()
//
// 정밀도 제한 (udecimal):
//   - 소수점 이하 최대 19자리
//   - 범위: ±34,028,236,692,093,846,346.3374607431768211455
//   - 19자리 초과 시 truncate (반올림 아님)
//
// 에러 처리:
//   - 기본 모드: 연산 실패 시 규칙 미충족 (결과 없음)
//   - StrictBuiltinErrors(true): eval_builtin_error 반환
//
// 이 함수를 호출하면 Rego에서 다음과 같이 자연스럽게 사용 가능:
//
//	reduce_price := entry_price * (1 + reduce_rate)
//	can_reduce := reduce_amt >= min_amount
//	rounded := round(price)
//	total := sum([price1, price2, price3])
func UseDecimalArithmetic() {
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

// parseOperands는 두 피연산자를 udecimal.Decimal로 파싱
func parseOperands(operands []*ast.Term) (udecimal.Decimal, udecimal.Decimal, error) {
	n1, err := builtins.NumberOperand(operands[0].Value, 1)
	if err != nil {
		return udecimal.Decimal{}, udecimal.Decimal{}, err
	}
	n2, err := builtins.NumberOperand(operands[1].Value, 2)
	if err != nil {
		return udecimal.Decimal{}, udecimal.Decimal{}, err
	}

	d1, err := udecimal.Parse(string(n1))
	if err != nil {
		return udecimal.Decimal{}, udecimal.Decimal{}, err
	}
	d2, err := udecimal.Parse(string(n2))
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
	n1, ok1 := operands[0].Value.(ast.Number)
	n2, ok2 := operands[1].Value.(ast.Number)

	if ok1 && ok2 {
		d1, err := udecimal.Parse(string(n1))
		if err != nil {
			return err
		}
		d2, err := udecimal.Parse(string(n2))
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

	if !ok1 && !ok3 {
		return builtins.NewOperandTypeErr(1, operands[0].Value, "number", "set")
	}
	return builtins.NewOperandTypeErr(2, operands[1].Value, "number", "set")
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
			return builtins.NewOperandErr(2, "divide by zero")
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
		d1, err := udecimal.Parse(string(n1))
		if err != nil {
			return err
		}
		d2, err := udecimal.Parse(string(n2))
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
		d1, err := udecimal.Parse(string(n1))
		if err != nil {
			return err
		}
		d2, err := udecimal.Parse(string(n2))
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
		return builtins.NewOperandErr(2, "modulo by zero")
	}
	result, err := d1.Mod(d2)
	if err != nil {
		return err
	}
	return numberResult(result, iter)
}

// === 단항 연산 ===

func parseUnaryOperand(operands []*ast.Term) (udecimal.Decimal, error) {
	n, err := builtins.NumberOperand(operands[0].Value, 1)
	if err != nil {
		return udecimal.Decimal{}, err
	}
	return udecimal.Parse(string(n))
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
			n, ok := x.Value.(ast.Number)
			if !ok {
				err = builtins.NewOperandElementErr(1, a, x.Value, "number")
				return
			}
			d, parseErr := udecimal.Parse(string(n))
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
			n, ok := x.Value.(ast.Number)
			if !ok {
				err = builtins.NewOperandElementErr(1, a, x.Value, "number")
				return
			}
			d, parseErr := udecimal.Parse(string(n))
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
			n, ok := x.Value.(ast.Number)
			if !ok {
				err = builtins.NewOperandElementErr(1, a, x.Value, "number")
				return
			}
			d, parseErr := udecimal.Parse(string(n))
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
			n, ok := x.Value.(ast.Number)
			if !ok {
				err = builtins.NewOperandElementErr(1, a, x.Value, "number")
				return
			}
			d, parseErr := udecimal.Parse(string(n))
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

func precisionMax(_ topdown.BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	switch a := operands[0].Value.(type) {
	case *ast.Array:
		if a.Len() == 0 {
			return nil
		}
		// 숫자 배열인지 확인
		allNumbers := true
		a.Until(func(x *ast.Term) bool {
			if _, ok := x.Value.(ast.Number); !ok {
				allNumbers = false
				return true
			}
			return false
		})
		if !allNumbers {
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
		var err error
		first := true
		a.Foreach(func(x *ast.Term) {
			if err != nil {
				return
			}
			d, parseErr := udecimal.Parse(string(x.Value.(ast.Number)))
			if parseErr != nil {
				err = parseErr
				return
			}
			if first || d.Cmp(maxVal) > 0 {
				maxVal = d
				maxTerm = x
				first = false
			}
		})
		if err != nil {
			return err
		}
		return iter(maxTerm)
	case ast.Set:
		if a.Len() == 0 {
			return nil
		}
		// 숫자 셋인지 확인
		allNumbers := true
		a.Until(func(x *ast.Term) bool {
			if _, ok := x.Value.(ast.Number); !ok {
				allNumbers = false
				return true
			}
			return false
		})
		if !allNumbers {
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
		var err error
		first := true
		a.Foreach(func(x *ast.Term) {
			if err != nil {
				return
			}
			d, parseErr := udecimal.Parse(string(x.Value.(ast.Number)))
			if parseErr != nil {
				err = parseErr
				return
			}
			if first || d.Cmp(maxVal) > 0 {
				maxVal = d
				maxTerm = x
				first = false
			}
		})
		if err != nil {
			return err
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
		// 숫자 배열인지 확인
		allNumbers := true
		a.Until(func(x *ast.Term) bool {
			if _, ok := x.Value.(ast.Number); !ok {
				allNumbers = false
				return true
			}
			return false
		})
		if !allNumbers {
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
		var err error
		first := true
		a.Foreach(func(x *ast.Term) {
			if err != nil {
				return
			}
			d, parseErr := udecimal.Parse(string(x.Value.(ast.Number)))
			if parseErr != nil {
				err = parseErr
				return
			}
			if first || d.Cmp(minVal) < 0 {
				minVal = d
				minTerm = x
				first = false
			}
		})
		if err != nil {
			return err
		}
		return iter(minTerm)
	case ast.Set:
		if a.Len() == 0 {
			return nil
		}
		// 숫자 셋인지 확인
		allNumbers := true
		a.Until(func(x *ast.Term) bool {
			if _, ok := x.Value.(ast.Number); !ok {
				allNumbers = false
				return true
			}
			return false
		})
		if !allNumbers {
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
		var err error
		first := true
		a.Foreach(func(x *ast.Term) {
			if err != nil {
				return
			}
			d, parseErr := udecimal.Parse(string(x.Value.(ast.Number)))
			if parseErr != nil {
				err = parseErr
				return
			}
			if first || d.Cmp(minVal) < 0 {
				minVal = d
				minTerm = x
				first = false
			}
		})
		if err != nil {
			return err
		}
		return iter(minTerm)
	default:
		return builtins.NewOperandTypeErr(1, operands[0].Value, "set", "array")
	}
}
