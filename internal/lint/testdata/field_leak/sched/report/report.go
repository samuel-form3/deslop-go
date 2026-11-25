package report

import "example.com/fl/queue"

func Backlog(q *queue.RunQueue) int {
	return len(q.Items) - q.Head
}

func Drained(q *queue.RunQueue) bool {
	return q.Head == len(q.Items)
}

// Title only reads a copy of plain data: Label is not leaked, nor kept.
func Title(q *queue.RunQueue) string {
	return q.Label
}

func Peek(q *queue.RunQueue) int {
	return q.Buffer()[0]
}
