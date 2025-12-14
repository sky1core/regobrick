package regobrick

import (
	"github.com/shopspring/decimal"
	"github.com/sky1core/regobrick/internal/types"
)

// RegoDecimal is an alias for types.RegoDecimal.
// It represents a numeric value that serializes to JSON as a numeric literal
// (e.g., 123.456) rather than a string (e.g., "123.456").
//
// Deprecated: Use Number instead. RegoDecimal will be removed in a future version.
type RegoDecimal = types.RegoDecimal

// NewRegoDecimal creates a RegoDecimal from an existing decimal.Decimal value.
// The resulting RegoDecimal retains the same precision and scale.
//
// Deprecated: Use Number(d.String()) instead.
func NewRegoDecimal(d decimal.Decimal) RegoDecimal {
	return RegoDecimal{
		Decimal: d,
	}
}

// NewRegoDecimalFromInt creates a RegoDecimal from an int64.
// This is a shortcut for decimal.NewFromInt(...) wrapped in a RegoDecimal.
//
// Deprecated: Use Number(strconv.FormatInt(i, 10)) instead.
func NewRegoDecimalFromInt(i int64) RegoDecimal {
	return RegoDecimal{
		Decimal: decimal.NewFromInt(i),
	}
}
