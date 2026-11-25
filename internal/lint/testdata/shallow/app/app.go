package app

import (
	"example.com/sh/deep"
	"example.com/sh/stats"
	"example.com/sh/store"
)

func Run(text string) string {
	s := store.New()
	s.Load(text)
	s.Put("a", "1")
	if s.Has("b") {
		s.Delete("b")
	}
	_ = s.Get("a")
	_ = s.Keys()
	s.Clear()
	return s.Save()
}

func RunDeep(text string) string {
	d := deep.New()
	d.Load(text)
	return d.Summary()
}

func Measure() float64 {
	var r stats.Rate
	var t stats.Tally
	var w stats.Window
	r.Add(1, 2)
	t.Add("x")
	w.Add(3)
	defer r.Reset()
	defer t.Reset()
	defer w.Reset()
	return r.Value() + float64(t.Count("x")) + w.Mean()
}
