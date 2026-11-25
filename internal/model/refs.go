package model

import (
	"go/ast"
	"go/token"
	"go/types"
)

// walk records what the code of a file references, calls and writes, each
// held by the thing whose declaration it is written in.
func (b *builder) walk(info *types.Info, pkg *Package, file *ast.File) {
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if f := b.funcOf(info, d, pkg); f != nil {
				shown := []ast.Node{d.Type}
				if d.Recv != nil {
					shown = append(shown, d.Recv)
				}
				b.doc(pkg, d.Name, d.Doc, shown...)
				b.inspect(info, pkg, d, f.Thing, f)
			}
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				b.specDoc(pkg, d, spec)
				if user := b.specThing(info, spec); user != nil {
					b.inspect(info, pkg, spec, user, nil)
				}
			}
		}
	}
}

func (b *builder) specThing(info *types.Info, spec ast.Spec) *Thing {
	switch s := spec.(type) {
	case *ast.TypeSpec:
		return b.things[info.Defs[s.Name]]
	case *ast.ValueSpec:
		for _, name := range s.Names {
			if t := b.things[info.Defs[name]]; t != nil {
				return t
			}
		}
	}
	return nil
}

func (b *builder) inspect(info *types.Info, pkg *Package, root ast.Node, user *Thing, userFunc *Func) {
	var stack []ast.Node
	ast.Inspect(root, func(n ast.Node) bool {
		if n == nil {
			stack = stack[:len(stack)-1]
			return true
		}
		stack = append(stack, n)
		switch n := n.(type) {
		case *ast.Ident:
			if r := b.ref(info, stack); r != nil {
				r.Pos, r.Pkg, r.User, r.UserFunc = b.fset.Position(n.Pos()), pkg, user, userFunc
				b.m.Refs = append(b.m.Refs, r)
			}
			b.paramUse(info, stack)
		case *ast.CompositeLit:
			for _, field := range b.unkeyed(info, n) {
				r := &Ref{Pos: b.fset.Position(n.Pos()), Pkg: pkg, User: user, UserFunc: userFunc, Thing: field.Thing, Field: field, Init: true}
				b.m.Refs = append(b.m.Refs, r)
			}
		case *ast.CallExpr:
			if c := b.call(info, n); c != nil {
				c.Pos, c.Pkg, c.Caller = b.fset.Position(n.Pos()), pkg, userFunc
				b.m.Calls = append(b.m.Calls, c)
			}
		case *ast.BasicLit:
			b.literal(pkg, n, parent(stack, 1))
		}
		return true
	})
}

// ref resolves the identifier on top of the stack to what it references, if
// that is declared in one of the loaded packages.
func (b *builder) ref(info *types.Info, stack []ast.Node) *Ref {
	id := stack[len(stack)-1].(*ast.Ident)
	switch obj := info.Uses[id].(type) {
	case *types.Func:
		f := b.funcs[obj.Origin()]
		if f == nil {
			return nil
		}
		r := &Ref{Thing: f.Thing, Func: f}
		call := calledAt(stack)
		r.Called = call >= 0
		if f.field != nil {
			r.Field = f.field
			// A getter's caller only copies the field when it reads plain data.
			r.Copy = !f.setter && r.Called && plain(info.TypeOf(stack[call].(ast.Expr))) && readOnly(info, stack[:call+1])
		}
		return r
	case *types.Var:
		if !obj.IsField() {
			if t := b.things[obj]; t != nil {
				return &Ref{Thing: t}
			}
			return nil
		}
		field := b.fields[obj.Origin()]
		if field == nil {
			return nil
		}
		r := &Ref{Thing: field.Thing, Field: field}
		switch p := parent(stack, 1).(type) {
		case *ast.SelectorExpr:
			r.Copy = p.Sel == id && plain(obj.Type()) && readOnly(info, stack[:len(stack)-1])
		case *ast.KeyValueExpr:
			_, inLiteral := parent(stack, 2).(*ast.CompositeLit)
			r.Init = p.Key == id && inLiteral
		}
		return r
	case *types.TypeName, *types.Const:
		if t := b.things[obj]; t != nil {
			return &Ref{Thing: t}
		}
	}
	return nil
}

// unkeyed returns the fields that a struct literal without keys sets.
func (b *builder) unkeyed(info *types.Info, lit *ast.CompositeLit) []*Field {
	if len(lit.Elts) == 0 {
		return nil
	}
	if _, keyed := lit.Elts[0].(*ast.KeyValueExpr); keyed {
		return nil
	}
	t := info.TypeOf(lit)
	if t == nil {
		return nil
	}
	if p, ok := t.Underlying().(*types.Pointer); ok {
		t = p.Elem()
	}
	st, ok := t.Underlying().(*types.Struct)
	if !ok {
		return nil
	}
	var fields []*Field
	for v := range st.Fields() {
		if f := b.fields[v.Origin()]; f != nil {
			fields = append(fields, f)
		}
	}
	return fields
}

// calledAt returns where in the stack is the call of the function named on
// top of it, or -1 when the function is not called there.
func calledAt(stack []ast.Node) int {
	e := stack[len(stack)-1]
	for i := len(stack) - 2; i >= 0; i-- {
		switch p := stack[i].(type) {
		case *ast.ParenExpr:
		case *ast.SelectorExpr:
			if p.Sel != e {
				return -1
			}
		case *ast.IndexExpr: // instantiation of a generic function
			if p.X != e {
				return -1
			}
		case *ast.IndexListExpr:
			if p.X != e {
				return -1
			}
		case *ast.CallExpr:
			if p.Fun != e {
				return -1
			}
			return i
		default:
			return -1
		}
		e = stack[i]
	}
	return -1
}

func parent(stack []ast.Node, up int) ast.Node {
	if len(stack) <= up {
		return nil
	}
	return stack[len(stack)-1-up]
}

// readOnly tells whether the expression on top of the stack is only read:
// not written, not taken the address of, not ranged into, and not the
// receiver of a method that may change it.
func readOnly(info *types.Info, stack []ast.Node) bool {
	e := stack[len(stack)-1]
	for i := len(stack) - 2; i >= 0; i-- {
		switch p := stack[i].(type) {
		case *ast.ParenExpr:
		case *ast.SelectorExpr:
			sel := info.Selections[p]
			if sel == nil || p.X != e {
				return true
			}
			if sel.Kind() != types.FieldVal {
				_, ptr := sel.Obj().(*types.Func).Signature().Recv().Type().(*types.Pointer)
				return !ptr
			}
		case *ast.IndexExpr:
			if p.X != e {
				return true
			}
			if _, array := info.TypeOf(p.X).Underlying().(*types.Array); !array {
				return true
			}
		case *ast.UnaryExpr:
			return p.Op != token.AND
		case *ast.AssignStmt:
			for _, lhs := range p.Lhs {
				if lhs == e {
					return false
				}
			}
			return true
		case *ast.IncDecStmt:
			return false
		case *ast.RangeStmt:
			return p.Key != e && p.Value != e
		default:
			return true
		}
		e = stack[i]
	}
	return true
}

// plain tells whether values of a type are plain data: copying one shares
// nothing with the original.
func plain(t types.Type) bool {
	if t == nil {
		return false
	}
	switch u := t.Underlying().(type) {
	case *types.Basic:
		return u.Kind() != types.UnsafePointer
	case *types.Array:
		return plain(u.Elem())
	case *types.Struct:
		for v := range u.Fields() {
			if !plain(v.Type()) {
				return false
			}
		}
		return true
	}
	return false
}
