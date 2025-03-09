package regobrick

import (
	"github.com/sky1core/regobrick/internal/builtin"
	"time"

	"github.com/open-policy-agent/opa/v1/rego"
	"github.com/open-policy-agent/opa/v1/types"
)

// ------------------------------------------------------------
// Internal category storage
// ------------------------------------------------------------
var customBuiltinCategories = map[string][]string{}

func storeCustomCategories(name string, categories []string) {
	if len(categories) > 0 {
		customBuiltinCategories[name] = categories
	} else {
		customBuiltinCategories[name] = nil
	}
}

// ------------------------------------------------------------
// Builtin registration options
// ------------------------------------------------------------
type builtinRegisterConfig struct {
	categories       []string
	nondeterministic bool
	decimalAsDefault bool
}

type BuiltinRegisterOption func(*builtinRegisterConfig)

func WithCategories(cats ...string) BuiltinRegisterOption {
	return func(cfg *builtinRegisterConfig) {
		cfg.categories = cats
	}
}

func WithNondeterministic() BuiltinRegisterOption {
	return func(cfg *builtinRegisterConfig) {
		cfg.nondeterministic = true
	}
}

// When set, RegoToGo will convert ast.Number to decimal.Decimal by default.
func WithDefaultDecimal() BuiltinRegisterOption {
	return func(cfg *builtinRegisterConfig) {
		cfg.decimalAsDefault = true
	}
}

// ------------------------------------------------------------
// Helpers
// ------------------------------------------------------------
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
	case RegoDecimal:
		return types.N
	case time.Time:
		return types.S
	default:
		return types.A
	}
}

// ------------------------------------------------------------
// RegisterBuiltinX_ (error-only) forms
// ------------------------------------------------------------

// RegisterBuiltin0_ has no arguments, returns error => null
func RegisterBuiltin0_(name string, fn func(rego.BuiltinContext) error, opts ...BuiltinRegisterOption) {
	cfg := builtinRegisterConfig{}
	for _, opt := range opts {
		opt(&cfg)
	}
	decl := types.NewFunction(types.Args(), nil)
	rego.RegisterBuiltinDyn(
		&rego.Function{
			Name:             name,
			Decl:             decl,
			Nondeterministic: cfg.nondeterministic,
		},
		builtin.Adapter0_(fn),
	)
	storeCustomCategories(name, cfg.categories)
}

// RegisterBuiltin1_ has 1 argument, returns error => null
func RegisterBuiltin1_[T1 any](name string, fn func(rego.BuiltinContext, T1) error, opts ...BuiltinRegisterOption) {
	cfg := builtinRegisterConfig{}
	for _, opt := range opts {
		opt(&cfg)
	}
	p1 := regoTypeOf[T1]()
	decl := types.NewFunction(types.Args(p1), nil)
	rego.RegisterBuiltinDyn(
		&rego.Function{
			Name:             name,
			Decl:             decl,
			Nondeterministic: cfg.nondeterministic,
		},
		builtin.Adapter1_[T1](fn),
	)
	storeCustomCategories(name, cfg.categories)
}

// RegisterBuiltin2_
func RegisterBuiltin2_[T1 any, T2 any](name string, fn func(rego.BuiltinContext, T1, T2) error, opts ...BuiltinRegisterOption) {
	cfg := builtinRegisterConfig{}
	for _, opt := range opts {
		opt(&cfg)
	}
	p1 := regoTypeOf[T1]()
	p2 := regoTypeOf[T2]()
	decl := types.NewFunction(types.Args(p1, p2), nil)
	rego.RegisterBuiltinDyn(
		&rego.Function{
			Name:             name,
			Decl:             decl,
			Nondeterministic: cfg.nondeterministic,
		},
		builtin.Adapter2_[T1, T2](fn),
	)
	storeCustomCategories(name, cfg.categories)
}

// RegisterBuiltin3_
func RegisterBuiltin3_[T1 any, T2 any, T3 any](name string, fn func(rego.BuiltinContext, T1, T2, T3) error, opts ...BuiltinRegisterOption) {
	cfg := builtinRegisterConfig{}
	for _, opt := range opts {
		opt(&cfg)
	}
	p1 := regoTypeOf[T1]()
	p2 := regoTypeOf[T2]()
	p3 := regoTypeOf[T3]()
	decl := types.NewFunction(types.Args(p1, p2, p3), nil)
	rego.RegisterBuiltinDyn(
		&rego.Function{
			Name:             name,
			Decl:             decl,
			Nondeterministic: cfg.nondeterministic,
		},
		builtin.Adapter3_[T1, T2, T3](fn),
	)
	storeCustomCategories(name, cfg.categories)
}

// RegisterBuiltin4_
func RegisterBuiltin4_[T1 any, T2 any, T3 any, T4 any](name string, fn func(rego.BuiltinContext, T1, T2, T3, T4) error, opts ...BuiltinRegisterOption) {
	cfg := builtinRegisterConfig{}
	for _, opt := range opts {
		opt(&cfg)
	}
	p1 := regoTypeOf[T1]()
	p2 := regoTypeOf[T2]()
	p3 := regoTypeOf[T3]()
	p4 := regoTypeOf[T4]()
	decl := types.NewFunction(types.Args(p1, p2, p3, p4), nil)
	rego.RegisterBuiltinDyn(
		&rego.Function{
			Name:             name,
			Decl:             decl,
			Nondeterministic: cfg.nondeterministic,
		},
		builtin.Adapter4_[T1, T2, T3, T4](fn),
	)
	storeCustomCategories(name, cfg.categories)
}

