package regobrick

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
)

// Number is a numeric representation passed to Rego.
// It is emitted as a numeric literal during JSON serialization and integrates
// directly with numeric DB columns.
//
// Contract:
//   - Exponent notation (e/E) is supported. In UseDecimalArithmetic() operations
//     it is expanded to plain decimal notation before parsing (e.g. "1e-8" →
//     "0.00000001"), so it behaves the same as standard OPA. However, if the
//     expanded result exceeds udecimal's precision (19 decimal places, e.g.
//     "1e-25"), evaluation still fails with a parse error.
//   - Input validity is the provider's responsibility; regobrick validates only
//     at operation time.
//   - DB Scan accepts string, []byte, int64, and float64.
//   - Preserving the precision of DECIMAL columns is the driver's responsibility
//     (it must return string/[]byte). Verifying this up front with a driver
//     integrity test is recommended.
//
// NULL/empty handling (json.Number and decimal library conventions):
//   - JSON: null → empty string, empty string → emitted as 0 (Go zero-value
//     convention)
//   - DB: NULL/empty → error (decimal library convention)
//
// Example:
//
//	input := map[string]any{
//	    "price": regobrick.Number("123.45"),
//	}
type Number json.Number

// String returns the string representation.
func (n Number) String() string {
	return string(n)
}

// MarshalJSON emits a JSON numeric literal (without quotes).
// json.Number emits an empty string ("") as 0 (Go convention: the zero value is
// valid).
func (n Number) MarshalJSON() ([]byte, error) {
	return json.Marshal(json.Number(n))
}

// UnmarshalJSON parses a JSON number.
// json.Number treats null as an empty string ("").
// An empty string is emitted as 0 by MarshalJSON.
func (n *Number) UnmarshalJSON(b []byte) error {
	var num json.Number
	if err := json.Unmarshal(b, &num); err != nil {
		return err
	}
	*n = Number(num)
	return nil
}

// Scan implements sql.Scanner, reading a numeric column from the DB.
// NULL/empty values return an error (a convention of decimal libraries such as
// udecimal and shopspring).
func (n *Number) Scan(src any) error {
	if src == nil {
		return fmt.Errorf("Number.Scan: NULL not supported")
	}
	switch v := src.(type) {
	case string:
		if v == "" {
			return fmt.Errorf("Number.Scan: empty string not supported")
		}
		*n = Number(v)
	case []byte:
		if len(v) == 0 {
			return fmt.Errorf("Number.Scan: empty value not supported")
		}
		*n = Number(v)
	case int64:
		*n = Number(strconv.FormatInt(v, 10))
	case float64:
		if math.IsNaN(v) {
			return fmt.Errorf("Number.Scan: cannot scan NaN into Number")
		}
		if math.IsInf(v, 0) {
			return fmt.Errorf("Number.Scan: cannot scan Inf into Number")
		}
		*n = Number(strconv.FormatFloat(v, 'f', -1, 64))
	default:
		return fmt.Errorf("Number.Scan: unsupported type %T", src)
	}
	return nil
}

// Value implements driver.Valuer, writing to a DECIMAL/NUMERIC column in the DB.
// An empty value returns an error (decimal library convention).
func (n Number) Value() (driver.Value, error) {
	if n == "" {
		return nil, fmt.Errorf("Number.Value: empty value not supported")
	}
	return string(n), nil
}
