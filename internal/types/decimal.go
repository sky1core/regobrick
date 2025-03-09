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

// ---------- Arithmetic ----------

// Add returns rd + other.
func (rd RegoDecimal) Add(other RegoDecimal) RegoDecimal {
	return RegoDecimal{rd.Decimal.Add(other.Decimal)}
}

// Sub returns rd - other.
func (rd RegoDecimal) Sub(other RegoDecimal) RegoDecimal {
	return RegoDecimal{rd.Decimal.Sub(other.Decimal)}
}

// Mul returns rd * other.
func (rd RegoDecimal) Mul(other RegoDecimal) RegoDecimal {
	return RegoDecimal{rd.Decimal.Mul(other.Decimal)}
}

// Div returns rd / other.
// Note: shopspring/decimal.Div() applies internal rounding for non-terminating decimals.
func (rd RegoDecimal) Div(other RegoDecimal) RegoDecimal {
	return RegoDecimal{rd.Decimal.Div(other.Decimal)}
}

// Mod returns rd mod other.
func (rd RegoDecimal) Mod(other RegoDecimal) RegoDecimal {
	return RegoDecimal{rd.Decimal.Mod(other.Decimal)}
}

// Neg returns the negative of rd.
func (rd RegoDecimal) Neg() RegoDecimal {
	return RegoDecimal{rd.Decimal.Neg()}
}

// Abs returns the absolute value of rd.
func (rd RegoDecimal) Abs() RegoDecimal {
	return RegoDecimal{rd.Decimal.Abs()}
}

// Pow returns rd raised to the power of exp.
func (rd RegoDecimal) Pow(exp RegoDecimal) RegoDecimal {
	return RegoDecimal{rd.Decimal.Pow(exp.Decimal)}
}

// ---------- Rounding ----------

// Round rounds rd to the specified number of decimal places.
func (rd RegoDecimal) Round(places int32) RegoDecimal {
	return RegoDecimal{rd.Decimal.Round(places)}
}

// Floor returns the largest integer less than or equal to rd.
func (rd RegoDecimal) Floor() RegoDecimal {
	return RegoDecimal{rd.Decimal.Floor()}
}

// Ceil returns the smallest integer greater than or equal to rd.
func (rd RegoDecimal) Ceil() RegoDecimal {
	return RegoDecimal{rd.Decimal.Ceil()}
}

// Truncate trims rd to the given precision without rounding.
func (rd RegoDecimal) Truncate(precision int32) RegoDecimal {
	return RegoDecimal{rd.Decimal.Truncate(precision)}
}

// ---------- Comparison / Checks ----------

// Cmp compares rd and other, returning -1 if rd < other, 0 if equal, 1 if rd > other.
func (rd RegoDecimal) Cmp(other RegoDecimal) int {
	return rd.Decimal.Cmp(other.Decimal)
}

// Equal reports whether rd and other have the same value.
func (rd RegoDecimal) Equal(other RegoDecimal) bool {
	return rd.Decimal.Equal(other.Decimal)
}

// NotEqual reports whether rd and other differ.
func (rd RegoDecimal) NotEqual(other RegoDecimal) bool {
	return !rd.Decimal.Equal(other.Decimal)
}

// LessThan reports whether rd is less than other.
func (rd RegoDecimal) LessThan(other RegoDecimal) bool {
	return rd.Decimal.LessThan(other.Decimal)
}

// LessThanOrEqual reports whether rd is less than or equal to other.
func (rd RegoDecimal) LessThanOrEqual(other RegoDecimal) bool {
	return rd.Decimal.LessThanOrEqual(other.Decimal)
}

// GreaterThan reports whether rd is greater than other.
func (rd RegoDecimal) GreaterThan(other RegoDecimal) bool {
	return rd.Decimal.GreaterThan(other.Decimal)
}

// GreaterThanOrEqual reports whether rd is greater than or equal to other.
func (rd RegoDecimal) GreaterThanOrEqual(other RegoDecimal) bool {
	return rd.Decimal.GreaterThanOrEqual(other.Decimal)
}
