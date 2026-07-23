package regobrick

import (
	"encoding/json"
	"math"
	"strconv"
	"testing"
)

// FuzzNumberJSONRoundTrip checks the JSON contract of Number: any value that
// unmarshals successfully must marshal back to valid JSON representing the
// same numeric value. This protects every downstream consumer that embeds
// Number in a struct it later re-serializes.
func FuzzNumberJSONRoundTrip(f *testing.F) {
	seeds := []string{
		`0`, `-1`, `123.45`, `1e-8`, `-2.5E+3`, `1e400`, `-0`,
		`0.30000000000000000000000001`, `null`, `"123"`, `"abc"`, `[]`, `{}`,
		`1e`, `--5`, ` 7 `, `0.1e309`,
	}
	for _, s := range seeds {
		f.Add([]byte(s))
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		var n Number
		if err := json.Unmarshal(data, &n); err != nil {
			return
		}
		// json.Number convention: null leaves the value empty; MarshalJSON emits
		// the empty value as 0. Both are documented on Number.
		out, err := json.Marshal(n)
		if err != nil {
			t.Fatalf("Number(%q) unmarshaled from %q but failed to marshal: %v", string(n), data, err)
		}
		if !json.Valid(out) {
			t.Fatalf("Number(%q) marshaled to invalid JSON: %q", string(n), out)
		}
		if string(n) == "" {
			if string(out) != "0" {
				t.Fatalf("empty Number marshaled to %q, want 0", out)
			}
			return
		}
		if string(out) != string(n) {
			t.Fatalf("round trip changed the literal: %q -> %q", string(n), out)
		}
	})
}

// FuzzNumberScanFloat checks the DB contract for float64 sources: every finite
// float scans successfully and the stored literal parses back to the identical
// float64 (strconv shortest round-trip); NaN and infinities are rejected.
func FuzzNumberScanFloat(f *testing.F) {
	seeds := []float64{
		0, math.Copysign(0, -1), 1, -1, 123.456, 1e-300, -1e300,
		math.MaxFloat64, math.SmallestNonzeroFloat64,
		math.NaN(), math.Inf(1), math.Inf(-1),
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, v float64) {
		var n Number
		err := n.Scan(v)
		if math.IsNaN(v) || math.IsInf(v, 0) {
			if err == nil {
				t.Fatalf("Scan(%v) must be rejected", v)
			}
			return
		}
		if err != nil {
			t.Fatalf("Scan(%v): %v", v, err)
		}
		back, perr := strconv.ParseFloat(string(n), 64)
		if perr != nil {
			t.Fatalf("Scan(%v) stored unparseable literal %q: %v", v, string(n), perr)
		}
		if back != v {
			t.Fatalf("Scan(%v) round-tripped to %v via %q", v, back, string(n))
		}
		// The stored literal must also be a valid driver.Value for write-back.
		if _, verr := n.Value(); verr != nil {
			t.Fatalf("Value() after Scan(%v): %v", v, verr)
		}
	})
}
