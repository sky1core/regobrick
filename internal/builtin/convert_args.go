package builtin

import (
	"fmt"

	"github.com/open-policy-agent/opa/v1/ast"
)

func convertArgs1[T1 any](terms []*ast.Term) (T1, error) {
	var zero T1
	if len(terms) < 1 {
		return zero, fmt.Errorf("expected 1 argument, got %d", len(terms))
	}
	var v0 T1
	if err := ast.As(terms[0].Value, &v0); err != nil {
		return zero, err
	}
	return v0, nil
}

func convertArgs2[T1 any, T2 any](terms []*ast.Term) (T1, T2, error) {
	var z1 T1
	var z2 T2
	if len(terms) < 2 {
		return z1, z2, fmt.Errorf("expected 2 arguments, got %d", len(terms))
	}
	var v0 T1
	if err := ast.As(terms[0].Value, &v0); err != nil {
		return z1, z2, err
	}
	var v1 T2
	if err := ast.As(terms[1].Value, &v1); err != nil {
		return z1, z2, err
	}
	return v0, v1, nil
}

func convertArgs3[T1 any, T2 any, T3 any](terms []*ast.Term) (T1, T2, T3, error) {
	var z1 T1
	var z2 T2
	var z3 T3
	if len(terms) < 3 {
		return z1, z2, z3, fmt.Errorf("expected 3 arguments, got %d", len(terms))
	}
	var v0 T1
	if err := ast.As(terms[0].Value, &v0); err != nil {
		return z1, z2, z3, err
	}
	var v1 T2
	if err := ast.As(terms[1].Value, &v1); err != nil {
		return z1, z2, z3, err
	}
	var v2 T3
	if err := ast.As(terms[2].Value, &v2); err != nil {
		return z1, z2, z3, err
	}
	return v0, v1, v2, nil
}

func convertArgs4[T1 any, T2 any, T3 any, T4 any](terms []*ast.Term) (T1, T2, T3, T4, error) {
	var z1 T1
	var z2 T2
	var z3 T3
	var z4 T4
	if len(terms) < 4 {
		return z1, z2, z3, z4, fmt.Errorf("expected 4 arguments, got %d", len(terms))
	}
	var v0 T1
	if err := ast.As(terms[0].Value, &v0); err != nil {
		return z1, z2, z3, z4, err
	}
	var v1 T2
	if err := ast.As(terms[1].Value, &v1); err != nil {
		return z1, z2, z3, z4, err
	}
	var v2 T3
	if err := ast.As(terms[2].Value, &v2); err != nil {
		return z1, z2, z3, z4, err
	}
	var v3 T4
	if err := ast.As(terms[3].Value, &v3); err != nil {
		return z1, z2, z3, z4, err
	}
	return v0, v1, v2, v3, nil
}

func convertArgs5[T1 any, T2 any, T3 any, T4 any, T5 any](terms []*ast.Term) (T1, T2, T3, T4, T5, error) {
	var z1 T1
	var z2 T2
	var z3 T3
	var z4 T4
	var z5 T5
	if len(terms) < 5 {
		return z1, z2, z3, z4, z5, fmt.Errorf("expected 5 arguments, got %d", len(terms))
	}
	var v0 T1
	if err := ast.As(terms[0].Value, &v0); err != nil {
		return z1, z2, z3, z4, z5, err
	}
	var v1 T2
	if err := ast.As(terms[1].Value, &v1); err != nil {
		return z1, z2, z3, z4, z5, err
	}
	var v2 T3
	if err := ast.As(terms[2].Value, &v2); err != nil {
		return z1, z2, z3, z4, z5, err
	}
	var v3 T4
	if err := ast.As(terms[3].Value, &v3); err != nil {
		return z1, z2, z3, z4, z5, err
	}
	var v4 T5
	if err := ast.As(terms[4].Value, &v4); err != nil {
		return z1, z2, z3, z4, z5, err
	}
	return v0, v1, v2, v3, v4, nil
}
