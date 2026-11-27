// Package model loads the Go packages of a module and records what their
// code knows: the things declared in them and every reference between them.
//
// A thing is a package-level declaration seen the way go doc sees it: a type
// owns its fields, its methods and the functions that construct it.
package model

import (
	"errors"
	"fmt"
	"go/token"
	"maps"
	"slices"
	"strings"

	"golang.org/x/tools/go/packages"
)

// Model is everything the lints look at.
type Model struct {
	Packages []*Package
	Things   []*Thing
	Refs     []*Ref
	Calls    []*Call
	Literals []*Literal
	Docs     []*Doc
}

// Package is a Go package of one of the loaded modules.
type Package struct {
	Path    string
	Module  string
	Name    string
	Pos     token.Position // the package clause of its first file
	InScope bool           // matched by the patterns the user asked for
}

// Short is the package path relative to its module.
func (p *Package) Short() string {
	if p.Path == p.Module {
		return p.Name
	}
	return strings.TrimPrefix(p.Path, p.Module+"/")
}

// Kind tells what a thing is declared as.
type Kind int

// The kinds of things.
const (
	Type     Kind = iota // a named type, with its methods, fields and constructors
	Function             // a function that constructs no type of its package
	Var                  // a package-level variable
	Const                // a package-level constant
)

// Thing is a package-level declaration with everything it owns.
type Thing struct {
	Name      string
	Pkg       *Package
	Kind      Kind
	Interface bool
	Pos       token.Position
	Funcs     []*Func  // methods and constructors of a type, or the function itself
	Fields    []*Field // fields of a struct type
	Embeds    []*Thing // types of its package that a struct type embeds
}

func (t *Thing) String() string { return t.Pkg.Short() + "." + t.Name }

// Key names the thing uniquely and stably.
func (t *Thing) Key() string { return t.Pkg.Path + "." + t.Name }

// Func is a function, a method, or a method declared by an interface.
type Func struct {
	Name      string
	Thing     *Thing
	Pos       token.Position
	Method    bool
	Abstract  bool // declared by an interface: it has no body
	Satisfies bool // implements a method of some interface, so it may be called through it
	Wrapper   bool // only forwards a call, or gets or sets a field
	Generated bool // declared in a file a tool generated, such as a DeepCopy method
	Variadic  bool
	Params    []*Param

	// PassesTo is set on a method that passes all of its parameters, as they
	// are, to a method of one of its receiver's fields or to a function of
	// another package. It names what it passes them to.
	PassesTo string

	field  *Field // the field a getter or setter wraps
	setter bool
}

func (f *Func) String() string {
	if f.Method {
		return f.Thing.String() + "." + f.Name
	}
	return f.Thing.Pkg.Short() + "." + f.Name
}

// Key names the function uniquely and stably.
func (f *Func) Key() string {
	if f.Method {
		return f.Thing.Key() + "." + f.Name
	}
	return f.Thing.Pkg.Path + "." + f.Name
}

// Param is a parameter of a function. The receiver is not one.
type Param struct {
	Name    string // empty for an unnamed parameter
	Func    *Func
	Index   int
	Context bool // a context.Context: the Go way to carry values through calls

	// PassedOn is set when the function body uses the parameter only to pass
	// it, as it is, to functions of the loaded packages.
	PassedOn bool
}

// Call is a place where code calls a function of the loaded packages by
// its name, and not through a function value.
type Call struct {
	Pos    token.Position
	Pkg    *Package
	Caller *Func // nil outside function bodies
	Callee *Func
	Args   []Arg
}

// Arg is an argument of a call.
type Arg struct {
	Const string // the exact constant value passed, or "nil"; empty if not constant
	Text  string // the argument as written
	Param *Param // the caller's parameter passed as it is, if any
}

// Literal is a string literal written in code, struct tags aside.
type Literal struct {
	Value string
	Pos   token.Position
	Pkg   *Package
}

// Doc is the doc comment of a package-level declaration.
type Doc struct {
	Pos   token.Position
	Pkg   *Package
	Name  string   // the name it documents
	Text  string   // the comment, without comment markers
	Names []string // the identifiers its declaration shows: its name, receiver, parameters and types
}

// Field is a field of a named struct type.
type Field struct {
	Name  string
	Thing *Thing
	Pos   token.Position
}

func (f *Field) String() string { return f.Thing.String() + "." + f.Name }

// Ref is one place where code names a thing, a function or a field.
type Ref struct {
	Pos      token.Position
	Pkg      *Package // where the reference is written
	User     *Thing   // the thing whose code holds the reference
	UserFunc *Func    // the function holding the reference, if any
	Thing    *Thing   // what is referenced
	Func     *Func    // set when a function or method is referenced
	Called   bool     // the function is called here, rather than taken as a value
	Field    *Field   // set when a field is referenced, directly or through a getter or setter
	Copy     bool     // the field is only read, and holds plain data
	Init     bool     // the field is set by a composite literal
}

// Item names what the reference points at.
func (r *Ref) Item() string {
	switch {
	case r.Field != nil && r.Func == nil:
		return r.Field.String()
	case r.Func != nil:
		return r.Func.String()
	}
	return r.Thing.String()
}

// Load loads every package of the modules that the patterns match, so that
// all the callers are known, and marks the packages the patterns match as in
// scope. Test files are left out.
func Load(dir string, patterns []string) (*Model, error) {
	matched, err := packages.Load(&packages.Config{Mode: packages.NeedName | packages.NeedModule, Dir: dir}, patterns...)
	if err != nil {
		return nil, err
	}
	inScope := map[string]bool{}
	modules := map[string]bool{}
	for _, p := range matched {
		if p.Module == nil {
			continue
		}
		inScope[p.PkgPath] = true
		modules[p.Module.Path] = true
	}
	if len(modules) == 0 {
		return nil, fmt.Errorf("no packages of a Go module match %s", strings.Join(patterns, " "))
	}
	var all []string
	for _, m := range slices.Sorted(maps.Keys(modules)) {
		all = append(all, m+"/...")
	}

	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedModule | packages.NeedImports |
			packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo,
		Dir:  dir,
		Fset: token.NewFileSet(),
	}
	pkgs, err := packages.Load(cfg, all...)
	if err != nil {
		return nil, err
	}
	if err := loadErrors(pkgs); err != nil {
		return nil, err
	}
	slices.SortFunc(pkgs, func(a, b *packages.Package) int { return strings.Compare(a.PkgPath, b.PkgPath) })
	return build(cfg.Fset, pkgs, inScope), nil
}

func loadErrors(pkgs []*packages.Package) error {
	var errs []error
	packages.Visit(pkgs, nil, func(p *packages.Package) {
		for _, e := range p.Errors {
			if len(errs) < 10 {
				errs = append(errs, e)
			}
		}
	})
	if len(errs) > 0 {
		return fmt.Errorf("packages do not load cleanly:\n%w", errors.Join(errs...))
	}
	return nil
}
