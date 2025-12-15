package regobrick

import (
	"encoding/json"
	"time"

	"github.com/open-policy-agent/opa/v1/rego"
	"github.com/open-policy-agent/opa/v1/types"
	"github.com/sky1core/regobrick/internal/builtin"
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
	categories    []string
	configurators []func(*rego.Function)
}

type BuiltinRegisterOption func(*builtinRegisterConfig)

func WithCategories(cats ...string) BuiltinRegisterOption {
	return func(cfg *builtinRegisterConfig) {
		cfg.categories = cats
	}
}

// WithNondeterministic marks the builtin as nondeterministic.
// Nondeterministic builtins may return different results for the same inputs.
func WithNondeterministic() BuiltinRegisterOption {
	return ConfigureFunction(func(f *rego.Function) {
		f.Nondeterministic = true
	})
}

// WithMemoize enables memoization for the builtin.
// Memoized builtins cache results for the same inputs within a single evaluation.
func WithMemoize() BuiltinRegisterOption {
	return ConfigureFunction(func(f *rego.Function) {
		f.Memoize = true
	})
}

// ConfigureFunction allows direct configuration of the rego.Function before registration.
// Use this for advanced options not directly supported by other options.
// Passing nil is a no-op.
func ConfigureFunction(configurator func(*rego.Function)) BuiltinRegisterOption {
	return func(cfg *builtinRegisterConfig) {
		if configurator != nil {
			cfg.configurators = append(cfg.configurators, configurator)
		}
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
	case json.Number, RegoDecimal, Number:
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

// RegisterBuiltin0_ registers a builtin with no arguments that returns only an error (null to Rego).
// Must be called during package initialization (init function).
// Calling after initialization may cause race conditions.
func RegisterBuiltin0_(name string, fn func(rego.BuiltinContext) error, opts ...BuiltinRegisterOption) {
	cfg := builtinRegisterConfig{}
	for _, opt := range opts {
		opt(&cfg)
	}
	decl := types.NewFunction(types.Args(), nil)
	regoFunc := &rego.Function{
		Name: name,
		Decl: decl,
	}
	for _, configurator := range cfg.configurators {
		configurator(regoFunc)
	}
	rego.RegisterBuiltinDyn(regoFunc, builtin.Adapter0_(fn))
	storeCustomCategories(name, cfg.categories)
}

// RegisterBuiltin1_ registers a builtin with 1 argument that returns only an error (null to Rego).
// Must be called during package initialization (init function).
// Calling after initialization may cause race conditions.
func RegisterBuiltin1_[T1 any](name string, fn func(rego.BuiltinContext, T1) error, opts ...BuiltinRegisterOption) {
	cfg := builtinRegisterConfig{}
	for _, opt := range opts {
		opt(&cfg)
	}
	p1 := regoTypeOf[T1]()
	decl := types.NewFunction(types.Args(p1), nil)
	regoFunc := &rego.Function{
		Name: name,
		Decl: decl,
	}
	for _, configurator := range cfg.configurators {
		configurator(regoFunc)
	}
	rego.RegisterBuiltinDyn(regoFunc, builtin.Adapter1_[T1](fn))
	storeCustomCategories(name, cfg.categories)
}

// RegisterBuiltin2_ registers a builtin with 2 arguments that returns only an error (null to Rego).
// Must be called during package initialization (init function).
// Calling after initialization may cause race conditions.
func RegisterBuiltin2_[T1 any, T2 any](name string, fn func(rego.BuiltinContext, T1, T2) error, opts ...BuiltinRegisterOption) {
	cfg := builtinRegisterConfig{}
	for _, opt := range opts {
		opt(&cfg)
	}
	p1 := regoTypeOf[T1]()
	p2 := regoTypeOf[T2]()
	decl := types.NewFunction(types.Args(p1, p2), nil)
	regoFunc := &rego.Function{
		Name: name,
		Decl: decl,
	}
	for _, configurator := range cfg.configurators {
		configurator(regoFunc)
	}
	rego.RegisterBuiltinDyn(regoFunc, builtin.Adapter2_[T1, T2](fn))
	storeCustomCategories(name, cfg.categories)
}

// RegisterBuiltin3_ registers a builtin with 3 arguments that returns only an error (null to Rego).
// Must be called during package initialization (init function).
// Calling after initialization may cause race conditions.
func RegisterBuiltin3_[T1 any, T2 any, T3 any](name string, fn func(rego.BuiltinContext, T1, T2, T3) error, opts ...BuiltinRegisterOption) {
	cfg := builtinRegisterConfig{}
	for _, opt := range opts {
		opt(&cfg)
	}
	p1 := regoTypeOf[T1]()
	p2 := regoTypeOf[T2]()
	p3 := regoTypeOf[T3]()
	decl := types.NewFunction(types.Args(p1, p2, p3), nil)
	regoFunc := &rego.Function{
		Name: name,
		Decl: decl,
	}
	for _, configurator := range cfg.configurators {
		configurator(regoFunc)
	}
	rego.RegisterBuiltinDyn(regoFunc, builtin.Adapter3_[T1, T2, T3](fn))
	storeCustomCategories(name, cfg.categories)
}

// RegisterBuiltin4_ registers a builtin with 4 arguments that returns only an error (null to Rego).
// Must be called during package initialization (init function).
// Calling after initialization may cause race conditions.
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
	regoFunc := &rego.Function{
		Name: name,
		Decl: decl,
	}
	for _, configurator := range cfg.configurators {
		configurator(regoFunc)
	}
	rego.RegisterBuiltinDyn(regoFunc, builtin.Adapter4_[T1, T2, T3, T4](fn))
	storeCustomCategories(name, cfg.categories)
}

// RegisterBuiltin5_ registers a builtin with 5 arguments that returns only an error (null to Rego).
// Must be called during package initialization (init function).
// Calling after initialization may cause race conditions.
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
	regoFunc := &rego.Function{
		Name: name,
		Decl: decl,
	}
	for _, configurator := range cfg.configurators {
		configurator(regoFunc)
	}
	rego.RegisterBuiltinDyn(regoFunc, builtin.Adapter5_[T1, T2, T3, T4, T5](fn))
	storeCustomCategories(name, cfg.categories)
}

