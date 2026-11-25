// Package lint finds sloppy code structure in a model: packages that are
// shallow, fields that are known outside their struct, and code that knows
// other code in a circle.
package lint

import (
	"fmt"
	"go/token"
	"io"
	"path/filepath"
	"slices"
	"strings"

	"github.com/samuel-form3/deslop-go/internal/model"
)

// heldInMind is how many things a reader can keep in mind at once.
const heldInMind = 7

// Lint names. Each is a red flag of A Philosophy of Software Design.
const (
	ModuleCycle         = "module-cycle"
	Tangle              = "tangle"
	FieldLeak           = "field-leak"
	ShallowModule       = "shallow-module"
	PassThroughMethod   = "pass-through-method"
	PassThroughVariable = "pass-through-variable"
	Overexposure        = "overexposure"
	InformationLeakage  = "information-leakage"
	CommentRepeatsCode  = "comment-repeats-code"
	VagueName           = "vague-name"
)

// Lints are the names of all the lints, in the order Run reports them.
var Lints = []string{
	ModuleCycle, Tangle, FieldLeak, ShallowModule,
	PassThroughMethod, PassThroughVariable, Overexposure,
	InformationLeakage, CommentRepeatsCode, VagueName,
}

// Finding is one piece of sloppy structure.
type Finding struct {
	Lint    string
	Key     string // names the finding stably, for allow files
	Message string
	Pos     token.Position
	Notes   []Note
	Picture []string
}

// Note points at one place that makes up a finding.
type Note struct {
	Pos  token.Position
	Text string
}

// Run returns the findings about the packages in scope, in a stable order.
func Run(m *model.Model) []Finding {
	tangles := findTangles(m)
	var all []Finding
	for _, found := range [][]Finding{
		moduleCycles(m),
		tangles.findings(),
		fieldLeaks(m, tangles),
		shallowModules(m),
		passThroughMethods(m),
		passThroughVariables(m),
		overexposure(m),
		informationLeakage(m),
		commentsRepeatCode(m),
		vagueNames(m),
	} {
		slices.SortStableFunc(found, func(a, b Finding) int { return strings.Compare(a.Key, b.Key) })
		all = append(all, found...)
	}
	return all
}

// Print writes findings, with file names relative to base.
func Print(w io.Writer, findings []Finding, base string) {
	where := func(pos token.Position) string {
		if rel, err := filepath.Rel(base, pos.Filename); err == nil && !strings.HasPrefix(rel, "..") {
			pos.Filename = rel
		}
		return pos.String()
	}
	for _, f := range findings {
		fmt.Fprintf(w, "error[%s]: %s\n", f.Lint, f.Message)
		fmt.Fprintf(w, "  --> %s\n", where(f.Pos))
		for _, n := range f.Notes {
			if n.Pos.IsValid() {
				fmt.Fprintf(w, "  = note: %s: %s\n", where(n.Pos), n.Text)
			} else {
				fmt.Fprintf(w, "  = note: %s\n", n.Text)
			}
		}
		for _, line := range f.Picture {
			fmt.Fprintf(w, "    %s\n", line)
		}
		fmt.Fprintln(w)
	}
}

// list joins names in backquotes, showing no more than can be held in mind.
func list(items []string) string {
	quoted := make([]string, len(items))
	for i, item := range items {
		quoted[i] = "`" + item + "`"
	}
	return join(quoted)
}

// join joins names, showing no more than can be held in mind.
func join(items []string) string {
	shown := items[:min(len(items), heldInMind)]
	s := strings.Join(shown, ", ")
	if more := len(items) - len(shown); more > 0 {
		s += fmt.Sprintf(" and %d more", more)
	}
	return s
}

// Count says how many errors there are.
func Count(n int) string { return plural(n, "error") }

func plural(n int, word string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, word)
	}
	if stem, ok := strings.CutSuffix(word, "y"); ok {
		return fmt.Sprintf("%d %sies", n, stem)
	}
	return fmt.Sprintf("%d %ss", n, word)
}

// components returns the strongly connected components of a graph that have
// more than one node, in the order of the nodes given.
func components[N comparable](nodes []N, edges map[N][]N) [][]N {
	index := map[N]int{}
	low := map[N]int{}
	onStack := map[N]bool{}
	var stack []N
	var found [][]N
	var visit func(n N)
	visit = func(n N) {
		index[n] = len(index)
		low[n] = index[n]
		stack = append(stack, n)
		onStack[n] = true
		for _, next := range edges[n] {
			if _, seen := index[next]; !seen {
				visit(next)
				low[n] = min(low[n], low[next])
			} else if onStack[next] {
				low[n] = min(low[n], index[next])
			}
		}
		if low[n] != index[n] {
			return
		}
		var comp []N
		for {
			top := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			onStack[top] = false
			comp = append(comp, top)
			if top == n {
				break
			}
		}
		if len(comp) > 1 {
			found = append(found, comp)
		}
	}
	for _, n := range nodes {
		if _, seen := index[n]; !seen {
			visit(n)
		}
	}
	return found
}

// shortestCircle returns a shortest path that leaves from and comes back to
// start, through nodes of the same component only.
func shortestCircle[N comparable](start N, edges map[N][]N, in func(N) bool) []N {
	prev := map[N]N{}
	queue := []N{start}
	seen := map[N]bool{}
	for len(queue) > 0 {
		n := queue[0]
		queue = queue[1:]
		for _, next := range edges[n] {
			if !in(next) {
				continue
			}
			if next == start {
				path := []N{n}
				for path[0] != start {
					path = append([]N{prev[path[0]]}, path...)
				}
				return path
			}
			if !seen[next] {
				seen[next] = true
				prev[next] = n
				queue = append(queue, next)
			}
		}
	}
	return nil
}
