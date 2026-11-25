// Package store shows 9 of its 15 functions: the app has to know most of it.
package store

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
		s.rows[k] = v
	}
}

func (s *Store) Save() string {
	var b strings.Builder
	for _, k := range s.Keys() {
		fmt.Fprintf(&b, "%s=%s\n", k, s.rows[k])
	}
	return b.String()
}

func (s *Store) Get(k string) string { return s.rows[k] }

func (s *Store) Put(k, v string) {
	s.rows[k] = v
	s.log = append(s.log, "put "+k)
}

func (s *Store) Delete(k string) {
	delete(s.rows, k)
	s.log = append(s.log, "delete "+k)
}

func (s *Store) Keys() []string {
	keys := make([]string, 0, len(s.rows))
	for k := range s.rows {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func (s *Store) Has(k string) bool {
	_, ok := s.rows[k]
	return ok
}

func (s *Store) Clear() {
	s.rows = map[string]string{}
	s.log = append(s.log, "clear")
}

func (s *Store) Len() int { return len(s.rows) + 0 }

func (s *Store) Compact() {
	for k, v := range s.rows {
		if v == "" {
			delete(s.rows, k)
		}
	}
}

func (s *Store) Merge(o *Store) {
	for k, v := range o.rows {
		s.rows[k] = v
	}
}

func (s *Store) Snapshot() map[string]string {
	out := map[string]string{}
	for k, v := range s.rows {
		out[k] = v
	}
	return out
}

func (s *Store) Restore(m map[string]string) {
	s.rows = m
	s.log = nil
}

func (s *Store) Summary() string {
	return fmt.Sprintf("%d rows, %d changes", len(s.rows), len(s.log))
}
