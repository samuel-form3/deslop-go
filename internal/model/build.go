package model

import (
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/packages"
)

type builder struct {
	fset   *token.FileSet
	m      *Model
	things map[types.Object]*Thing // type names, package-level vars and consts, free functions
	funcs  map[*types.Func]*Func
	fields map[*types.Var]*Field
	params map[*types.Var]*Param
	types  map[*Thing]*types.Named

	// How each parameter is used in its function's body.
	passed map[*Param]int // passed as it is to a function of the loaded packages
	used   map[*Param]int // any other use
}

func build(fset *token.FileSet, pkgs []*packages.Package, inScope map[string]bool) *Model {
	b := &builder{
		fset:   fset,
		m:      &Model{},
		things: map[types.Object]*Thing{},
		funcs:  map[*types.Func]*Func{},
		fields: map[*types.Var]*Field{},
		params: map[*types.Var]*Param{},
		types:  map[*Thing]*types.Named{},
		passed: map[*Param]int{},
		used:   map[*Param]int{},
	}
	own := make([]*Package, len(pkgs))
	for i, p := range pkgs {
		own[i] = &Package{Path: p.PkgPath, Module: p.Module.Path, Name: p.Name, InScope: inScope[p.PkgPath]}
		if len(p.Syntax) > 0 {
			own[i].Pos = fset.Position(p.Syntax[0].Package)
		}
		b.m.Packages = append(b.m.Packages, own[i])
	}
	for i, p := range pkgs {
		b.declare(p, own[i])
	}
	b.linkEmbedded()
	b.markSatisfied(pkgs)
	// Every wrapper must be known before any call to one is walked.
	for _, p := range pkgs {
		for _, file := range p.Syntax {
			b.markWrappers(p.TypesInfo, file)
		}
	}
	for i, p := range pkgs {
		for _, file := range p.Syntax {
			b.walk(p.TypesInfo, own[i], file)
		}
	}
	for p, n := range b.passed {
		p.PassedOn = n > 0 && b.used[p] == 0
	}
	return b.m
}

func (b *builder) newThing(obj types.Object, pkg *Package, kind Kind) *Thing {
	t := &Thing{Name: obj.Name(), Pkg: pkg, Kind: kind, Pos: b.fset.Position(obj.Pos())}
	b.m.Things = append(b.m.Things, t)
	b.things[obj] = t
	return t
}

func (b *builder) newFunc(fn *types.Func, owner *Thing) *Func {
	sig := fn.Signature()
	f := &Func{
		Name:     fn.Name(),
		Thing:    owner,
		Pos:      b.fset.Position(fn.Pos()),
		Method:   sig.Recv() != nil,
		Variadic: sig.Variadic(),
	}
	for i := range sig.Params().Len() {
		v := sig.Params().At(i)
		p := &Param{Name: v.Name(), Func: f, Index: i, Context: isContext(v.Type())}
		if p.Name == "_" {
			p.Name = ""
		}
		f.Params = append(f.Params, p)
		b.params[v] = p
	}
	owner.Funcs = append(owner.Funcs, f)
	b.funcs[fn] = f
	return f
}

func isContext(t types.Type) bool {
	named, ok := types.Unalias(t).(*types.Named)
	if !ok {
		return false
	}
	obj := named.Obj()
	return obj.Pkg() != nil && obj.Pkg().Path() == "context" && obj.Name() == "Context"
}

// declare records the package-level declarations of a package: types first,
// so that constructors can join the type they construct.
func (b *builder) declare(p *packages.Package, pkg *Package) {
	scope := p.Types.Scope()
	for _, name := range scope.Names() {
		tn, ok := scope.Lookup(name).(*types.TypeName)
		if !ok || tn.IsAlias() {
			continue
		}
		named, ok := tn.Type().(*types.Named)
		if !ok {
			continue
		}
		t := b.newThing(tn, pkg, Type)
		b.types[t] = named
		for m := range named.Methods() {
			b.newFunc(m, t)
		}
		switch u := named.Underlying().(type) {
		case *types.Interface:
			t.Interface = true
			for m := range u.ExplicitMethods() {
				b.newFunc(m, t).Abstract = true
			}
		case *types.Struct:
			for v := range u.Fields() {
				f := &Field{Name: v.Name(), Thing: t, Pos: b.fset.Position(v.Pos())}
				t.Fields = append(t.Fields, f)
				b.fields[v] = f
			}
		}
	}
	for _, name := range scope.Names() {
		switch obj := scope.Lookup(name).(type) {
		case *types.Func:
			if owner := b.constructed(obj, p.Types); owner != nil {
				b.newFunc(obj, owner)
				b.things[obj] = owner
				continue
			}
			b.newFunc(obj, b.newThing(obj, pkg, Function))
		case *types.Var:
			b.newThing(obj, pkg, Var)
		case *types.Const:
			b.newThing(obj, pkg, Const)
		}
	}
}

