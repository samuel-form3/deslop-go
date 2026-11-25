// Package tree hands prefix down the levels of a nested structure. Each
// level has to pass it on: that is not a pass-through variable.
package tree

type Forest struct{ trees []*Tree }
type Tree struct{ root *Node }
type Node struct{ leaves []*Leaf }
type Leaf struct{ name string }

func (f *Forest) Label(prefix string) {
	for _, t := range f.trees {
		t.Label(prefix)
	}
}

func (t *Tree) Label(prefix string) {
	if t.root != nil {
		t.root.Label(prefix)
	}
}

func (n *Node) Label(prefix string) {
	for _, l := range n.leaves {
		l.Label(prefix)
	}
}

func (l *Leaf) Label(prefix string) { l.name = prefix + l.name }
