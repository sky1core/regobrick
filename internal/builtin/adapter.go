package builtin

import (
	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/rego"
	"reflect"
)

func isNil(v any) bool {
	if v == nil {
		return true
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Ptr, reflect.Slice, reflect.Map,
		reflect.Func, reflect.Chan, reflect.Interface:
		return rv.IsNil()
	default:
		return false
	}
}

func toTerm(res any) (*ast.Term, error) {
	if isNil(res) {
		return ast.NullTerm(), nil
	}
	val, err := ast.InterfaceToValue(res)
	if err != nil {
		return nil, err
	}
	return ast.NewTerm(val), nil
}

func Adapter0_(fn func(rego.BuiltinContext) error) func(rego.BuiltinContext, []*ast.Term) (*ast.Term, error) {
	return func(bctx rego.BuiltinContext, terms []*ast.Term) (*ast.Term, error) {
		// No arguments to convert
		if err := fn(bctx); err != nil {
			return nil, err
		}
		return ast.NullTerm(), nil
	}
}

func Adapter0[R any](fn func(rego.BuiltinContext) (R, error)) func(rego.BuiltinContext, []*ast.Term) (*ast.Term, error) {
	return func(bctx rego.BuiltinContext, terms []*ast.Term) (*ast.Term, error) {
		res, err := fn(bctx)
		if err != nil {
			return nil, err
		}
		return toTerm(res)
	}
}

func Adapter1_[T1 any](fn func(rego.BuiltinContext, T1) error) func(rego.BuiltinContext, []*ast.Term) (*ast.Term, error) {
	return func(bctx rego.BuiltinContext, terms []*ast.Term) (*ast.Term, error) {
		v0, err := convertArgs1[T1](terms)
		if err != nil {
			return nil, err
		}
		if err := fn(bctx, v0); err != nil {
			return nil, err
		}
		return ast.NullTerm(), nil
	}
}

func Adapter1[T1 any, R any](fn func(rego.BuiltinContext, T1) (R, error)) func(rego.BuiltinContext, []*ast.Term) (*ast.Term, error) {
	return func(bctx rego.BuiltinContext, terms []*ast.Term) (*ast.Term, error) {
		v0, err := convertArgs1[T1](terms)
		if err != nil {
			return nil, err
		}
		res, fnErr := fn(bctx, v0)
		if fnErr != nil {
			return nil, fnErr
		}
		return toTerm(res)
	}
}

func Adapter2_[T1 any, T2 any](fn func(rego.BuiltinContext, T1, T2) error) func(rego.BuiltinContext, []*ast.Term) (*ast.Term, error) {
	return func(bctx rego.BuiltinContext, terms []*ast.Term) (*ast.Term, error) {
		v0, v1, err := convertArgs2[T1, T2](terms)
		if err != nil {
			return nil, err
		}
		if err := fn(bctx, v0, v1); err != nil {
			return nil, err
		}
		return ast.NullTerm(), nil
	}
}

func Adapter2[T1 any, T2 any, R any](fn func(rego.BuiltinContext, T1, T2) (R, error)) func(rego.BuiltinContext, []*ast.Term) (*ast.Term, error) {
	return func(bctx rego.BuiltinContext, terms []*ast.Term) (*ast.Term, error) {
		v0, v1, err := convertArgs2[T1, T2](terms)
		if err != nil {
			return nil, err
		}
		res, fnErr := fn(bctx, v0, v1)
		if fnErr != nil {
			return nil, fnErr
		}
		return toTerm(res)
	}
}

func Adapter3_[T1 any, T2 any, T3 any](fn func(rego.BuiltinContext, T1, T2, T3) error) func(rego.BuiltinContext, []*ast.Term) (*ast.Term, error) {
	return func(bctx rego.BuiltinContext, terms []*ast.Term) (*ast.Term, error) {
		v0, v1, v2, err := convertArgs3[T1, T2, T3](terms)
		if err != nil {
			return nil, err
		}
		if err := fn(bctx, v0, v1, v2); err != nil {
			return nil, err
		}
		return ast.NullTerm(), nil
	}
}

func Adapter3[T1 any, T2 any, T3 any, R any](fn func(rego.BuiltinContext, T1, T2, T3) (R, error)) func(rego.BuiltinContext, []*ast.Term) (*ast.Term, error) {
	return func(bctx rego.BuiltinContext, terms []*ast.Term) (*ast.Term, error) {
		v0, v1, v2, err := convertArgs3[T1, T2, T3](terms)
		if err != nil {
			return nil, err
		}
		res, fnErr := fn(bctx, v0, v1, v2)
		if fnErr != nil {
			return nil, fnErr
		}
		return toTerm(res)
	}
}

func Adapter4_[T1 any, T2 any, T3 any, T4 any](fn func(rego.BuiltinContext, T1, T2, T3, T4) error) func(rego.BuiltinContext, []*ast.Term) (*ast.Term, error) {
	return func(bctx rego.BuiltinContext, terms []*ast.Term) (*ast.Term, error) {
		v0, v1, v2, v3, err := convertArgs4[T1, T2, T3, T4](terms)
		if err != nil {
			return nil, err
		}
		if err := fn(bctx, v0, v1, v2, v3); err != nil {
			return nil, err
		}
		return ast.NullTerm(), nil
	}
}

func Adapter4[T1 any, T2 any, T3 any, T4 any, R any](fn func(rego.BuiltinContext, T1, T2, T3, T4) (R, error)) func(rego.BuiltinContext, []*ast.Term) (*ast.Term, error) {
	return func(bctx rego.BuiltinContext, terms []*ast.Term) (*ast.Term, error) {
		v0, v1, v2, v3, err := convertArgs4[T1, T2, T3, T4](terms)
		if err != nil {
			return nil, err
		}
		res, fnErr := fn(bctx, v0, v1, v2, v3)
		if fnErr != nil {
			return nil, fnErr
		}
		return toTerm(res)
	}
}

func Adapter5_[T1 any, T2 any, T3 any, T4 any, T5 any](fn func(rego.BuiltinContext, T1, T2, T3, T4, T5) error) func(rego.BuiltinContext, []*ast.Term) (*ast.Term, error) {
	return func(bctx rego.BuiltinContext, terms []*ast.Term) (*ast.Term, error) {
		v0, v1, v2, v3, v4, err := convertArgs5[T1, T2, T3, T4, T5](terms)
		if err != nil {
			return nil, err
		}
		if err := fn(bctx, v0, v1, v2, v3, v4); err != nil {
			return nil, err
		}
		return ast.NullTerm(), nil
	}
}

func Adapter5[T1 any, T2 any, T3 any, T4 any, T5 any, R any](fn func(rego.BuiltinContext, T1, T2, T3, T4, T5) (R, error)) func(rego.BuiltinContext, []*ast.Term) (*ast.Term, error) {
	return func(bctx rego.BuiltinContext, terms []*ast.Term) (*ast.Term, error) {
		v0, v1, v2, v3, v4, err := convertArgs5[T1, T2, T3, T4, T5](terms)
		if err != nil {
			return nil, err
		}
		res, fnErr := fn(bctx, v0, v1, v2, v3, v4)
		if fnErr != nil {
			return nil, fnErr
		}
		return toTerm(res)
	}
}
