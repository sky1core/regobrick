// Package types provides wrappers around shopspring/decimal. These wrappers ensure JSON
// serialization outputs numeric literals (e.g. 123.456), instead of string values (e.g. "123.456").
package types

import (
	"github.com/shopspring/decimal"
)

type RegoDecimal struct {
	decimal.Decimal
}

// MarshalJSON writes out the decimal value without quotes.
// Example: 123.456 instead of "123.456".
func (rd RegoDecimal) MarshalJSON() ([]byte, error) {
	return []byte(rd.Decimal.String()), nil
}
