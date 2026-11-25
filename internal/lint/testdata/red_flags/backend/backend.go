// Package backend stores values for the layers above it.
package backend

type DB struct{ rows map[string]string }

// Get looks k up, and reports whether it was there.
func (d *DB) Get(k string) (string, bool) {
	v, ok := d.rows[k]
	return v, ok
}

// Put stores v under k, replacing any value already there.
func (d *DB) Put(k, v string) {
	if d.rows == nil {
		d.rows = map[string]string{}
	}
	d.rows[k] = v
}

// Greeting is a message, not a design decision: repeating it leaks nothing.
const Greeting = "hello world"