// ------------------------------------------------------------
// RegisterBuiltinX (value + error) forms
// ------------------------------------------------------------

// RegisterBuiltin0 registers a builtin with no arguments.
// Must be called during package initialization (init function).
// Calling after initialization may cause race conditions.
func RegisterBuiltin0[R any](name string, fn func(rego.BuiltinContext) (R, error), opts ...BuiltinRegisterOption) {
	cfg := builtinRegisterConfig{}
	for _, opt := range opts {
		opt(&cfg)
	}
	rType := regoTypeOf[R]()
	decl := types.NewFunction(types.Args(), rType)
	regoFunc := &rego.Function{
		Name: name,
		Decl: decl,
	}
	for _, configurator := range cfg.configurators {
		configurator(regoFunc)
	}
	rego.RegisterBuiltinDyn(regoFunc, builtin.Adapter0(fn))
	storeCustomCategories(name, cfg.categories)
}

// RegisterBuiltin1 registers a builtin with 1 argument.
// Must be called during package initialization (init function).
// Calling after initialization may cause race conditions.
func RegisterBuiltin1[T1 any, R any](name string, fn func(rego.BuiltinContext, T1) (R, error), opts ...BuiltinRegisterOption) {
	cfg := builtinRegisterConfig{}
	for _, opt := range opts {
		opt(&cfg)
	}
	p1 := regoTypeOf[T1]()
	rType := regoTypeOf[R]()
	decl := types.NewFunction(types.Args(p1), rType)
	regoFunc := &rego.Function{
		Name: name,
		Decl: decl,
	}
	for _, configurator := range cfg.configurators {
		configurator(regoFunc)
	}
	rego.RegisterBuiltinDyn(regoFunc, builtin.Adapter1[T1, R](fn))
	storeCustomCategories(name, cfg.categories)
}

// RegisterBuiltin2 registers a builtin with 2 arguments.
// Must be called during package initialization (init function).
// Calling after initialization may cause race conditions.
func RegisterBuiltin2[T1 any, T2 any, R any](name string, fn func(rego.BuiltinContext, T1, T2) (R, error), opts ...BuiltinRegisterOption) {
	cfg := builtinRegisterConfig{}
	for _, opt := range opts {
		opt(&cfg)
	}
	p1 := regoTypeOf[T1]()
	p2 := regoTypeOf[T2]()
	rType := regoTypeOf[R]()
	decl := types.NewFunction(types.Args(p1, p2), rType)
	regoFunc := &rego.Function{
		Name: name,
		Decl: decl,
	}
	for _, configurator := range cfg.configurators {
		configurator(regoFunc)
	}
	rego.RegisterBuiltinDyn(regoFunc, builtin.Adapter2[T1, T2, R](fn))
	storeCustomCategories(name, cfg.categories)
}

// RegisterBuiltin3 registers a builtin with 3 arguments.
// Must be called during package initialization (init function).
// Calling after initialization may cause race conditions.
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
	regoFunc := &rego.Function{
		Name: name,
		Decl: decl,
	}
	for _, configurator := range cfg.configurators {
		configurator(regoFunc)
	}
	rego.RegisterBuiltinDyn(regoFunc, builtin.Adapter3[T1, T2, T3, R](fn))
	storeCustomCategories(name, cfg.categories)
}

// RegisterBuiltin4 registers a builtin with 4 arguments.
// Must be called during package initialization (init function).
// Calling after initialization may cause race conditions.
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
	regoFunc := &rego.Function{
		Name: name,
		Decl: decl,
	}
	for _, configurator := range cfg.configurators {
		configurator(regoFunc)
	}
	rego.RegisterBuiltinDyn(regoFunc, builtin.Adapter4[T1, T2, T3, T4, R](fn))
	storeCustomCategories(name, cfg.categories)
}

// RegisterBuiltin5 registers a builtin with 5 arguments.
// Must be called during package initialization (init function).
// Calling after initialization may cause race conditions.
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
	regoFunc := &rego.Function{
		Name: name,
		Decl: decl,
	}
	for _, configurator := range cfg.configurators {
		configurator(regoFunc)
	}
	rego.RegisterBuiltinDyn(regoFunc, builtin.Adapter5[T1, T2, T3, T4, T5, R](fn))
	storeCustomCategories(name, cfg.categories)
}
