package lint

import (
	"fmt"
	"slices"
	"strings"

	"github.com/samuel-form3/deslop-go/internal/model"
)

// moduleCycles finds sibling directories of a module that use each other in
// a circle. Go forbids import cycles between packages, but not between the
// directories that group them: when code under a/x uses code under a/y, and
// code under a/y uses code under a/x, neither x nor y is the layer below the
// other.
func moduleCycles(m *model.Model) []Finding {
	var found []Finding
	for _, d := range directoryGraphs(m) {
		for _, circle := range components(d.nodes(), d.edges) {
			if f, ok := d.finding(circle); ok {
				found = append(found, f)
			}
		}
	}
	return found
}

// tie is one directory using another.
type tie struct{ from, to string }

// directoryGraph is how the child directories of one parent use each other.
type directoryGraph struct {
	parent string
	module string
	edges  map[string][]string
	refs   map[tie][]*model.Ref
}

// directoryGraphs groups the references between packages of a module by the
// deepest directory that holds both ends.
func directoryGraphs(m *model.Model) []*directoryGraph {
	byParent := map[string]*directoryGraph{}
	var graphs []*directoryGraph
	for _, r := range m.Refs {
		from, to := r.Pkg, r.Thing.Pkg
		if from == to || from.Module != to.Module {
			continue
		}
		parent, a, b, ok := siblings(from.Path, to.Path)
		if !ok {
			continue
		}
		d := byParent[parent]
		if d == nil {
			d = &directoryGraph{parent: parent, module: from.Module, edges: map[string][]string{}, refs: map[tie][]*model.Ref{}}
			byParent[parent] = d
			graphs = append(graphs, d)
		}
		t := tie{a, b}
		if d.refs[t] == nil {
			d.edges[a] = append(d.edges[a], b)
		}
		d.refs[t] = append(d.refs[t], r)
	}
	for _, d := range graphs {
		for _, to := range d.edges {
			slices.Sort(to)
		}
	}
	return graphs
}

func (d *directoryGraph) nodes() []string {
	var nodes []string
	for from := range d.edges {
		nodes = append(nodes, from)
	}
	slices.Sort(nodes)
	return nodes
}

func (d *directoryGraph) finding(circle []string) (Finding, bool) {
	slices.Sort(circle)
	names := make([]string, len(circle))
	for i, c := range circle {
		names[i] = relPath(d.module, c)
	}
	notes := []Note{{Text: "members: " + list(names)}}
	inScope := false
	for _, from := range circle {
		for _, to := range d.edges[from] {
			if !slices.Contains(circle, to) {
				continue
			}
			refs := d.refs[tie{from, to}]
			var items []string
			for _, r := range refs {
				inScope = inScope || r.Pkg.InScope || r.Thing.Pkg.InScope
				items = append(items, r.Item())
			}
			slices.Sort(items)
			items = slices.Compact(items)
			notes = append(notes, Note{Pos: refs[0].Pos, Text: fmt.Sprintf("`%s` uses `%s`: %s %s",
				relPath(d.module, from), relPath(d.module, to), plural(len(items), "item"), list(items))})
		}
	}
	if !inScope {
		return Finding{}, false
	}
	return Finding{
		Lint:    ModuleCycle,
		Key:     strings.Join(circle, ","),
		Message: fmt.Sprintf("%s in `%s` use each other in a circle", plural(len(circle), "directory"), relPath(d.module, d.parent)),
		Pos:     notes[1].Pos,
		Notes:   notes,
	}, true
}

// siblings returns the deepest directory holding both package paths, and its
// two children that hold each. It fails if one path holds the other.
func siblings(a, b string) (parent, childA, childB string, ok bool) {
	as, bs := strings.Split(a, "/"), strings.Split(b, "/")
	i := 0
	for i < len(as) && i < len(bs) && as[i] == bs[i] {
		i++
	}
	if i == len(as) || i == len(bs) {
		return "", "", "", false
	}
	return strings.Join(as[:i], "/"), strings.Join(as[:i+1], "/"), strings.Join(bs[:i+1], "/"), true
}

// relPath is a path within a module, relative to the module's own path.
func relPath(module, path string) string {
	if path == module {
		return module
	}
	return strings.TrimPrefix(path, module+"/")
}
