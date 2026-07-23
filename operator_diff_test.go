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

// randHighFracDecimalString generates a decimal string with up to 2 integer
// digits and 10..19 fractional digits, so that products routinely exceed 19
// decimal places and exercise udecimal's truncate-toward-zero path.
func randHighFracDecimalString(rng *rand.Rand) string {
	var sb strings.Builder
	if rng.Intn(2) == 1 {
		sb.WriteByte('-')
	}
	fmt.Fprintf(&sb, "%d.", rng.Int63n(100))
	fracDigits := 10 + rng.Intn(10)
	for j := 0; j < fracDigits; j++ {
		fmt.Fprintf(&sb, "%d", rng.Int63n(10))
	}
	return sb.String()
}

// TestDecimalMultiply_TruncationDifferentialVsBigRat covers the range the main
// differential test deliberately avoids: operands whose exact product exceeds
// 19 decimal places. Reference: exact product truncated toward zero at 19 dp
// (udecimal semantics, verified by direct probing, including negatives and
// underflow-to-zero like 1e-10 * 1e-11).
func TestDecimalMultiply_TruncationDifferentialVsBigRat(t *testing.T) {
	ensureDecimalArithmeticEnabled()
	rng := rand.New(rand.NewSource(20260724))

	const iterations = 3000
	for i := 0; i < iterations; i++ {
		lhs := randHighFracDecimalString(rng)
		rhs := randHighFracDecimalString(rng)
		ra := mustRat(t, lhs)
		rb := mustRat(t, rhs)

		got, err := callBinaryOp(t, precisionMultiply, lhs, rhs)
		if err != nil {
			t.Fatalf("mul(%s, %s): %v", lhs, rhs, err)
		}
		requireNumberEquals(t, "mul", lhs, rhs, got, truncRat19(new(big.Rat).Mul(ra, rb)))
	}
}

func callAggregate(t *testing.T, fn func(topdown.BuiltinContext, []*ast.Term, func(*ast.Term) error) error, elems []string) (ast.Value, error) {
	t.Helper()
	arrTerms := make([]*ast.Term, len(elems))
	for i, e := range elems {
		arrTerms[i] = ast.NumberTerm(json.Number(e))
	}
	var got *ast.Term
	err := fn(
		topdown.BuiltinContext{},
		[]*ast.Term{ast.ArrayTerm(arrTerms...)},
		func(term *ast.Term) error {
			got = term
			return nil
		},
	)
	if err != nil {
		return nil, err
	}
	if got == nil {
		return nil, fmt.Errorf("aggregate returned no result")
	}
	return got.Value, nil
}

func requireAggEquals(t *testing.T, op string, elems []string, gotVal ast.Value, want *big.Rat) {
	t.Helper()
	num, ok := gotVal.(ast.Number)
	if !ok {
		t.Fatalf("%s(%v): expected ast.Number, got %T", op, elems, gotVal)
	}
	got := mustRat(t, string(num))
	if got.Cmp(want) != 0 {
		t.Fatalf("%s(%v) = %s, exact reference %s", op, elems, string(num), want.RatString())
	}
}

// TestDecimalAggregates_DifferentialVsBigRat checks sum/product/max/min over
// random arrays against exact big.Rat arithmetic. Operands are kept in a range
// where every aggregate is exact in udecimal (no truncation), so the reference
// is a plain rational computation rather than a shadow reimplementation:
//   - sum: elements <=9 int / <=9 frac digits, up to 50 of them
//   - product: up to 5 factors, <=2 int / <=3 frac digits (<=15 dp total)
//   - max/min: exact ordering; result compared numerically
func TestDecimalAggregates_DifferentialVsBigRat(t *testing.T) {
	ensureDecimalArithmeticEnabled()
	rng := rand.New(rand.NewSource(20260725))

	const iterations = 600
	for i := 0; i < iterations; i++ {
		// --- sum / max / min over the same array ---
		n := 1 + rng.Intn(50)
		elems := make([]string, n)
		rats := make([]*big.Rat, n)
		for j := range elems {
			elems[j] = randDecimalString(rng)
			rats[j] = mustRat(t, elems[j])
		}

		sum := new(big.Rat)
		maxRat := rats[0]
		minRat := rats[0]
		for _, r := range rats {
			sum.Add(sum, r)
			if r.Cmp(maxRat) > 0 {
				maxRat = r
			}
			if r.Cmp(minRat) < 0 {
				minRat = r
			}
		}

		got, err := callAggregate(t, precisionSum, elems)
		if err != nil {
			t.Fatalf("sum(%v): %v", elems, err)
		}
		requireAggEquals(t, "sum", elems, got, sum)

		got, err = callAggregate(t, precisionMax, elems)
		if err != nil {
			t.Fatalf("max(%v): %v", elems, err)
		}
		requireAggEquals(t, "max", elems, got, maxRat)

		got, err = callAggregate(t, precisionMin, elems)
		if err != nil {
			t.Fatalf("min(%v): %v", elems, err)
		}
		requireAggEquals(t, "min", elems, got, minRat)

		// --- product over a smaller, exact-range array ---
		pn := 1 + rng.Intn(5)
		pelems := make([]string, pn)
		product := new(big.Rat).SetInt64(1)
		for j := range pelems {
			var sb strings.Builder
			if rng.Intn(2) == 1 {
				sb.WriteByte('-')
			}
			fmt.Fprintf(&sb, "%d.%03d", rng.Int63n(100), rng.Int63n(1000))
			pelems[j] = sb.String()
			product.Mul(product, mustRat(t, pelems[j]))
		}

		got, err = callAggregate(t, precisionProduct, pelems)
		if err != nil {
			t.Fatalf("product(%v): %v", pelems, err)
		}
		requireAggEquals(t, "product", pelems, got, product)
	}
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
