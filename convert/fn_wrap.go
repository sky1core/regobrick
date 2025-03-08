package convert

import (
	"fmt"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/rego"
)

// fnWrapXConvert functions now accept ropts ...RegoToGoOption and pass them to RegoToGo.

func fnWrap1Convert[T1 any](terms []*ast.Term, ropts ...RegoToGoOption) (T1, error) {
	var zero T1
	if len(terms) < 1 {
		return zero, fmt.Errorf("expected 1 argument, got %d", len(terms))
	}
	v0, err := RegoToGo[T1](terms[0].Value, ropts...)
	if err != nil {
		return zero, err
	}
	return v0, nil
}

func fnWrap2Convert[T1 any, T2 any](terms []*ast.Term, ropts ...RegoToGoOption) (T1, T2, error) {
	var z1 T1
	var z2 T2
	if len(terms) < 2 {
		return z1, z2, fmt.Errorf("expected 2 arguments, got %d", len(terms))
	}
	v0, err := RegoToGo[T1](terms[0].Value, ropts...)
	if err != nil {
		return z1, z2, err
	}
	v1, err := RegoToGo[T2](terms[1].Value, ropts...)
	if err != nil {
		return z1, z2, err
	}
	return v0, v1, nil
}

func fnWrap3Convert[T1 any, T2 any, T3 any](terms []*ast.Term, ropts ...RegoToGoOption) (T1, T2, T3, error) {
	var z1 T1
	var z2 T2
	var z3 T3
	if len(terms) < 3 {
		return z1, z2, z3, fmt.Errorf("expected 3 arguments, got %d", len(terms))
	}
	v0, err := RegoToGo[T1](terms[0].Value, ropts...)
	if err != nil {
		return z1, z2, z3, err
	}
	v1, err := RegoToGo[T2](terms[1].Value, ropts...)
	if err != nil {
		return z1, z2, z3, err
	}
	v2, err := RegoToGo[T3](terms[2].Value, ropts...)
	if err != nil {
		return z1, z2, z3, err
	}
	return v0, v1, v2, nil
}

func fnWrap4Convert[T1 any, T2 any, T3 any, T4 any](terms []*ast.Term, ropts ...RegoToGoOption) (T1, T2, T3, T4, error) {
	var z1 T1
	var z2 T2
	var z3 T3
	var z4 T4
	if len(terms) < 4 {
		return z1, z2, z3, z4, fmt.Errorf("expected 4 arguments, got %d", len(terms))
	}
	v0, err := RegoToGo[T1](terms[0].Value, ropts...)
	if err != nil {
		return z1, z2, z3, z4, err
	}
	v1, err := RegoToGo[T2](terms[1].Value, ropts...)
	if err != nil {
		return z1, z2, z3, z4, err
	}
	v2, err := RegoToGo[T3](terms[2].Value, ropts...)
	if err != nil {
		return z1, z2, z3, z4, err
	}
	v3, err := RegoToGo[T4](terms[3].Value, ropts...)
	if err != nil {
		return z1, z2, z3, z4, err
	}
	return v0, v1, v2, v3, nil
}

func fnWrap5Convert[T1 any, T2 any, T3 any, T4 any, T5 any](terms []*ast.Term, ropts ...RegoToGoOption) (T1, T2, T3, T4, T5, error) {
	var z1 T1
	var z2 T2
	var z3 T3
	var z4 T4
	var z5 T5
	if len(terms) < 5 {
		return z1, z2, z3, z4, z5, fmt.Errorf("expected 5 arguments, got %d", len(terms))
	}
	v0, err := RegoToGo[T1](terms[0].Value, ropts...)
	if err != nil {
		return z1, z2, z3, z4, z5, err
	}
	v1, err := RegoToGo[T2](terms[1].Value, ropts...)
	if err != nil {
		return z1, z2, z3, z4, z5, err
	}
	v2, err := RegoToGo[T3](terms[2].Value, ropts...)
	if err != nil {
		return z1, z2, z3, z4, z5, err
	}
	v3, err := RegoToGo[T4](terms[3].Value, ropts...)
	if err != nil {
		return z1, z2, z3, z4, z5, err
	}
	v4, err := RegoToGo[T5](terms[4].Value, ropts...)
	if err != nil {
		return z1, z2, z3, z4, z5, err
	}
	return v0, v1, v2, v3, v4, nil
}

