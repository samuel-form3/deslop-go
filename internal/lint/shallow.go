package lint

import (
	"fmt"
	"slices"

	"github.com/samuel-form3/deslop-go/internal/model"
)

// breakEven is how many functions a package must hide for each one it
// shows, not to be shallow.
const breakEven = 1

// shallowModules finds packages that show other packages of the module as
// many functions as they hide, and more than a reader can keep in mind in
// one place: one type with many functions, many types tied together by
// functions, or many free functions.
//
// Functions count for nothing when they only wrap another function or a
// field, or when they may be called through an interface instead of by name.
func shallowModules(m *model.Model) []Finding {
	shown := map[*model.Func]bool{}
	uses := map[*model.Func][]*model.Thing{}
	for _, r := range m.Refs {
		if r.Func != nil && r.Pkg != r.Func.Thing.Pkg {
			shown[r.Func] = true
		}
		if f := r.UserFunc; f != nil && r.Thing.Kind == model.Type && r.Thing.Pkg == r.Pkg && r.Thing != f.Thing {
			uses[f] = append(uses[f], r.Thing)
		}
	}

	funcs := map[*model.Package][]*model.Func{}
	for _, t := range m.Things {
		for _, f := range t.Funcs {
			if !f.Abstract && !f.Satisfies && !f.Wrapper && f.Name != "init" && f.Name != "main" {
				funcs[t.Pkg] = append(funcs[t.Pkg], f)
			}
		}
	}

	var found []Finding
	for _, pkg := range m.Packages {
		if !pkg.InScope {
			continue
		}
		var showing []*model.Func
		for _, f := range funcs[pkg] {
			if shown[f] {
				showing = append(showing, f)
			}
		}
		hidden := len(funcs[pkg]) - len(showing)
		if len(showing) <= heldInMind || hidden > len(showing)*breakEven {
			continue
		}
		lists := groupShown(showing, uses)
		if lists[0].things <= heldInMind && len(lists[0].funcs) <= heldInMind {
			continue
		}
		var notes []Note
		for i, l := range lists {
			if i == heldInMind {
				notes = append(notes, Note{Text: fmt.Sprintf("and %s more", plural(len(lists)-i, "list"))})
				break
			}
			names := make([]string, len(l.funcs))
			for j, f := range l.funcs {
				names[j] = f.Name
				if f.Method {
					names[j] = f.Thing.Name + "." + f.Name
				}
			}
			notes = append(notes, Note{Pos: l.funcs[0].Pos, Text: fmt.Sprintf("%s: %s shown: %s",
				l.name, plural(len(l.funcs), "function"), list(names))})
		}
		found = append(found, Finding{
			Lint:    ShallowModule,
			Key:     pkg.Path,
			Message: fmt.Sprintf("`%s` is shallow: it shows %d of its %s", pkg.Short(), len(showing), plural(len(funcs[pkg]), "function")),
			Pos:     pkg.Pos,
			Notes:   notes,
		})
	}
	return found
}

// shownList is the functions shown for one type, or for types that free
// functions tie together, or for no type at all.
type shownList struct {
	name   string
	things int
	funcs  []*model.Func
}

// groupShown sorts shown functions into lists, widest first. A method or a
// constructor goes on the list of its type. A free function goes on the list
// of the types of its package that it uses, tying them into one list, or on
// the list of free functions if it uses none.
func groupShown(showing []*model.Func, uses map[*model.Func][]*model.Thing) []*shownList {
	root := map[*model.Thing]*model.Thing{}
	var find func(t *model.Thing) *model.Thing
	find = func(t *model.Thing) *model.Thing {
		if r, ok := root[t]; ok && r != t {
			root[t] = find(r)
			return root[t]
		}
		root[t] = t
		return t
	}
	free := &model.Thing{Name: "free functions"}
	home := map[*model.Func]*model.Thing{}
	for _, f := range showing {
		switch {
		case f.Thing.Kind == model.Type:
			home[f] = f.Thing
		case len(uses[f]) == 0:
			home[f] = free
		default:
			home[f] = uses[f][0]
			for _, t := range uses[f][1:] {
				root[find(t)] = find(uses[f][0])
			}
		}
		find(home[f])
	}

	byRoot := map[*model.Thing]*shownList{}
	var all []*shownList
	for _, f := range showing {
		r := find(home[f])
		l := byRoot[r]
		if l == nil {
			l = &shownList{}
			byRoot[r] = l
			all = append(all, l)
		}
		l.funcs = append(l.funcs, f)
	}
	members := map[*model.Thing][]string{}
	for t := range root {
		if t != free {
			members[find(t)] = append(members[find(t)], t.Name)
		}
	}
	for r, l := range byRoot {
		if r == free {
			l.name = "free functions"
			l.things = len(l.funcs)
			continue
		}
		names := members[r]
		slices.Sort(names)
		l.things = len(names)
		l.name = list(names)
	}
	slices.SortStableFunc(all, func(a, b *shownList) int { return len(b.funcs) - len(a.funcs) })
	return all
}
