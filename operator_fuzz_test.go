package regobrick

import (
	"math/big"
	"regexp"
	"strings"
	"testing"
)

// strictDecimalRe matches plain decimal notation with an optional exponent, the
// grammar shared by expandExponent, udecimal and big.Rat. Comparisons against
// big.Rat are only made for inputs in this form so that Rat-specific extensions
// (fractions like "1/2", hex floats) cannot cause false mismatches.
var strictDecimalRe = regexp.MustCompile(`^[+-]?([0-9]+(\.[0-9]*)?|\.[0-9]+)([eE][+-]?[0-9]+)?$`)

func ratFromString(t *testing.T, s string) (*big.Rat, bool) {
	t.Helper()
	r, ok := new(big.Rat).SetString(s)
	return r, ok
}

// FuzzExpandExponent checks the invariants of the exponent-expansion path:
// no panic, bounded output size, no residual exponent marker, and exact value
// preservation verified against big.Rat.
func FuzzExpandExponent(f *testing.F) {
	seeds := []string{
		"1e-8", "-2.5E+3", "1.5e0", "1e0", "1E19", "-1e-19",
		"0.1", "123.45", "-0", "+5e2", "5.e1", ".5e1", ".e1", "e5", "1e",
		"1e-25", "1e30", "1e2000000000", "1e-2000000000",
		"00.00e00", "0.000000000000000000001e21", "9999999999999999999e-19",
		"1.2345678901234567890123e5", "١e2", "1e+0", "--1e2", "1.2.3e4",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, s string) {
		out := expandExponent(s) // must not panic

		if out == s {
			return
		}

		// The function only rewrites strings it fully recognized, so any change
		// implies a successful expansion.
		if strings.ContainsAny(out, "eE") {
			t.Fatalf("expandExponent(%q) = %q still contains an exponent marker", s, out)
		}
		// maxExpandedLen bounds the digits+exponent budget; one extra byte for the
		// sign is the only addition on top.
		if len(out) > maxExpandedLen+1 {
			t.Fatalf("expandExponent(%q) produced %d bytes (max %d): %q", s, len(out), maxExpandedLen+1, out)
		}

		if !strictDecimalRe.MatchString(s) {
			return
		}
		rIn, okIn := ratFromString(t, s)
		rOut, okOut := ratFromString(t, out)
		if !okIn || !okOut {
			// big.Rat and expandExponent may disagree on edge grammar (e.g. "5.");
			// value comparison is only meaningful when both sides parse.
			return
		}
		if rIn.Cmp(rOut) != 0 {
			t.Fatalf("expandExponent(%q) = %q changed the value: %s != %s", s, out, rIn.String(), rOut.String())
		}
	})
}

// FuzzParseDecimal checks that parseDecimal never panics and that every
// successful parse is exactly equal to the big.Rat interpretation of the input.
func FuzzParseDecimal(f *testing.F) {
	seeds := []string{
		"0", "1", "-1", "0.1", "-123.456", "1e-8", "-2.5E+3",
		"1e-25", "1e30", "1e2000000000", "0.1234567890123456789",
		"0.12345678901234567891", "34028236692093846346.3374607431768211455",
		"999999999999999999999", "abc", "", "NaN", "Inf", "0x10", "1/2", "1_000",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, s string) {
		d, err := parseDecimal(s) // must not panic
		if err != nil {
			return
		}

		if !strictDecimalRe.MatchString(s) {
			return
		}
		rIn, okIn := ratFromString(t, s)
		rOut, okOut := ratFromString(t, d.String())
		if !okIn || !okOut {
			return
		}
		if rIn.Cmp(rOut) != 0 {
			t.Fatalf("parseDecimal(%q) = %s: value differs from exact %s", s, d.String(), rIn.RatString())
		}
	})
}
