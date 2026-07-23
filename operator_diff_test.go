package regobrick

import (
	"encoding/json"
	"fmt"
	"math/big"
	"math/rand"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/topdown"
)

// Differential test: the decimal operators are checked against exact big.Rat
// arithmetic on randomly generated (deterministic seed) operands.
//
// Reference semantics, matching observed udecimal behavior:
//   - +, -, *: exact (operands are generated with <=9 integer digits and <=9
//     fractional digits, so results stay within udecimal's 19-dp range)
//   - /: exact quotient truncated toward zero at 19 decimal places
//   - %: a - truncToInteger(a/b)*b (sign follows the dividend)
//   - comparisons: exact rational comparison

var pow19 = new(big.Int).Exp(big.NewInt(10), big.NewInt(19), nil)

func mustRat(t *testing.T, s string) *big.Rat {
	t.Helper()
	r, ok := new(big.Rat).SetString(s)
	if !ok {
		t.Fatalf("big.Rat failed to parse %q", s)
	}
	return r
}

// truncRat19 truncates r toward zero at 19 decimal places.
func truncRat19(r *big.Rat) *big.Rat {
	scaled := new(big.Rat).Mul(r, new(big.Rat).SetInt(pow19))
	i := new(big.Int).Quo(scaled.Num(), scaled.Denom()) // Quo truncates toward zero
	return new(big.Rat).SetFrac(i, pow19)
}

// truncRatToInt truncates r toward zero to an integer.
func truncRatToInt(r *big.Rat) *big.Rat {
	i := new(big.Int).Quo(r.Num(), r.Denom())
	return new(big.Rat).SetInt(i)
}

func callBinaryOp(t *testing.T, fn func(topdown.BuiltinContext, []*ast.Term, func(*ast.Term) error) error, lhs, rhs string) (ast.Value, error) {
	t.Helper()
	var got *ast.Term
	err := fn(
		topdown.BuiltinContext{},
		[]*ast.Term{ast.NumberTerm(json.Number(lhs)), ast.NumberTerm(json.Number(rhs))},
		func(term *ast.Term) error {
			got = term
			return nil
		},
	)
	if err != nil {
		return nil, err
	}
	if got == nil {
		return nil, fmt.Errorf("operator returned no result")
	}
	return got.Value, nil
}

func requireNumberEquals(t *testing.T, op, lhs, rhs string, gotVal ast.Value, want *big.Rat) {
	t.Helper()
	num, ok := gotVal.(ast.Number)
	if !ok {
		t.Fatalf("%s(%s, %s): expected ast.Number, got %T", op, lhs, rhs, gotVal)
	}
	got := mustRat(t, string(num))
	if got.Cmp(want) != 0 {
		t.Fatalf("%s(%s, %s) = %s, exact reference %s", op, lhs, rhs, string(num), want.RatString())
	}
}

func requireBoolEquals(t *testing.T, op, lhs, rhs string, gotVal ast.Value, want bool) {
	t.Helper()
	bv, ok := gotVal.(ast.Boolean)
	if !ok {
		t.Fatalf("%s(%s, %s): expected ast.Boolean, got %T", op, lhs, rhs, gotVal)
	}
	if bool(bv) != want {
		t.Fatalf("%s(%s, %s) = %v, exact reference %v", op, lhs, rhs, bool(bv), want)
	}
}

// randDecimalString generates a decimal string with up to 9 integer digits and
// up to 9 fractional digits, with a random sign.
func randDecimalString(rng *rand.Rand) string {
	var sb strings.Builder
	if rng.Intn(2) == 1 {
		sb.WriteByte('-')
	}
	fmt.Fprintf(&sb, "%d", rng.Int63n(1_000_000_000))
	if fracDigits := rng.Intn(10); fracDigits > 0 {
		fmt.Fprintf(&sb, ".%0*d", fracDigits, rng.Int63n(int64(pow10(fracDigits))))
	}
	return sb.String()
}

func pow10(n int) int {
	p := 1
	for i := 0; i < n; i++ {
		p *= 10
	}
	return p
}

func TestDecimalOperators_DifferentialVsBigRat(t *testing.T) {
	ensureDecimalArithmeticEnabled()
	rng := rand.New(rand.NewSource(20260723))

	const iterations = 3000
	for i := 0; i < iterations; i++ {
		lhs := randDecimalString(rng)
		rhs := randDecimalString(rng)
		ra := mustRat(t, lhs)
		rb := mustRat(t, rhs)

		// Arithmetic: exact within the generated operand bounds.
		got, err := callBinaryOp(t, precisionPlus, lhs, rhs)
		if err != nil {
			t.Fatalf("plus(%s, %s): %v", lhs, rhs, err)
		}
		requireNumberEquals(t, "plus", lhs, rhs, got, new(big.Rat).Add(ra, rb))

		got, err = callBinaryOp(t, precisionMinus, lhs, rhs)
		if err != nil {
			t.Fatalf("minus(%s, %s): %v", lhs, rhs, err)
		}
		requireNumberEquals(t, "minus", lhs, rhs, got, new(big.Rat).Sub(ra, rb))

		got, err = callBinaryOp(t, precisionMultiply, lhs, rhs)
		if err != nil {
			t.Fatalf("mul(%s, %s): %v", lhs, rhs, err)
		}
		requireNumberEquals(t, "mul", lhs, rhs, got, new(big.Rat).Mul(ra, rb))

		// Division / modulo: skip zero divisors (their error behavior is covered
		// by the explicit operator tests).
		if rb.Sign() != 0 {
			exactQuo := new(big.Rat).Quo(ra, rb)

			got, err = callBinaryOp(t, precisionDivide, lhs, rhs)
			if err != nil {
				t.Fatalf("div(%s, %s): %v", lhs, rhs, err)
			}
			requireNumberEquals(t, "div", lhs, rhs, got, truncRat19(exactQuo))

			got, err = callBinaryOp(t, precisionRem, lhs, rhs)
			if err != nil {
				t.Fatalf("rem(%s, %s): %v", lhs, rhs, err)
			}
			mod := new(big.Rat).Sub(ra, new(big.Rat).Mul(truncRatToInt(exactQuo), rb))
			requireNumberEquals(t, "rem", lhs, rhs, got, mod)
		}

		// Comparisons: exact rational ordering.
		cmp := ra.Cmp(rb)
		for _, tc := range []struct {
			name string
			fn   func(topdown.BuiltinContext, []*ast.Term, func(*ast.Term) error) error
			want bool
		}{
			{"gt", precisionGT, cmp > 0},
			{"gte", precisionGTE, cmp >= 0},
			{"lt", precisionLT, cmp < 0},
			{"lte", precisionLTE, cmp <= 0},
			{"eq", precisionEqual, cmp == 0},
			{"neq", precisionNotEqual, cmp != 0},
		} {
			got, err = callBinaryOp(t, tc.fn, lhs, rhs)
			if err != nil {
				t.Fatalf("%s(%s, %s): %v", tc.name, lhs, rhs, err)
			}
			requireBoolEquals(t, tc.name, lhs, rhs, got, tc.want)
		}
	}
}
