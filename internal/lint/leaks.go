package lint

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/samuel-form3/deslop-go/internal/model"
)

// fieldLeaks finds fields that code outside their struct works on, when the
// struct otherwise guards its fields: it has methods that work on them, and
// some field that nothing outside knows.
//
// The struct is its methods, its constructors, the types of its package that
// embed it, and whatever it is tangled with; everything else is outside,
// whatever the visibility of the field. Reading a copy of plain data, or
// setting a field in a composite literal, is not working on a field; writing
// it, taking its address, indexing it or calling its methods is, and so is
// calling a getter or setter that does it.
func fieldLeaks(m *model.Model, t *tangles) []Finding {
	outside := map[*model.Field][]*model.Ref{}
	working := map[*model.Thing]bool{}
	for _, r := range m.Refs {
		if r.Field == nil || r.User == nil {
			continue
		}
		owner := r.Field.Thing
		if r.User == owner || slices.Contains(r.User.Embeds, owner) || t.same(r.User, owner) {
			if r.UserFunc != nil && r.UserFunc.Method && r.UserFunc.Thing == owner {
				working[owner] = true
			}
			continue
		}
		outside[r.Field] = append(outside[r.Field], r)
	}

	leaking := func(f *model.Field) []*model.Ref {
		var refs []*model.Ref
		for _, r := range outside[f] {
			if !r.Copy && !r.Init {
				refs = append(refs, r)
			}
		}
		return refs
	}

	var found []Finding
	for _, thing := range m.Things {
		if !working[thing] {
			continue
		}
		var kept, known []string
		for _, f := range thing.Fields {
			if len(outside[f]) == 0 {
				kept = append(kept, f.Name)
			} else if len(leaking(f)) > 0 {
				known = append(known, f.Name)
			}
		}
		if len(kept) == 0 {
			continue
		}
		for _, f := range thing.Fields {
			refs := leaking(f)
			if len(refs) == 0 {
				continue
			}
			inScope := thing.Pkg.InScope
			for _, r := range refs {
				inScope = inScope || r.Pkg.InScope
			}
			if !inScope {
				continue
			}
			found = append(found, leak(thing, f, refs, kept, known))
		}
	}
	return found
}

func leak(thing *model.Thing, f *model.Field, refs []*model.Ref, kept, known []string) Finding {
	byPkg := map[*model.Package]int{}
	var pkgs []*model.Package
	for _, r := range refs {
		if byPkg[r.Pkg] == 0 {
			pkgs = append(pkgs, r.Pkg)
		}
		byPkg[r.Pkg]++
	}
	slices.SortStableFunc(pkgs, func(a, b *model.Package) int {
		return cmp.Or(byPkg[b]-byPkg[a], strings.Compare(a.Path, b.Path))
	})

	var notes []Note
	for i, r := range refs {
		if i == heldInMind {
			notes = append(notes, Note{Text: fmt.Sprintf("and %d more", len(refs)-i)})
			break
		}
		user := r.User.String()
		if r.UserFunc != nil {
			user = r.UserFunc.String()
		}
		notes = append(notes, Note{Pos: r.Pos, Text: "`" + user + "`"})
	}

	var others []string
	for _, name := range known {
		if name != f.Name {
			others = append(others, name)
		}
	}

	picture := []string{thing.String()}
	head := "  " + f.Name + " ◀━"
	pad := strings.Repeat(" ", len([]rune(head)))
	width := len(fmt.Sprint(byPkg[pkgs[0]]))
	shown := pkgs
	if len(shown) > heldInMind {
		shown = shown[:heldInMind]
	}
	for i, p := range shown {
		branch := pad + "┣━ "
		switch {
		case len(shown) == 1 && len(pkgs) == 1:
			branch = head + "━━ "
		case i == 0:
			branch = head + "┳━ "
		case i == len(shown)-1 && len(pkgs) == len(shown):
			branch = pad + "┗━ "
		}
		picture = append(picture, fmt.Sprintf("%s%*d  %s", branch, width, byPkg[p], p.Short()))
	}
	if more := len(pkgs) - len(shown); more > 0 {
		picture = append(picture, fmt.Sprintf("%s┗━ %s more", pad, plural(more, "package")))
	}
	if len(others) > 0 {
		picture = append(picture, "  also known outside: "+join(others))
	}
	picture = append(picture, "  kept: "+join(kept))

	return Finding{
		Lint: FieldLeak,
		Key:  f.Thing.Key() + "." + f.Name,
		Message: fmt.Sprintf("`%s` is known outside `%s`: %s in %s",
			f.String(), thing.Name, plural(len(refs), "place"), plural(len(pkgs), "package")),
		Pos:     refs[0].Pos,
		Notes:   notes,
		Picture: picture,
	}
}
