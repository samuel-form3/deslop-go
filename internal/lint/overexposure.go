package lint

import (
	"fmt"
	"strings"

	"github.com/samuel-form3/deslop-go/internal/model"
)

// sameCalls is how many calls must pass a parameter the same value for it to
// be a decision the function should make itself.
const sameCalls = 3

// overexposure finds parameters that every caller sets to the same constant.
// Each caller has to know about a choice that nobody makes: the function
// should take it, and pull that complexity down.
//
// Functions whose signature is not theirs to choose are left out: those that
// implement an interface, and those taken as values.
func overexposure(m *model.Model) []Finding {
	calls := map[*model.Func][]*model.Call{}
	for _, c := range m.Calls {
		calls[c.Callee] = append(calls[c.Callee], c)
	}
	escapes := escaping(m)

	var found []Finding
	for _, t := range m.Things {
		for _, f := range t.Funcs {
			cs := calls[f]
			if !t.Pkg.InScope || f.Abstract || f.Satisfies || escapes[f] || len(cs) < sameCalls {
				continue
			}
			var fixed, names []string
			for i, p := range f.Params {
				if f.Variadic && i == len(f.Params)-1 {
					continue
				}
				if value, ok := constant(cs, i); ok {
					fixed = append(fixed, fmt.Sprintf("`%s` is always `%s`", paramName(p), value))
					names = append(names, paramName(p))
				}
			}
			if len(fixed) == 0 {
				continue
			}
			var notes []Note
			for _, c := range cs[:min(len(cs), heldInMind)] {
				notes = append(notes, Note{Pos: c.Pos, Text: "called here"})
			}
			found = append(found, Finding{
				Lint:    Overexposure,
				Key:     f.Key() + "(" + strings.Join(names, ",") + ")",
				Message: fmt.Sprintf("all %d callers of `%s` pass the same: %s", len(cs), f, strings.Join(fixed, ", ")),
				Pos:     f.Pos,
				Notes:   notes,
			})
		}
	}
	return found
}

// constant returns the constant that every call passes as its i-th argument,
// as written in the first call.
func constant(calls []*model.Call, i int) (string, bool) {
	for _, c := range calls {
		if i >= len(c.Args) || c.Args[i].Const == "" || c.Args[i].Const != calls[0].Args[i].Const {
			return "", false
		}
	}
	return calls[0].Args[i].Text, true
}

func paramName(p *model.Param) string {
	if p.Name == "" {
		return fmt.Sprintf("#%d", p.Index+1)
	}
	return p.Name
}
