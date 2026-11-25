package queue

type RunQueue struct {
	Items []int
	Head  int
	Label string
	buf   []int
	limit int
}

// New is a constructor: it is part of RunQueue, so setting fields here is no leak.
func New(label string, limit int) *RunQueue {
	return &RunQueue{Label: label, limit: limit}
}

func (q *RunQueue) Push(x int) bool {
	if len(q.Items)-q.Head >= q.limit {
		return false
	}
	q.Items = append(q.Items, x)
	return true
}

// Buffer is a getter: its callers know buf.
func (q *RunQueue) Buffer() []int { return q.buf }

// Bounded embeds RunQueue in the same package: it works on the fields of
// RunQueue as RunQueue itself would.
type Bounded struct {
	RunQueue
	max int
}

func (b *Bounded) Trim() { b.Items = b.Items[:b.max] }

// Options are set by callers in a composite literal: that is how Go
// configures, not a leak.
type Options struct {
	Sizes []int
	retry int
}

func (o *Options) Valid() bool { return o.retry >= 0 && len(o.Sizes) > 0 }

// Stats is plain data: all of its fields are known outside, so nothing is kept
// and there is nothing to guard.
type Stats struct {
	Pushed  int
	Dropped []int
}

func (s *Stats) Total() int { return s.Pushed + len(s.Dropped) }
