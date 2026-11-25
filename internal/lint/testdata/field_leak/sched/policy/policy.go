package policy

import "example.com/fl/queue"

func Admit(q *queue.RunQueue, x int) {
	q.Items = append(q.Items, x)
}

func Next(q *queue.RunQueue) int {
	x := q.Items[q.Head]
	q.Head++
	return x
}

func Configure() *queue.Options {
	return &queue.Options{Sizes: []int{8, 64}}
}

func Record(s *queue.Stats, x int) {
	s.Pushed++
	s.Dropped = append(s.Dropped, x)
}