// FnWrapX_ and FnWrapX now accept ropts and pass them to fnWrapXConvert.

func FnWrap0_(fn func(rego.BuiltinContext) error, ropts ...RegoToGoOption) func(rego.BuiltinContext, []*ast.Term) (*ast.Term, error) {
	return func(bctx rego.BuiltinContext, terms []*ast.Term) (*ast.Term, error) {
		// No terms to convert, so just call fn.
		if err := fn(bctx); err != nil {
			return nil, err
		}
		return ast.NullTerm(), nil
	}
}

func FnWrap0[R any](fn func(rego.BuiltinContext) (R, error), ropts ...RegoToGoOption) func(rego.BuiltinContext, []*ast.Term) (*ast.Term, error) {
	return func(bctx rego.BuiltinContext, terms []*ast.Term) (*ast.Term, error) {
		// No terms to convert, so just call fn.
		res, err := fn(bctx)
		if err != nil {
			return nil, err
		}
		val, convErr := GoToRego(res)
		if convErr != nil {
			return nil, convErr
		}
		return ast.NewTerm(val), nil
	}
}

func FnWrap1_[T1 any](fn func(rego.BuiltinContext, T1) error, ropts ...RegoToGoOption) func(rego.BuiltinContext, []*ast.Term) (*ast.Term, error) {
	return func(bctx rego.BuiltinContext, terms []*ast.Term) (*ast.Term, error) {
		v0, err := fnWrap1Convert[T1](terms, ropts...)
		if err != nil {
			return nil, err
		}
		if err := fn(bctx, v0); err != nil {
			return nil, err
		}
		return ast.NullTerm(), nil
	}
}

func FnWrap1[T1 any, R any](fn func(rego.BuiltinContext, T1) (R, error), ropts ...RegoToGoOption) func(rego.BuiltinContext, []*ast.Term) (*ast.Term, error) {
	return func(bctx rego.BuiltinContext, terms []*ast.Term) (*ast.Term, error) {
		v0, err := fnWrap1Convert[T1](terms, ropts...)
		if err != nil {
			return nil, err
		}
		res, fnErr := fn(bctx, v0)
		if fnErr != nil {
			return nil, fnErr
		}
		val, convErr := GoToRego(res)
		if convErr != nil {
			return nil, convErr
		}
		return ast.NewTerm(val), nil
	}
}

func FnWrap2_[T1 any, T2 any](fn func(rego.BuiltinContext, T1, T2) error, ropts ...RegoToGoOption) func(rego.BuiltinContext, []*ast.Term) (*ast.Term, error) {
	return func(bctx rego.BuiltinContext, terms []*ast.Term) (*ast.Term, error) {
		v0, v1, err := fnWrap2Convert[T1, T2](terms, ropts...)
		if err != nil {
			return nil, err
		}
		if err := fn(bctx, v0, v1); err != nil {
			return nil, err
		}
		return ast.NullTerm(), nil
	}
}

func FnWrap2[T1 any, T2 any, R any](fn func(rego.BuiltinContext, T1, T2) (R, error), ropts ...RegoToGoOption) func(rego.BuiltinContext, []*ast.Term) (*ast.Term, error) {
	return func(bctx rego.BuiltinContext, terms []*ast.Term) (*ast.Term, error) {
		v0, v1, err := fnWrap2Convert[T1, T2](terms, ropts...)
		if err != nil {
			return nil, err
		}
		res, fnErr := fn(bctx, v0, v1)
		if fnErr != nil {
			return nil, fnErr
		}
		val, convErr := GoToRego(res)
		if convErr != nil {
			return nil, convErr
		}
		return ast.NewTerm(val), nil
	}
}

