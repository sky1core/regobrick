package regobrick

import (
	"github.com/sky1core/regobrick/convert"
	"time"

	"github.com/open-policy-agent/opa/rego"
	"github.com/open-policy-agent/opa/types"
	"github.com/shopspring/decimal"
)

// regoTypeOf maps a generic Go type T to a Rego type.
// Recognized basic types return their specific Rego type;
// all others default to types.A (Any).
func regoTypeOf[T any]() types.Type {
	var zero T
	switch any(zero).(type) {
	case bool:
		return types.B
	case int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64,
		float32, float64:
		return types.N
	case string:
		return types.S
	case decimal.Decimal, decimal.NullDecimal:
		return types.N
	case time.Time:
		return types.S
	default:
		return types.A
	}
}

// RegisterBuiltin0_ registers a function with no arguments and returns only error.
// On the Rego side, this builtin produces null.
func RegisterBuiltin0_(name string, nondeterministic bool, fn func(ctx rego.BuiltinContext) error) {
	decl := types.NewFunction(types.Args(), nil)
	rego.RegisterBuiltinDyn(
		&rego.Function{
			Name:             name,
			Decl:             decl,
			Nondeterministic: nondeterministic,
		},
		convert.FnWrap0_(fn),
	)
}

// RegisterBuiltin0 registers a function with no arguments, returning a value + error.
func RegisterBuiltin0[R any](name string, nondeterministic bool, fn func(ctx rego.BuiltinContext) (R, error)) {
	rType := regoTypeOf[R]()
	decl := types.NewFunction(types.Args(), rType)
	rego.RegisterBuiltinDyn(
		&rego.Function{
			Name:             name,
			Decl:             decl,
			Nondeterministic: nondeterministic,
		},
		convert.FnWrap0(fn),
	)
}

// RegisterBuiltin1_ registers a function with 1 argument and returns only error.
// On the Rego side, this builtin produces null.
func RegisterBuiltin1_[T1 any](name string, nondeterministic bool, fn func(ctx rego.BuiltinContext, arg1 T1) error) {
	p1 := regoTypeOf[T1]()
	decl := types.NewFunction(types.Args(p1), nil)
	rego.RegisterBuiltinDyn(
		&rego.Function{
			Name:             name,
			Decl:             decl,
			Nondeterministic: nondeterministic,
		},
		convert.FnWrap1_[T1](fn),
	)
}

// RegisterBuiltin1 registers a function with 1 argument, returning a value + error.
func RegisterBuiltin1[T1 any, R any](name string, nondeterministic bool, fn func(ctx rego.BuiltinContext, arg1 T1) (R, error)) {
	p1 := regoTypeOf[T1]()
	rType := regoTypeOf[R]()
	decl := types.NewFunction(types.Args(p1), rType)
	rego.RegisterBuiltinDyn(
		&rego.Function{
			Name:             name,
			Decl:             decl,
			Nondeterministic: nondeterministic,
		},
		convert.FnWrap1[T1, R](fn),
	)
}

// RegisterBuiltin2_ registers a function with 2 arguments and returns only error.
// On the Rego side, this builtin produces null.
func RegisterBuiltin2_[T1 any, T2 any](name string, nondeterministic bool, fn func(ctx rego.BuiltinContext, arg1 T1, arg2 T2) error) {
	p1 := regoTypeOf[T1]()
	p2 := regoTypeOf[T2]()
	decl := types.NewFunction(types.Args(p1, p2), nil)
	rego.RegisterBuiltinDyn(
		&rego.Function{
			Name:             name,
			Decl:             decl,
			Nondeterministic: nondeterministic,
		},
		convert.FnWrap2_[T1, T2](fn),
	)
}

// RegisterBuiltin2 registers a function with 2 arguments, returning a value + error.
func RegisterBuiltin2[T1 any, T2 any, R any](name string, nondeterministic bool, fn func(ctx rego.BuiltinContext, arg1 T1, arg2 T2) (R, error)) {
	p1 := regoTypeOf[T1]()
	p2 := regoTypeOf[T2]()
	rType := regoTypeOf[R]()
	decl := types.NewFunction(types.Args(p1, p2), rType)
	rego.RegisterBuiltinDyn(
		&rego.Function{
			Name:             name,
			Decl:             decl,
			Nondeterministic: nondeterministic,
		},
		convert.FnWrap2[T1, T2, R](fn),
	)
}

// RegisterBuiltin3_ registers a function with 3 arguments and returns only error.
// On the Rego side, this builtin produces null.
func RegisterBuiltin3_[T1 any, T2 any, T3 any](name string, nondeterministic bool, fn func(ctx rego.BuiltinContext, arg1 T1, arg2 T2, arg3 T3) error) {
	p1 := regoTypeOf[T1]()
	p2 := regoTypeOf[T2]()
	p3 := regoTypeOf[T3]()
	decl := types.NewFunction(types.Args(p1, p2, p3), nil)
	rego.RegisterBuiltinDyn(
		&rego.Function{
			Name:             name,
			Decl:             decl,
			Nondeterministic: nondeterministic,
		},
		convert.FnWrap3_[T1, T2, T3](fn),
	)
}

