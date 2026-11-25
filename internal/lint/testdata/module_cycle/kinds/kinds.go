package kinds

import "example.com/mc/socket/limits"

type Kind int

const (
	Stream Kind = iota
	Datagram
)

// Backlog reaches down into socket/limits, while socket uses kinds.
func Backlog(k Kind) int {
	if k == Stream {
		return limits.BacklogMost
	}
	return 0
}