// constructed returns the type a function constructs, as go doc decides it:
// the one type of its own package that its results are, or point to.
func (b *builder) constructed(fn *types.Func, pkg *types.Package) *Thing {
	var owner *Thing
	for result := range fn.Signature().Results().Variables() {
		t := result.Type()
		if p, ok := t.(*types.Pointer); ok {
			t = p.Elem()
		}
		named, ok := t.(*types.Named)
		if !ok || named.Obj().Pkg() != pkg {
			continue
		}
		thing := b.things[named.Origin().Obj()]
		if thing == nil || (owner != nil && owner != thing) {
			return nil
		}
		owner = thing
	}
	return owner
}

// linkEmbedded records the types of its own package that each struct type
// embeds, once every type is declared.
func (b *builder) linkEmbedded() {
	for t, named := range b.types {
		st, ok := named.Underlying().(*types.Struct)
		if !ok {
			continue
		}
		for v := range st.Fields() {
			if !v.Embedded() {
				continue
			}
			typ := v.Type()
			if p, ok := typ.(*types.Pointer); ok {
				typ = p.Elem()
			}
			if inner, ok := typ.(*types.Named); ok {
				if e := b.things[inner.Origin().Obj()]; e != nil && e.Pkg == t.Pkg {
					t.Embeds = append(t.Embeds, e)
				}
			}
		}
	}
}

// markSatisfied marks the methods that implement a method of an interface
// declared in the modules or in a package they import. Such methods can be
// called through the interface, where no reference names them.
func (b *builder) markSatisfied(pkgs []*packages.Package) {
	byMethod := interfacesByMethod(pkgs)
	for t, named := range b.types {
		if t.Interface || named.TypeParams().Len() > 0 {
			continue
		}
		ptr := types.NewPointer(named)
		checked := map[*types.Interface]bool{}
		for _, f := range t.Funcs {
			if !f.Method {
				continue
			}
			for _, iface := range byMethod[f.Name] {
				if checked[iface] {
					continue
				}
				checked[iface] = true
				if types.Implements(ptr, iface) {
					markMethods(t, iface)
				}
			}
		}
	}
}

// markMethods marks the methods of t that iface declares.
func markMethods(t *Thing, iface *types.Interface) {
	for m := range iface.Methods() {
		for _, f := range t.Funcs {
			if f.Method && f.Name == m.Name() {
				f.Satisfies = true
			}
		}
	}
}

// interfacesByMethod indexes, by method name, the interfaces that the
// packages declare or can see exported by their imports.
func interfacesByMethod(pkgs []*packages.Package) map[string][]*types.Interface {
	byMethod := map[string][]*types.Interface{}
	seen := map[*types.Interface]bool{}
	add := func(t types.Type) {
		iface, ok := t.Underlying().(*types.Interface)
		if !ok || seen[iface] || iface.NumMethods() == 0 || !iface.IsMethodSet() {
			return
		}
		seen[iface] = true
		for m := range iface.Methods() {
			byMethod[m.Name()] = append(byMethod[m.Name()], iface)
		}
	}
	addScope := func(pkg *types.Package, exportedOnly bool) {
		scope := pkg.Scope()
		for _, name := range scope.Names() {
			if tn, ok := scope.Lookup(name).(*types.TypeName); ok && (!exportedOnly || tn.Exported()) {
				add(tn.Type())
			}
		}
	}

	add(types.Universe.Lookup("error").Type())
	for _, p := range pkgs {
		addScope(p.Types, false)
		for _, imported := range p.Types.Imports() {
			addScope(imported, true)
		}
	}
	return byMethod
}

// funcOf returns the function a declaration declares, recording the ones
// that no scope holds, such as init.
func (b *builder) funcOf(info *types.Info, d *ast.FuncDecl, pkg *Package) *Func {
	fn, _ := info.Defs[d.Name].(*types.Func)
	if fn == nil {
		return nil
	}
	if f := b.funcs[fn]; f != nil {
		return f
	}
	return b.newFunc(fn, b.newThing(fn, pkg, Function))
}
