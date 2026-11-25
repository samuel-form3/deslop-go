package model

import (
	"go/ast"
	"go/token"
	"go/types"
)

// markWrappers finds the functions whose body is one statement that gets or
// sets a field of their receiver, or passes their parameters on to another
// call. Their callers know what they wrap.
func (b *builder) markWrappers(info *types.Info, file *ast.File) {
	for _, decl := range file.Decls {
		d, ok := decl.(*ast.FuncDecl)
		if !ok || d.Body == nil || len(d.Body.List) != 1 {
			continue
		}
		fn, _ := info.Defs[d.Name].(*types.Func)
		f := b.funcs[fn]
		if f == nil {
			continue
		}
		params := map[types.Object]bool{}
		var recv types.Object
		if d.Recv != nil {
			for _, field := range d.Recv.List {
				for _, name := range field.Names {
					recv = info.Defs[name]
					params[recv] = true
				}
			}
		}
		for _, field := range d.Type.Params.List {
			for _, name := range field.Names {
				params[info.Defs[name]] = true
			}
		}

		switch s := d.Body.List[0].(type) {
		case *ast.ReturnStmt:
			if len(s.Results) != 1 {
				continue
			}
			result := ast.Unparen(s.Results[0])
			if u, ok := result.(*ast.UnaryExpr); ok && u.Op == token.AND {
				result = ast.Unparen(u.X)
			}
			if field := b.receiverField(info, result, recv); field != nil {
				f.Wrapper, f.field = true, field
			} else if forwards(info, result, params) {
				f.Wrapper = true
				f.PassesTo = passesTo(info, d, result, recv)
			}
		case *ast.ExprStmt:
			if forwards(info, s.X, params) {
				f.Wrapper = true
				f.PassesTo = passesTo(info, d, s.X, recv)
			}
		case *ast.AssignStmt:
			if s.Tok != token.ASSIGN || len(s.Lhs) != 1 || len(s.Rhs) != 1 || !isParam(info, s.Rhs[0], params) {
				continue
			}
			if field := b.receiverField(info, s.Lhs[0], recv); field != nil {
				f.Wrapper, f.field, f.setter = true, field, true
			}
		}
	}
}

// receiverField returns the field that e selects directly from the receiver.
func (b *builder) receiverField(info *types.Info, e ast.Expr, recv types.Object) *Field {
	sel, ok := e.(*ast.SelectorExpr)
	if !ok || recv == nil {
		return nil
	}
	if x, ok := ast.Unparen(sel.X).(*ast.Ident); !ok || info.Uses[x] != recv {
		return nil
	}
	v, ok := info.Uses[sel.Sel].(*types.Var)
	if !ok || !v.IsField() {
		return nil
	}
	return b.fields[v.Origin()]
}

// forwards tells whether e calls a function with nothing but the parameters.
func forwards(info *types.Info, e ast.Expr, params map[types.Object]bool) bool {
	call, ok := ast.Unparen(e).(*ast.CallExpr)
	if !ok {
		return false
	}
	if tv, ok := info.Types[call.Fun]; ok && (tv.IsType() || tv.IsBuiltin()) {
		return false
	}
	for _, arg := range call.Args {
		if !isParam(info, arg, params) {
			return false
		}
	}
	return true
}

// passesTo names what a forwarding method passes its parameters to, when it
// passes all of them, in order, to a method of one of its receiver's fields
// or to a function of another package: the same call, one layer down. It is
// empty for anything else, such as a call to the receiver's own methods.
func passesTo(info *types.Info, d *ast.FuncDecl, e ast.Expr, recv types.Object) string {
	call := ast.Unparen(e).(*ast.CallExpr)
	if recv == nil {
		return ""
	}
	var params []types.Object
	for _, field := range d.Type.Params.List {
		if len(field.Names) == 0 {
			return "" // unnamed parameters cannot be passed on
		}
		for _, name := range field.Names {
			params = append(params, info.Defs[name])
		}
	}
	if len(call.Args) != len(params) {
		return ""
	}
	for i, arg := range call.Args {
		if id, ok := ast.Unparen(arg).(*ast.Ident); !ok || info.Uses[id] != params[i] {
			return ""
		}
	}

	sel, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr)
	if !ok {
		return "" // a function of its own package
	}
	switch x := ast.Unparen(sel.X).(type) {
	case *ast.Ident:
		if _, ok := info.Uses[x].(*types.PkgName); ok {
			return types.ExprString(sel)
		}
	case *ast.SelectorExpr:
		root := x
		for {
			inner, ok := ast.Unparen(root.X).(*ast.SelectorExpr)
			if !ok {
				break
			}
			root = inner
		}
		if id, ok := ast.Unparen(root.X).(*ast.Ident); ok && info.Uses[id] == recv {
			return types.ExprString(sel)
		}
	}
	return ""
}

func isParam(info *types.Info, e ast.Expr, params map[types.Object]bool) bool {
	id, ok := ast.Unparen(e).(*ast.Ident)
	return ok && params[info.Uses[id]]
}
