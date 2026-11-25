package lint

import (
	"fmt"
	"go/token"
	"strings"

	"github.com/samuel-form3/deslop-go/internal/model"
)

// passThroughMethods finds types whose methods mostly pass their parameters
// straight on to a method of one of their fields, or to a function of another
// package. Such a type adds a layer without adding an abstraction: it is
// unclear which of the two layers is responsible for what.
//
// Methods that implement an interface are left out: decorators and adapters
// have to have the interface's signature.
func passThroughMethods(m *model.Model) []Finding {
	var found []Finding
	for _, t := range m.Things {
		if t.Kind != model.Type || t.Interface || !t.Pkg.InScope {
			continue
		}
		var methods int
		var passing []*model.Func
		for _, f := range t.Funcs {
			if !f.Method {
				continue
			}
			methods++
			if f.PassesTo != "" && !f.Satisfies {
				passing = append(passing, f)
			}
		}
		if len(passing) < 2 || len(passing)*2 < methods {
			continue
		}
		var notes []Note
		for _, f := range passing[:min(len(passing), heldInMind)] {
			notes = append(notes, Note{Pos: f.Pos, Text: fmt.Sprintf("`%s` only calls `%s`", f.Name, f.PassesTo)})
		}
		found = append(found, Finding{
			Lint:    PassThroughMethod,
			Key:     t.Key(),
			Message: fmt.Sprintf("`%s` passes %d of its %s straight through", t, len(passing), plural(methods, "method")),
			Pos:     t.Pos,
			Notes:   notes,
		})
	}
	return found
}

// passThroughChain is how many functions in a row that only pass a value on
// make a long chain.
const passThroughChain = 3

// passThroughVariables finds parameters handed down a long chain of
// functions that do nothing with them but pass them on. Each function in the
// chain has to know about a value that only the end of the chain needs.
//
// A context.Context is left out: it is Go's own answer to the problem.
// Functions that only forward, or implement an interface, or are taken as
// values, are left out as well, since their signature is not theirs to
// choose. So are chains of methods of the same name, which hand a value down
// the levels of a nested structure.
func passThroughVariables(m *model.Model) []Finding {
	g := newPassGraph(m)
	var chains [][]*model.Param
	byFuncs := map[string][]*model.Param{} // the first parameter of each chain, by the functions it runs through
	for _, t := range m.Things {
		for _, f := range t.Funcs {
			for _, p := range f.Params {
				if !g.start(p) {
					continue
				}
				chain := g.chain(p, map[*model.Param]bool{})
				if len(chain) < passThroughChain || !inScope(chain) || sameName(chain) {
					continue
				}
				key := chainKey(chain)
				if byFuncs[key] == nil {
					chains = append(chains, chain)
				}
				byFuncs[key] = append(byFuncs[key], p)
			}
		}
	}
	var found []Finding
	for _, chain := range chains {
		found = append(found, g.finding(chain, byFuncs[chainKey(chain)]))
	}
	return found
}

func chainKey(chain []*model.Param) string {
	keys := make([]string, len(chain))
	for i, p := range chain {
		keys[i] = p.Func.Key()
	}
	return strings.Join(keys, ",")
}

func sameName(chain []*model.Param) bool {
	for _, p := range chain {
		if p.Func.Name != chain[0].Func.Name {
			return false
		}
	}
	return true
}

// passGraph links each parameter that is only passed on to the parameters it
// is passed into.
type passGraph struct {
	escapes map[*model.Func]bool
	next    map[*model.Param][]hop
	fed     map[*model.Param]bool // passed in by a middleman
	memo    map[*model.Param][]*model.Param
}

type hop struct {
	to *model.Param
	at token.Position
}

func newPassGraph(m *model.Model) *passGraph {
	g := &passGraph{
		escapes: escaping(m),
		next:    map[*model.Param][]hop{},
		fed:     map[*model.Param]bool{},
		memo:    map[*model.Param][]*model.Param{},
	}
	for _, c := range m.Calls {
		for i, arg := range c.Args {
			to := paramAt(c.Callee, i)
			if arg.Param == nil || to == nil || !g.middle(arg.Param) {
				continue
			}
			g.next[arg.Param] = append(g.next[arg.Param], hop{to, c.Pos})
			if g.middle(to) {
				g.fed[to] = true
			}
		}
	}
	return g
}

// middle tells whether a function does nothing with a parameter but pass it on.
func (g *passGraph) middle(p *model.Param) bool {
	f := p.Func
	return p.PassedOn && p.Name != "" && !p.Context && !f.Wrapper && !f.Satisfies && !g.escapes[f]
}

// start tells whether a chain of middlemen starts at p: no other middleman
// passes it in.
func (g *passGraph) start(p *model.Param) bool {
	return g.middle(p) && !g.fed[p]
}

// chain returns the longest run of middlemen that starts at p.
func (g *passGraph) chain(p *model.Param, seen map[*model.Param]bool) []*model.Param {
	if c, ok := g.memo[p]; ok {
		return c
	}
	seen[p] = true
	var longest []*model.Param
	for _, h := range g.next[p] {
		if g.middle(h.to) && !seen[h.to] {
			if c := g.chain(h.to, seen); len(c) > len(longest) {
				longest = c
			}
		}
	}
	delete(seen, p)
	g.memo[p] = append([]*model.Param{p}, longest...)
	return g.memo[p]
}

// finding reports a chain, which the given parameters of its first function
// all run through.
func (g *passGraph) finding(chain, params []*model.Param) Finding {
	names := make([]string, len(chain))
	var notes []Note
	for i, p := range chain {
		names[i] = p.Func.String()
		h := g.next[p][0]
		if i+1 < len(chain) {
			for _, candidate := range g.next[p] {
				if candidate.to == chain[i+1] {
					h = candidate
				}
			}
		}
		notes = append(notes, Note{Pos: h.at, Text: fmt.Sprintf("`%s` only passes `%s` on to `%s`", p.Func.Name, p.Name, h.to.Func.Name)})
		if i == len(chain)-1 {
			names = append(names, h.to.Func.String())
		}
	}
	vars := make([]string, len(params))
	for i, p := range params {
		vars[i] = p.Name
	}
	verb, pronoun := "passes", "it"
	if len(params) > 1 {
		verb, pronoun = "pass", "them"
	}
	first := chain[0].Func
	return Finding{
		Lint: PassThroughVariable,
		Key:  first.Key() + "(" + strings.Join(vars, ",") + ")",
		Message: fmt.Sprintf("%s %s through %s that do nothing with %s: `%s`",
			list(vars), verb, plural(len(chain), "function"), pronoun, strings.Join(names, "` → `")),
		Pos:   first.Pos,
		Notes: notes,
	}
}

func inScope(chain []*model.Param) bool {
	for _, p := range chain {
		if p.Func.Thing.Pkg.InScope {
			return true
		}
	}
	return false
}

// paramAt returns the parameter that the i-th argument of a call is passed as.
func paramAt(f *model.Func, i int) *model.Param {
	switch {
	case i < len(f.Params):
		return f.Params[i]
	case f.Variadic:
		return f.Params[len(f.Params)-1]
	}
	return nil
}

// escaping returns the functions taken as values somewhere: they may be
// called in ways no call names.
func escaping(m *model.Model) map[*model.Func]bool {
	escapes := map[*model.Func]bool{}
	for _, r := range m.Refs {
		if r.Func != nil && !r.Called {
			escapes[r.Func] = true
		}
	}
	return escapes
}
