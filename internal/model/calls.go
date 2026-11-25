package model

import (
	"go/ast"
	"go/types"
)

// callee returns the function of the loaded packages that a call names, or
// nil when it calls something else, or calls through a function value or a
// method expression.
func (b *builder) callee(info *types.Info, call *ast.CallExpr) *Func {
	fun := ast.Unparen(call.Fun)
	switch f := fun.(type) {
	case *ast.IndexExpr: // instantiation of a generic function
		fun = f.X
	case *ast.IndexListExpr:
		fun = f.X
	}
	var id *ast.Ident
	switch f := fun.(type) {
	case *ast.Ident:
		id = f
	case *ast.SelectorExpr:
		if sel := info.Selections[f]; sel != nil && sel.Kind() == types.MethodExpr {
			return nil // its arguments start with the receiver
		}
		id = f.Sel
	default:
		return nil
	}
	fn, _ := info.Uses[id].(*types.Func)
	if fn == nil {
		return nil
	}
	return b.funcs[fn.Origin()]
}

// call records the arguments of a call to a function of the loaded packages.
func (b *builder) call(info *types.Info, call *ast.CallExpr) *Call {
	callee := b.callee(info, call)
	if callee == nil {
		return nil
	}
	c := &Call{Callee: callee, Args: make([]Arg, len(call.Args))}
	for i, arg := range call.Args {
		a := &c.Args[i]
		a.Text = types.ExprString(arg)
		if tv, ok := info.Types[arg]; ok {
			switch {
			case tv.Value != nil:
				a.Const = tv.Value.ExactString()
			case tv.IsNil():
				a.Const = "nil"
			}
		}
		if id, ok := ast.Unparen(arg).(*ast.Ident); ok {
			if v, ok := info.Uses[id].(*types.Var); ok {
				a.Param = b.params[v]
			}
		}
	}
	return c
}

// paramUse counts how the identifier on top of the stack uses a parameter of
// the function it is written in: passed on as it is to a function of the
// loaded packages, or used in any other way.
func (b *builder) paramUse(info *types.Info, stack []ast.Node) {
	id := stack[len(stack)-1].(*ast.Ident)
	v, ok := info.Uses[id].(*types.Var)
	if !ok {
		return
	}
	p := b.params[v]
	if p == nil {
		return
	}
	if call, ok := parent(stack, 1).(*ast.CallExpr); ok && call.Fun != id && b.callee(info, call) != nil {
		b.passed[p]++
		return
	}
	b.used[p]++
}
