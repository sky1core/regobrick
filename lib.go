package regobrick

import (
	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/rego"

	// module.ParseModule(...) is in this package
	"github.com/sky1core/regobrick/module"
)

// RegoModule represents a single Rego module definition.
type RegoModule struct {
	Filename string
	Source   string
	Imports  []string
	AST      *ast.Module
}

// Option is used to collect module information before parsing.
type Option func(*[]RegoModule) error

// WithModule registers a Rego module (filename, source, imports).
func WithModule(filename, source string, imports []string) Option {
	return func(modules *[]RegoModule) error {
		*modules = append(*modules, RegoModule{
			Filename: filename,
			Source:   source,
			Imports:  imports,
		})
		return nil
	}
}

// New processes the given options, parses each Rego module,
// and returns a function that can be directly passed to rego.New(...).
//
// Example:
//
//	fn, err := regobrick.New(
//	    regobrick.WithModule("modA.rego", regoSrc, []string{"data.regobrick.default_false"}),
//	)
//	r := rego.New(rego.Query("data.demo.allow"), fn)
func New(opts ...Option) (func(*rego.Rego), error) {
	var modules []RegoModule

	// Collect modules from all options
	for _, opt := range opts {
		if err := opt(&modules); err != nil {
			return nil, err
		}
	}

	// Parse each module
	for i, rm := range modules {
		parsed, err := module.ParseModule(rm.Filename, rm.Source, rm.Imports)
		if err != nil {
			return nil, err
		}
		modules[i].AST = parsed
	}

	// Return a function pointer that can be passed into rego.New(...)
	fn := func(r *rego.Rego) {
		for _, m := range modules {
			if m.AST != nil {
				rego.ParsedModule(m.AST)(r)
			}
		}
	}

	return fn, nil
}
