package module

import (
	"fmt"
	"strings"

	"github.com/open-policy-agent/opa/v1/rego"
)

type ModuleOption struct {
	Filename string
	Source   string
	Imports  []string
}

// Module returns a rego.Rego option that adds a single Rego module built from
// filename, source, and optional imports.
//
// Fail-fast contract: when ParseModule fails, Module panics with a message of the
// form `regobrick: cannot process module "<filename>": <cause>` UNLESS the caller
// requested nothing from regobrick — that is, imports is empty AND the source does
// not reference any "data.regobrick." feature. In that "nothing requested" case
// Module falls back to rego.Module(filename, source) so that plain (e.g. v0)
// workflows keep working and surface their own errors during compilation.
//
// A panic therefore only happens when regobrick was actually asked to do something
// (inject imports or apply a feature transform) but could not, which would
// otherwise silently drop the requested behavior.
func Module(filename, source string, imports []string) func(*rego.Rego) {
	return func(r *rego.Rego) {
		parsedModule, err := ParseModule(filename, source, imports)
		if err != nil {
			// Nothing regobrick-specific was requested: preserve the historical
			// fallback so plain modules (including v0 syntax) still compile.
			if len(imports) == 0 && !strings.Contains(source, "data.regobrick.") {
				rego.Module(filename, source)(r)
				return
			}
			panic(fmt.Sprintf("regobrick: cannot process module %q: %v", filename, err))
		}
		rego.ParsedModule(parsedModule)(r)
	}
}

// Modules adds multiple Rego modules to the Brick. It applies Module to each
// option and follows the same fail-fast contract: a module that requested
// regobrick behavior (imports or a "data.regobrick." feature) but failed to parse
// causes a panic, while a fully plain module falls back to rego.Module.
func Modules(moduleOpts ...ModuleOption) func(*rego.Rego) {
	return func(r *rego.Rego) {
		for _, opt := range moduleOpts {
			Module(opt.Filename, opt.Source, opt.Imports)(r)
		}
	}
}