// RegisterBuiltin3 registers a function with 3 arguments, returning a value + error.
func RegisterBuiltin3[T1 any, T2 any, T3 any, R any](name string, nondeterministic bool, fn func(ctx rego.BuiltinContext, arg1 T1, arg2 T2, arg3 T3) (R, error)) {
	p1 := regoTypeOf[T1]()
	p2 := regoTypeOf[T2]()
	p3 := regoTypeOf[T3]()
	rType := regoTypeOf[R]()
	decl := types.NewFunction(types.Args(p1, p2, p3), rType)
	rego.RegisterBuiltinDyn(
		&rego.Function{
			Name:             name,
			Decl:             decl,
			Nondeterministic: nondeterministic,
		},
		convert.FnWrap3[T1, T2, T3, R](fn),
	)
}

// RegisterBuiltin4_ registers a function with 4 arguments and returns only error.
// On the Rego side, this builtin produces null.
func RegisterBuiltin4_[T1 any, T2 any, T3 any, T4 any](name string, nondeterministic bool, fn func(ctx rego.BuiltinContext, arg1 T1, arg2 T2, arg3 T3, arg4 T4) error) {
	p1 := regoTypeOf[T1]()
	p2 := regoTypeOf[T2]()
	p3 := regoTypeOf[T3]()
	p4 := regoTypeOf[T4]()
	decl := types.NewFunction(types.Args(p1, p2, p3, p4), nil)
	rego.RegisterBuiltinDyn(
		&rego.Function{
			Name:             name,
			Decl:             decl,
			Nondeterministic: nondeterministic,
		},
		convert.FnWrap4_[T1, T2, T3, T4](fn),
	)
}

// RegisterBuiltin4 registers a function with 4 arguments, returning a value + error.
func RegisterBuiltin4[T1 any, T2 any, T3 any, T4 any, R any](name string, nondeterministic bool, fn func(ctx rego.BuiltinContext, arg1 T1, arg2 T2, arg3 T3, arg4 T4) (R, error)) {
	p1 := regoTypeOf[T1]()
	p2 := regoTypeOf[T2]()
	p3 := regoTypeOf[T3]()
	p4 := regoTypeOf[T4]()
	rType := regoTypeOf[R]()
	decl := types.NewFunction(types.Args(p1, p2, p3, p4), rType)
	rego.RegisterBuiltinDyn(
		&rego.Function{
			Name:             name,
			Decl:             decl,
			Nondeterministic: nondeterministic,
		},
		convert.FnWrap4[T1, T2, T3, T4, R](fn),
	)
}

// RegisterBuiltin5_ registers a function with 5 arguments and returns only error.
// On the Rego side, this builtin produces null.
func RegisterBuiltin5_[T1 any, T2 any, T3 any, T4 any, T5 any](name string, nondeterministic bool, fn func(ctx rego.BuiltinContext, arg1 T1, arg2 T2, arg3 T3, arg4 T4, arg5 T5) error) {
	p1 := regoTypeOf[T1]()
	p2 := regoTypeOf[T2]()
	p3 := regoTypeOf[T3]()
	p4 := regoTypeOf[T4]()
	p5 := regoTypeOf[T5]()
	decl := types.NewFunction(types.Args(p1, p2, p3, p4, p5), nil)
	rego.RegisterBuiltinDyn(
		&rego.Function{
			Name:             name,
			Decl:             decl,
			Nondeterministic: nondeterministic,
		},
		convert.FnWrap5_[T1, T2, T3, T4, T5](fn),
	)
}

// RegisterBuiltin5 registers a function with 5 arguments, returning a value + error.
func RegisterBuiltin5[T1 any, T2 any, T3 any, T4 any, T5 any, R any](name string, nondeterministic bool, fn func(ctx rego.BuiltinContext, arg1 T1, arg2 T2, arg3 T3, arg4 T4, arg5 T5) (R, error)) {
	p1 := regoTypeOf[T1]()
	p2 := regoTypeOf[T2]()
	p3 := regoTypeOf[T3]()
	p4 := regoTypeOf[T4]()
	p5 := regoTypeOf[T5]()
	rType := regoTypeOf[R]()
	decl := types.NewFunction(types.Args(p1, p2, p3, p4, p5), rType)
	rego.RegisterBuiltinDyn(
		&rego.Function{
			Name:             name,
			Decl:             decl,
			Nondeterministic: nondeterministic,
		},
		convert.FnWrap5[T1, T2, T3, T4, T5, R](fn),
	)
}
