package user

import "example.com/refs/box"

func Read(b *box.Box) int     { return b.N + b.Pos.X }
func Write(b *box.Box)        { b.N++ }
func Address(b *box.Box) *int { return &b.N }
func Index(b *box.Box) int    { return b.Xs[0] }
func Getter(b *box.Box) int   { return len(b.Values()) + len(b.Name()) }
func Setter(b *box.Box)       { b.SetName("x") }
func Literal() *box.Box       { return &box.Box{N: 1} }
