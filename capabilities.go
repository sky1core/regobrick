package regobrick

import (
	"github.com/open-policy-agent/opa/v1/ast"
)

// coreInfixes is a list of infixes that should always be allowed.
var coreInfixes = []string{
	"=",
	":=",
	"in",
}

// infixAllowed checks if the given infix is in the coreInfixes list.
func infixAllowed(infix string) bool {
	for _, core := range coreInfixes {
		if infix == core {
			return true
		}
	}
	return false
}

// FilterCapabilities filters OPA built-in functions based on a list of allowed names
// and categories, along with built-ins whose infix is in coreInfixes.
//
//   - If a built-in's infix is one of the core infixes, it is kept.
//   - Otherwise, if the built-in name is in allowedNames or any of its categories are in allowedCats,
//     it is kept.
//   - If a built-in has custom category mappings, those are also checked against allowedCats.
//
// The resulting capabilities object contains only the filtered built-ins.
func FilterCapabilities(allowedNames []string, allowedCats []string) *ast.Capabilities {
	nameSet := make(map[string]bool, len(allowedNames))
	for _, n := range allowedNames {
		nameSet[n] = true
	}
	catSet := make(map[string]bool, len(allowedCats))
	for _, c := range allowedCats {
		catSet[c] = true
	}

	baseCaps := ast.CapabilitiesForThisVersion()

	filtered := make([]*ast.Builtin, 0, len(baseCaps.Builtins))
nextBuiltin:
	for _, b := range baseCaps.Builtins {
		// Always allow built-ins with infix in coreInfixes.
		if infixAllowed(b.Infix) {
			filtered = append(filtered, b)
			continue
		}

		// Allow built-ins if their name is explicitly allowed.
		if nameSet[b.Name] {
			filtered = append(filtered, b)
			continue
		}

		// Allow built-ins if any of their categories is explicitly allowed.
		for _, cat := range b.Categories {
			if catSet[cat] {
				filtered = append(filtered, b)
				continue nextBuiltin
			}
		}

		// 커스텀 빌트인에 대한 카테고리 매핑을 확인합니다.
		if customCats, ok := customBuiltinCategories[b.Name]; ok {
			for _, cat := range customCats {
				if catSet[cat] {
					filtered = append(filtered, b)
					continue nextBuiltin
				}
			}
		}
	}

	// Copy base capabilities but replace built-ins with the filtered list.
	newCaps := *baseCaps
	newCaps.Builtins = filtered
	return &newCaps
}
