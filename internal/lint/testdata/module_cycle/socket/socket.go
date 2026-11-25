package socket

import "example.com/mc/kinds"

type Socket struct {
	kind    kinds.Kind
	backlog int
}

func Listen(k kinds.Kind) *Socket {
	return &Socket{kind: k, backlog: kinds.Backlog(k)}
}
