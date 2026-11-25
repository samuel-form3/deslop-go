package box

type Box struct {
	N    int
	Xs   []int
	Pos  struct{ X, Y int }
	name string
}

func New() *Box { return &Box{name: "box"} }

func (b *Box) Name() string        { return b.name }
func (b *Box) Values() []int       { return b.Xs }
func (b *Box) SetName(name string) { b.name = name }
func (b *Box) Close() error        { return b.close() }
func (b *Box) close() error        { return nil }
func (b *Box) Error() string       { return b.name + "!" }