// RegisterBuiltin5_
func RegisterBuiltin5_[T1 any, T2 any, T3 any, T4 any, T5 any](name string, fn func(rego.BuiltinContext, T1, T2, T3, T4, T5) error, opts ...BuiltinRegisterOption) {
	cfg := builtinRegisterConfig{}
	for _, opt := range opts {
		opt(&cfg)
	}
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
			Nondeterministic: cfg.nondeterministic,
		},
		builtin.Adapter5_[T1, T2, T3, T4, T5](fn),
	)
	storeCustomCategories(name, cfg.categories)
}

// ------------------------------------------------------------
// RegisterBuiltinX (value + error) forms
// ------------------------------------------------------------

// RegisterBuiltin0
func RegisterBuiltin0[R any](name string, fn func(rego.BuiltinContext) (R, error), opts ...BuiltinRegisterOption) {
	cfg := builtinRegisterConfig{}
	for _, opt := range opts {
		opt(&cfg)
	}
	rType := regoTypeOf[R]()
	decl := types.NewFunction(types.Args(), rType)
	rego.RegisterBuiltinDyn(
		&rego.Function{
			Name:             name,
			Decl:             decl,
			Nondeterministic: cfg.nondeterministic,
		},
		builtin.Adapter0(fn),
	)
	storeCustomCategories(name, cfg.categories)
}

// RegisterBuiltin1
func RegisterBuiltin1[T1 any, R any](name string, fn func(rego.BuiltinContext, T1) (R, error), opts ...BuiltinRegisterOption) {
	cfg := builtinRegisterConfig{}
	for _, opt := range opts {
		opt(&cfg)
	}
	p1 := regoTypeOf[T1]()
	rType := regoTypeOf[R]()
	decl := types.NewFunction(types.Args(p1), rType)
	rego.RegisterBuiltinDyn(
		&rego.Function{
			Name:             name,
			Decl:             decl,
			Nondeterministic: cfg.nondeterministic,
		},
		builtin.Adapter1[T1, R](fn),
	)
	storeCustomCategories(name, cfg.categories)
}

// RegisterBuiltin2
func RegisterBuiltin2[T1 any, T2 any, R any](name string, fn func(rego.BuiltinContext, T1, T2) (R, error), opts ...BuiltinRegisterOption) {
	cfg := builtinRegisterConfig{}
	for _, opt := range opts {
		opt(&cfg)
	}
	p1 := regoTypeOf[T1]()
	p2 := regoTypeOf[T2]()
	rType := regoTypeOf[R]()
	decl := types.NewFunction(types.Args(p1, p2), rType)
	rego.RegisterBuiltinDyn(
		&rego.Function{
			Name:             name,
			Decl:             decl,
			Nondeterministic: cfg.nondeterministic,
		},
		builtin.Adapter2[T1, T2, R](fn),
	)
	storeCustomCategories(name, cfg.categories)
}

// RegisterBuiltin3
func RegisterBuiltin3[T1 any, T2 any, T3 any, R any](name string, fn func(rego.BuiltinContext, T1, T2, T3) (R, error), opts ...BuiltinRegisterOption) {
	cfg := builtinRegisterConfig{}
	for _, opt := range opts {
		opt(&cfg)
	}
	p1 := regoTypeOf[T1]()
	p2 := regoTypeOf[T2]()
	p3 := regoTypeOf[T3]()
	rType := regoTypeOf[R]()
	decl := types.NewFunction(types.Args(p1, p2, p3), rType)
	rego.RegisterBuiltinDyn(
		&rego.Function{
			Name:             name,
			Decl:             decl,
			Nondeterministic: cfg.nondeterministic,
		},
		builtin.Adapter3[T1, T2, T3, R](fn),
	)
	storeCustomCategories(name, cfg.categories)
}

// RegisterBuiltin4
func RegisterBuiltin4[T1 any, T2 any, T3 any, T4 any, R any](name string, fn func(rego.BuiltinContext, T1, T2, T3, T4) (R, error), opts ...BuiltinRegisterOption) {
	cfg := builtinRegisterConfig{}
	for _, opt := range opts {
		opt(&cfg)
	}
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
			Nondeterministic: cfg.nondeterministic,
		},
		builtin.Adapter4[T1, T2, T3, T4, R](fn),
	)
	storeCustomCategories(name, cfg.categories)
}

// RegisterBuiltin5
func RegisterBuiltin5[T1 any, T2 any, T3 any, T4 any, T5 any, R any](name string, fn func(rego.BuiltinContext, T1, T2, T3, T4, T5) (R, error), opts ...BuiltinRegisterOption) {
	cfg := builtinRegisterConfig{}
	for _, opt := range opts {
		opt(&cfg)
	}
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
			Nondeterministic: cfg.nondeterministic,
		},
		builtin.Adapter5[T1, T2, T3, T4, T5, R](fn),
	)
	storeCustomCategories(name, cfg.categories)
}
