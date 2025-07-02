package regobrick

import (
	"encoding/json"
	"testing"

	"github.com/shopspring/decimal"
)

func TestNewRegoDecimal(t *testing.T) {
	tests := []struct {
		name     string
		input    decimal.Decimal
		expected string
	}{
		{
			name:     "positive decimal",
			input:    decimal.NewFromFloat(123.45),
			expected: "123.45",
		},
		{
			name:     "negative decimal",
			input:    decimal.NewFromFloat(-67.89),
			expected: "-67.89",
		},
		{
			name:     "zero",
			input:    decimal.Zero,
			expected: "0",
		},
		{
			name:     "large number",
			input:    decimal.NewFromInt(1000000),
			expected: "1000000",
		},
		{
			name:     "small decimal",
			input:    decimal.NewFromFloat(0.001),
			expected: "0.001",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rd := NewRegoDecimal(tt.input)
			if rd.String() != tt.expected {
				t.Errorf("NewRegoDecimal() = %v, want %v", rd.String(), tt.expected)
			}
		})
	}
}

func TestNewRegoDecimalFromInt(t *testing.T) {
	tests := []struct {
		name     string
		input    int64
		expected string
	}{
		{
			name:     "positive integer",
			input:    42,
			expected: "42",
		},
		{
			name:     "negative integer",
			input:    -123,
			expected: "-123",
		},
		{
			name:     "zero",
			input:    0,
			expected: "0",
		},
		{
			name:     "large integer",
			input:    9223372036854775807, // max int64
			expected: "9223372036854775807",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rd := NewRegoDecimalFromInt(tt.input)
			if rd.String() != tt.expected {
				t.Errorf("NewRegoDecimalFromInt() = %v, want %v", rd.String(), tt.expected)
			}
		})
	}
}

func TestRegoDecimalJSONMarshaling(t *testing.T) {
	tests := []struct {
		name     string
		input    RegoDecimal
		expected string
	}{
		{
			name:     "positive decimal",
			input:    NewRegoDecimal(decimal.NewFromFloat(123.45)),
			expected: "123.45",
		},
		{
			name:     "negative decimal",
			input:    NewRegoDecimal(decimal.NewFromFloat(-67.89)),
			expected: "-67.89",
		},
		{
			name:     "zero",
			input:    NewRegoDecimal(decimal.Zero),
			expected: "0",
		},
		{
			name:     "integer",
			input:    NewRegoDecimalFromInt(100),
			expected: "100",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jsonBytes, err := json.Marshal(tt.input)
			if err != nil {
				t.Fatalf("json.Marshal() error = %v", err)
			}

			jsonStr := string(jsonBytes)
			if jsonStr != tt.expected {
				t.Errorf("JSON marshaling = %v, want %v", jsonStr, tt.expected)
			}

			// Verify it's a numeric literal, not a string
			if jsonStr[0] == '"' || jsonStr[len(jsonStr)-1] == '"' {
				t.Errorf("JSON output should be numeric literal, got string: %v", jsonStr)
			}
		})
	}
}

func TestRegoDecimalInStruct(t *testing.T) {
	type TestStruct struct {
		Amount RegoDecimal `json:"amount"`
		Name   string      `json:"name"`
	}

	testData := TestStruct{
		Amount: NewRegoDecimal(decimal.NewFromFloat(99.99)),
		Name:   "test",
	}

	jsonBytes, err := json.Marshal(testData)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	expected := `{"amount":99.99,"name":"test"}`
	actual := string(jsonBytes)

	if actual != expected {
		t.Errorf("Struct JSON marshaling = %v, want %v", actual, expected)
	}
}

func TestRegoDecimalArithmetic(t *testing.T) {
	// Test basic arithmetic operations if they exist in the RegoDecimal type
	rd1 := NewRegoDecimal(decimal.NewFromFloat(10.5))
	rd2 := NewRegoDecimal(decimal.NewFromFloat(5.25))

	// Test addition
	sum := rd1.Decimal.Add(rd2.Decimal)
	expectedSum := "15.75"
	if sum.String() != expectedSum {
		t.Errorf("Add() = %v, want %v", sum.String(), expectedSum)
	}

	// Test subtraction
	diff := rd1.Decimal.Sub(rd2.Decimal)
	expectedDiff := "5.25"
	if diff.String() != expectedDiff {
		t.Errorf("Sub() = %v, want %v", diff.String(), expectedDiff)
	}
}

func TestRegoDecimalPrecision(t *testing.T) {
	// Test that RegoDecimal maintains precision better than float64
	tests := []struct {
		name     string
		value    string
		expected string
	}{
		{
			name:     "financial precision",
			value:    "123.456789",
			expected: "123.456789",
		},
		{
			name:     "many decimal places",
			value:    "0.123456789012345",
			expected: "0.123456789012345",
		},
		{
			name:     "large number with decimals",
			value:    "999999999.99",
			expected: "999999999.99",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dec, err := decimal.NewFromString(tt.value)
			if err != nil {
				t.Fatalf("decimal.NewFromString() error = %v", err)
			}

			rd := NewRegoDecimal(dec)
			if rd.String() != tt.expected {
				t.Errorf("Precision test = %v, want %v", rd.String(), tt.expected)
			}
		})
	}
}
