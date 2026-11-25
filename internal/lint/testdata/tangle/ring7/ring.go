// Package ring7 holds a circle of seven types: it still fits in mind.
package ring7

type A struct{ next *B }
type B struct{ next *C }
type C struct{ next *D }
type D struct{ next *E }
type E struct{ next *F }
type F struct{ next *G }
type G struct{ next *A }
