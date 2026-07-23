package types

import (
	"encoding/json"
	"testing"

	"github.com/shopspring/decimal"
)

func TestRegoDecimal_MarshalJSON(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"decimal", "123.456", "123.456"},
		{"integer", "42", "42"},
		{"negative", "-0.5", "-0.5"},
		{"zero value", "0", "0"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d, err := decimal.NewFromString(tc.in)
			if err != nil {
				t.Fatalf("setup: %v", err)
			}
			b, err := json.Marshal(RegoDecimal{Decimal: d})
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			// The whole point of RegoDecimal: a numeric literal, not a string.
			if string(b) != tc.want {
				t.Fatalf("got %s, want %s", b, tc.want)
			}
		})
	}
}

func TestRegoDecimal_MarshalJSON_InsideStruct(t *testing.T) {
	d, err := decimal.NewFromString("9.99")
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	b, err := json.Marshal(struct {
		Price RegoDecimal `json:"price"`
	}{Price: RegoDecimal{Decimal: d}})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(b) != `{"price":9.99}` {
		t.Fatalf("got %s", b)
	}
}
