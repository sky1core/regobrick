package regobrick

import (
	"github.com/open-policy-agent/opa/v1/rego"
	"github.com/sky1core/regobrick/convert"
	"github.com/sky1core/regobrick/module"
)

// ModuleOption holds information about a Rego module.
type ModuleOption struct {
	Filename string
	Source   string
	Imports  []string
}

// Brick holds configuration for RegoBrick.
type Brick struct {
	modules  []ModuleOption
	rawInput interface{}
}

type FnBrickOption = func(*Brick)
type FnRegoOption = func(*rego.Rego)

// New returns a new Brick.
func New(options ...FnBrickOption) *Brick {
	b := &Brick{}
	for _, opt := range options {
		opt(b)
	}
	return b
}

// Rego builds a rego.Rego instance using the configured modules and input.
func (b *Brick) Rego(regoOpts ...FnRegoOption) (*rego.Rego, error) {
	opts := make([]FnRegoOption, 0)

	for _, m := range b.modules {
		parsedModule, err := module.ParseModule(m.Filename, m.Source, m.Imports)
		if err != nil {
			return nil, err
		}

		opts = append(opts, rego.ParsedModule(parsedModule))
	}

	if b.rawInput != nil {
		parsedInput, err := convert.GoToRego(b.rawInput)
		if err != nil {
			return nil, err
		}
		opts = append(opts, rego.ParsedInput(parsedInput))
	}

	opts = append(opts, regoOpts...)

	return rego.New(opts...), nil
}

// Module adds a single Rego module to the Brick.
func Module(filename, source string, imports []string) FnBrickOption {
	return func(b *Brick) {
		b.modules = append(b.modules, ModuleOption{
			Filename: filename,
			Source:   source,
			Imports:  imports,
		})
	}
}

// Modules adds multiple Rego modules to the Brick.
func Modules(moduleOpts ...ModuleOption) FnBrickOption {
	return func(b *Brick) {
		b.modules = append(b.modules, moduleOpts...)
	}
}

// Input sets the input data for evaluation.
// If the value is a decimal.Decimal, it is converted into a JSON number
// rather than a string, preserving numeric precision.
func Input(input interface{}) FnBrickOption {
	return func(b *Brick) {
		b.rawInput = input
	}
}
