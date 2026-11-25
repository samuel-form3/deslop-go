package model

import (
	"go/ast"
	"go/token"
	"strconv"
)

// doc records the doc comment of a declaration, with the identifiers that
// its declaration shows a reader: the name, and those in the given nodes.
func (b *builder) doc(pkg *Package, name *ast.Ident, comment *ast.CommentGroup, shown ...ast.Node) {
	if comment == nil || name.Name == "_" {
		return
	}
	d := &Doc{Pos: b.fset.Position(comment.Pos()), Pkg: pkg, Name: name.Name, Text: comment.Text(), Names: []string{name.Name}}
	for _, n := range shown {
		if n == nil {
			continue
		}
		ast.Inspect(n, func(n ast.Node) bool {
			if id, ok := n.(*ast.Ident); ok {
				d.Names = append(d.Names, id.Name)
			}
			return true
		})
	}
	b.m.Docs = append(b.m.Docs, d)
}

// specDoc records the doc comment of a type, variable or constant. A lone
// spec in a declaration is documented by the comment on the declaration.
func (b *builder) specDoc(pkg *Package, d *ast.GenDecl, spec ast.Spec) {
	switch s := spec.(type) {
	case *ast.TypeSpec:
		b.doc(pkg, s.Name, specComment(d, s.Doc))
	case *ast.ValueSpec:
		if len(s.Names) == 1 {
			b.doc(pkg, s.Names[0], specComment(d, s.Doc), s.Type) // a nil Type converts to a nil Node
		}
	}
}

func specComment(d *ast.GenDecl, own *ast.CommentGroup) *ast.CommentGroup {
	if own == nil && len(d.Specs) == 1 {
		return d.Doc
	}
	return own
}

// literal records a string literal, unless it is a struct tag.
func (b *builder) literal(pkg *Package, lit *ast.BasicLit, parent ast.Node) {
	if lit.Kind != token.STRING {
		return
	}
	if _, tag := parent.(*ast.Field); tag {
		return
	}
	value, err := strconv.Unquote(lit.Value)
	if err != nil {
		return
	}
	b.m.Literals = append(b.m.Literals, &Literal{Value: value, Pos: b.fset.Position(lit.Pos()), Pkg: pkg})
}
