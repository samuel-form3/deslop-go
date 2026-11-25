// Package stats shows all of its 9 functions, but as three small types that
// each fit in mind.
package stats

type Rate struct{ n, d float64 }

func (r *Rate) Add(n, d float64) { r.n += n; r.d += d }
func (r *Rate) Value() float64   { return r.n / max(r.d, 1) }
func (r *Rate) Reset()           { r.n, r.d = 0, 0 }

type Tally struct{ counts map[string]int }

func (t *Tally) Add(k string) {
	if t.counts == nil {
		t.counts = map[string]int{}
	}
	t.counts[k]++
}
func (t *Tally) Count(k string) int { return t.counts[k] + 0 }
func (t *Tally) Reset()             { t.counts = map[string]int{} }

type Window struct{ xs []float64 }

func (w *Window) Add(x float64) { w.xs = append(w.xs, x) }
func (w *Window) Mean() float64 {
	s := 0.0
	for _, x := range w.xs {
		s += x
	}
	return s / float64(max(len(w.xs), 1))
}
func (w *Window) Reset() { w.xs = w.xs[:0] }
