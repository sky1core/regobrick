// Package regobrick provides high-level functions for applying RegoBrick
// transformations and adding modules to OPA rego.Rego objects.
//
// # Custom builtin type conversion
//
// The Go types in a RegisterBuiltinX signature determine the Rego types the
// builtin is declared with, and every argument and return value is converted at
// the boundary through JSON.
//
//	Go type                Rego type   Round-trip characteristics
//	string                 string      verbatim, except for control characters (see below)
//	bool                   boolean     verbatim
//	int, int64, ...        number      exact within the Go type's range, except uint64
//	                                   return values above 2^63-1 (see below); a number
//	                                   with a fractional part is rejected, not truncated
//	float64                number      rounded to float64 precision; large magnitudes
//	                                   serialize in exponent notation (see below)
//	float32                number      rounded to float32 precision; serialized in plain
//	                                   decimal notation
//	json.Number, Number    number      digits preserved exactly in both directions;
//	                                   declare these when precision matters
//	time.Time              string      RFC3339 in both directions
//	struct                 object      keys are the JSON tags
//	[]T                    array       a nil slice returns null, an empty slice returns []
//	map[string]T           object      a nil map returns null
//	pointer                pointed-to  a nil pointer returns null
//	any                    any         any Rego value representable as JSON, which excludes sets
//
// Every scalar row above is declared to OPA with its concrete Rego type —
// including time.Time, which is declared as string — so a mismatched call is
// rejected at compile time, before evaluation. The composite rows (struct, []T,
// map[string]T, pointer, any) are declared as any, so their shape is checked
// only while converting, at evaluation time.
//
// # Custom builtin conversion limits
//
// Argument conversion decodes each argument's Rego representation as JSON, so a
// Rego value whose representation is not JSON cannot reach a builtin at all. The
// declared Go parameter type does not change this: the failure happens before
// the Go type is involved.
//
//   - Sets cannot be passed as arguments. A set's representation ({1, 2, 3}) is
//     not valid JSON, so the call fails — default mode: no result;
//     rego.StrictBuiltinErrors(true): eval_builtin_error with "invalid character
//     '1' looking for beginning of object key string". This also applies to a set
//     nested inside an object or array argument. Convert the set to an array
//     before passing it.
//   - Strings containing control characters cannot be passed as arguments. Rego
//     string representations are Go-quoted, so U+0001 becomes \x01 and U+0007
//     becomes \a — escapes JSON does not have. The call fails the same way, with
//     "invalid character 'x' in string escape code". Escapes shared with JSON
//     (\n, \t, ...) and printable non-ASCII text are unaffected.
//
// On the return side, two numeric conversions do not preserve what the Go value
// held:
//
//   - Returned float64 values are formatted with Go's shortest round-trip float
//     formatting, so a builtin returning float64(9007199254740993) yields
//     9.007199254740992e+15: the lost digit is float64's own precision limit, the
//     notation is the formatting.
//   - Returned uint64 values above 2^63-1 come back negative. OPA narrows a
//     returned uint64 to a signed int before building the Rego number (v1.11.0,
//     ast/interning.go), so uint64(1 << 63) reaches the policy as
//     -9223372036854775808 and uint64(math.MaxUint64) as -1, with no error raised
//     anywhere. Arguments are not affected — a uint64 parameter receives
//     18446744073709551615 intact — and neither is uint, which takes a different
//     conversion route.
//
// Declare json.Number or Number to get exact decimal digits in both cases.
package regobrick