func FnWrap3_[T1 any, T2 any, T3 any](fn func(rego.BuiltinContext, T1, T2, T3) error, ropts ...RegoToGoOption) func(rego.BuiltinContext, []*ast.Term) (*ast.Term, error) {
	return func(bctx rego.BuiltinContext, terms []*ast.Term) (*ast.Term, error) {
		v0, v1, v2, err := fnWrap3Convert[T1, T2, T3](terms, ropts...)
		if err != nil {
			return nil, err
		}
		if err := fn(bctx, v0, v1, v2); err != nil {
			return nil, err
		}
		return ast.NullTerm(), nil
	}
}

func FnWrap3[T1 any, T2 any, T3 any, R any](fn func(rego.BuiltinContext, T1, T2, T3) (R, error), ropts ...RegoToGoOption) func(rego.BuiltinContext, []*ast.Term) (*ast.Term, error) {
	return func(bctx rego.BuiltinContext, terms []*ast.Term) (*ast.Term, error) {
		v0, v1, v2, err := fnWrap3Convert[T1, T2, T3](terms, ropts...)
		if err != nil {
			return nil, err
		}
		res, fnErr := fn(bctx, v0, v1, v2)
		if fnErr != nil {
			return nil, fnErr
		}
		val, convErr := GoToRego(res)
		if convErr != nil {
			return nil, convErr
		}
		return ast.NewTerm(val), nil
	}
}

func FnWrap4_[T1 any, T2 any, T3 any, T4 any](fn func(rego.BuiltinContext, T1, T2, T3, T4) error, ropts ...RegoToGoOption) func(rego.BuiltinContext, []*ast.Term) (*ast.Term, error) {
	return func(bctx rego.BuiltinContext, terms []*ast.Term) (*ast.Term, error) {
		v0, v1, v2, v3, err := fnWrap4Convert[T1, T2, T3, T4](terms, ropts...)
		if err != nil {
			return nil, err
		}
		if err := fn(bctx, v0, v1, v2, v3); err != nil {
			return nil, err
		}
		return ast.NullTerm(), nil
	}
}

func FnWrap4[T1 any, T2 any, T3 any, T4 any, R any](fn func(rego.BuiltinContext, T1, T2, T3, T4) (R, error), ropts ...RegoToGoOption) func(rego.BuiltinContext, []*ast.Term) (*ast.Term, error) {
	return func(bctx rego.BuiltinContext, terms []*ast.Term) (*ast.Term, error) {
		v0, v1, v2, v3, err := fnWrap4Convert[T1, T2, T3, T4](terms, ropts...)
		if err != nil {
			return nil, err
		}
		res, fnErr := fn(bctx, v0, v1, v2, v3)
		if fnErr != nil {
			return nil, fnErr
		}
		val, convErr := GoToRego(res)
		if convErr != nil {
			return nil, convErr
		}
		return ast.NewTerm(val), nil
	}
}

func FnWrap5_[T1 any, T2 any, T3 any, T4 any, T5 any](fn func(rego.BuiltinContext, T1, T2, T3, T4, T5) error, ropts ...RegoToGoOption) func(rego.BuiltinContext, []*ast.Term) (*ast.Term, error) {
	return func(bctx rego.BuiltinContext, terms []*ast.Term) (*ast.Term, error) {
		v0, v1, v2, v3, v4, err := fnWrap5Convert[T1, T2, T3, T4, T5](terms, ropts...)
		if err != nil {
			return nil, err
		}
		if err := fn(bctx, v0, v1, v2, v3, v4); err != nil {
			return nil, err
		}
		return ast.NullTerm(), nil
	}
}

func FnWrap5[T1 any, T2 any, T3 any, T4 any, T5 any, R any](fn func(rego.BuiltinContext, T1, T2, T3, T4, T5) (R, error), ropts ...RegoToGoOption) func(rego.BuiltinContext, []*ast.Term) (*ast.Term, error) {
	return func(bctx rego.BuiltinContext, terms []*ast.Term) (*ast.Term, error) {
		v0, v1, v2, v3, v4, err := fnWrap5Convert[T1, T2, T3, T4, T5](terms, ropts...)
		if err != nil {
			return nil, err
		}
		res, fnErr := fn(bctx, v0, v1, v2, v3, v4)
		if fnErr != nil {
			return nil, fnErr
		}
		val, convErr := GoToRego(res)
		if convErr != nil {
			return nil, convErr
		}
		return ast.NewTerm(val), nil
	}
}
