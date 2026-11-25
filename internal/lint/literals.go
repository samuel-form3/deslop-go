package lint

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/samuel-form3/deslop-go/internal/model"
)

// informationLeakage finds the same keys, paths, names or formats written
// out as string literals in more than one package. Each package knows the
// same design decision, so changing it means finding all of them. One
// package should own it, behind a constant or a function.
//
// Only literals that look like such decisions count: no spaces, at least a
// few letters, and some punctuation, such as "app.kubernetes.io/name",
// "X-Request-Id", "/var/lib/app" or "config.yaml". Messages, words and
// format strings do not. The literals that the same packages share are
// reported together, as one decision is often spelled in several literals.
func informationLeakage(m *model.Model) []Finding {
	byValue := map[string][]*model.Literal{}
	var values []string
	for _, l := range m.Literals {
		if !decision(l.Value) {
			continue
		}
		if byValue[l.Value] == nil {
			values = append(values, l.Value)
		}
		byValue[l.Value] = append(byValue[l.Value], l)
	}

	// Group the leaked values by the packages that share them.
	type group struct {
		pkgs   []string
		values []string
		first  []*model.Literal // where each value is first written
	}
	var groups []*group
	byPkgs := map[string]*group{}
	for _, value := range values {
		lits := byValue[value]
		var pkgs []string
		inScope := false
		for _, l := range lits {
			inScope = inScope || l.Pkg.InScope
			pkgs = append(pkgs, l.Pkg.Path)
		}
		slices.Sort(pkgs)
		pkgs = slices.Compact(pkgs)
		if len(pkgs) < 2 || !inScope {
			continue
		}
		key := strings.Join(pkgs, ",")
		g := byPkgs[key]
		if g == nil {
			g = &group{pkgs: pkgs}
			byPkgs[key] = g
			groups = append(groups, g)
		}
		g.values = append(g.values, value)
		g.first = append(g.first, lits[0])
	}

	var found []Finding
	for _, g := range groups {
		module := g.first[0].Pkg.Module
		names := make([]string, len(g.pkgs))
		for i, p := range g.pkgs {
			names[i] = relPath(module, p)
		}
		quoted := make([]string, len(g.values))
		var notes []Note
		for i, v := range g.values {
			quoted[i] = strconv.Quote(v)
			if i < heldInMind {
				notes = append(notes, Note{Pos: g.first[i].Pos, Text: fmt.Sprintf("%s, written %s", quoted[i], plural(len(byValue[v]), "time"))})
			}
		}
		if more := len(g.values) - heldInMind; more > 0 {
			notes = append(notes, Note{Text: fmt.Sprintf("and %d more", more)})
		}
		what := quoted[0] + " is"
		if len(quoted) > 1 {
			what = fmt.Sprintf("%d literals are", len(quoted))
		}
		found = append(found, Finding{
			Lint:    InformationLeakage,
			Key:     strings.Join(g.pkgs, ",") + ":" + strings.Join(quoted, ","),
			Message: fmt.Sprintf("%s written in each of %s", what, list(names)),
			Pos:     g.first[0].Pos,
			Notes:   notes,
		})
	}
	return found
}

// decision tells whether a string literal looks like a design decision: a
// key, path, name or format, rather than a word or a message.
func decision(s string) bool {
	if len(s) < 5 || strings.ContainsAny(s, "%{}") {
		return false
	}
	var letters, marks int
	for _, r := range s {
		switch {
		case unicode.IsSpace(r):
			return false
		case unicode.IsLetter(r):
			letters++
		case strings.ContainsRune("./-_:", r):
			marks++
		}
	}
	return letters >= 3 && marks > 0
}
