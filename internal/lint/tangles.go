package lint

import (
	"fmt"
	"slices"
	"strings"

	"github.com/samuel-form3/deslop-go/internal/model"
)

// tangles are the groups of things that know each other in a circle: a
// reader cannot understand any of them without all the others.
//
// Go packages cannot import each other in a circle, so a tangle never spans
// packages. Within a package, a tangle is sloppy when it holds more things
// than a reader can keep in mind.
type tangles struct {
	of     map[*model.Thing]int
	groups [][]*model.Thing
	edges  map[*model.Thing][]*model.Thing
	ties   map[[2]*model.Thing]*model.Ref
}

func findTangles(m *model.Model) *tangles {
	t := &tangles{
		of:    map[*model.Thing]int{},
		edges: map[*model.Thing][]*model.Thing{},
		ties:  map[[2]*model.Thing]*model.Ref{},
	}
	for _, r := range m.Refs {
		if r.User == nil || r.User == r.Thing {
			continue
		}
		tie := [2]*model.Thing{r.User, r.Thing}
		if _, ok := t.ties[tie]; !ok {
			t.ties[tie] = r
			t.edges[r.User] = append(t.edges[r.User], r.Thing)
		}
	}
	t.groups = components(m.Things, t.edges)
	for i, group := range t.groups {
		for _, thing := range group {
			t.of[thing] = i
		}
	}
	return t
}

// same tells whether two things are in the same tangle.
func (t *tangles) same(a, b *model.Thing) bool {
	ia, okA := t.of[a]
	ib, okB := t.of[b]
	return okA && okB && ia == ib
}

func (t *tangles) findings() []Finding {
	var found []Finding
	for i, group := range t.groups {
		if len(group) <= heldInMind || !group[0].Pkg.InScope {
			continue
		}
		members := slices.SortedFunc(slices.Values(group), func(a, b *model.Thing) int { return strings.Compare(a.Name, b.Name) })
		names := make([]string, len(members))
		files := map[string]bool{}
		for j, m := range members {
			names[j] = m.Name
			files[m.Pos.Filename] = true
		}
		pkg := members[0].Pkg

		notes := []Note{{Text: "members: " + list(names)}}
		circle := shortestCircle(members[0], t.edges, func(n *model.Thing) bool { return t.of[n] == i })
		for j, user := range circle {
			used := circle[(j+1)%len(circle)]
			r := t.ties[[2]*model.Thing{user, used}]
			notes = append(notes, Note{Pos: r.Pos, Text: fmt.Sprintf("`%s` uses `%s`", user.Name, shortItem(r))})
		}
		found = append(found, Finding{
			Lint: Tangle,
			Key:  pkg.Path + ":" + strings.Join(names, ","),
			Message: fmt.Sprintf("%s in `%s` know each other in a circle, across %s",
				plural(len(members), "thing"), pkg.Short(), plural(len(files), "file")),
			Pos:   members[0].Pos,
			Notes: notes,
		})
	}
	return found
}

// shortItem names what a reference points at, within its package.
func shortItem(r *model.Ref) string {
	return strings.TrimPrefix(r.Item(), r.Thing.Pkg.Short()+".")
}
