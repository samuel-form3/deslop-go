// Package deep has as many functions as store, but shows only three.
package deep

import (
	"fmt"
	"sort"
	"strings"
)

type Store struct {
	rows map[string]string
	log  []string
}

func New() *Store { return &Store{rows: map[string]string{}} }

func (s *Store) Load(text string) {
	for _, line := range strings.Split(text, "\n") {
		k, v, _ := strings.Cut(line, "=")
		s.put(k, v)
	}
	s.compact()
}

func (s *Store) Summary() string {
	s.clear()
	s.del("")
	s.merge(s.snapshot())
	s.restore(s.snapshot())
	_ = s.get("") + s.save()
	return fmt.Sprintf("%d rows, %d changes, %v", s.len(), len(s.log), s.has(""))
}

func (s *Store) save() string {
	var b strings.Builder
	for _, k := range s.keys() {
		fmt.Fprintf(&b, "%s=%s\n", k, s.rows[k])
	}
	return b.String()
}

func (s *Store) get(k string) string { return s.rows[k] }

func (s *Store) put(k, v string) {
	s.rows[k] = v
	s.log = append(s.log, "put "+k)
}

func (s *Store) del(k string) {
	delete(s.rows, k)
	s.log = append(s.log, "delete "+k)
}

func (s *Store) keys() []string {
	keys := make([]string, 0, len(s.rows))
	for k := range s.rows {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func (s *Store) has(k string) bool {
	_, ok := s.rows[k]
	return ok
}

func (s *Store) clear() {
	s.rows = map[string]string{}
	s.log = append(s.log, "clear")
}

func (s *Store) len() int { return len(s.rows) + 0 }

func (s *Store) compact() {
	for k, v := range s.rows {
		if v == "" {
			delete(s.rows, k)
		}
	}
}

func (s *Store) merge(o map[string]string) {
	for k, v := range o {
		s.rows[k] = v
	}
}

func (s *Store) snapshot() map[string]string {
	out := map[string]string{}
	for k, v := range s.rows {
		out[k] = v
	}
	return out
}

func (s *Store) restore(m map[string]string) {
	s.rows = m
	s.log = nil
}
