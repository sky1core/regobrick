package module

import (
	"github.com/open-policy-agent/opa/v1/rego"
)

type ModuleOption struct {
	Filename string
	Source   string
	Imports  []string
}

func Module(filename, source string, imports []string) func(*rego.Rego) {
	return func(r *rego.Rego) {
		// Try to parse upfront, checking for syntax errors.
		parsedModule, err := ParseModule(filename, source, imports)
		if err != nil {
			rego.Module(filename, source)(r)
			return
		}
		rego.ParsedModule(parsedModule)(r)
	}
}

// Modules adds multiple Rego modules to the Brick.
func Modules(moduleOpts ...ModuleOption) func(*rego.Rego) {
	return func(r *rego.Rego) {
		for _, opt := range moduleOpts {
			Module(opt.Filename, opt.Source, opt.Imports)(r)
		}
	}
}
