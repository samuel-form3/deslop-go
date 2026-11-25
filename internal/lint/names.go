package lint

import (
	"fmt"
	"strings"

	"github.com/samuel-form3/deslop-go/internal/model"
)

// vaguePackages are package names that say nothing about what a package
// provides, so they gather anything. Go code reads every name with its
// package ("version.Info", "store.Open"), so a vague package name makes
// every name in it vague too.
var vaguePackages = map[string]bool{
	"base": true, "common": true, "general": true, "helper": true, "helpers": true,
	"lib": true, "misc": true, "shared": true, "stuff": true, "util": true,
	"utility": true, "utilities": true, "utils": true,
}

// vagueNames finds packages named after nothing in particular.
func vagueNames(m *model.Model) []Finding {
	var found []Finding
	for _, pkg := range m.Packages {
		if !pkg.InScope || !vaguePackages[strings.ToLower(pkg.Name)] {
			continue
		}
		var names []string
		for _, t := range m.Things {
			if t.Pkg == pkg {
				names = append(names, t.Name)
			}
		}
		found = append(found, Finding{
			Lint:    VagueName,
			Key:     pkg.Path,
			Message: fmt.Sprintf("package `%s` is named after nothing it provides", pkg.Short()),
			Pos:     pkg.Pos,
			Notes:   []Note{{Text: fmt.Sprintf("it holds %s: %s", plural(len(names), "thing"), list(names))}},
		})
	}
	return found
}
