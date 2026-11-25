// Package ring8 holds a circle of eight types: one more than fits in mind.
package ring8

type A struct{ next *B }
type B struct{ next *C }
type C struct{ next *D }
type D struct{ next *E }
type E struct{ next *F }
type F struct{ next *G }
type G struct{ next *H }
type H struct{ next *A }
