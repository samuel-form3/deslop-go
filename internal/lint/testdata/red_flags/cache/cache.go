package cache

import "example.com/rf/backend"

// Cache adds nothing to the backend it holds: two of its three methods pass
// straight through.
type Cache struct {
	db   *backend.DB
	hits int
}

func New() *Cache { return &Cache{db: &backend.DB{}} }

func (c *Cache) Get(k string) (string, bool) { return c.db.Get(k) }

func (c *Cache) Put(k, v string) { c.db.Put(k, v) }

// Hits counts the lookups since the cache was made.
func (c *Cache) Hits() int { return c.hits }

// TraceHeader is also spelled out in server: both know the header's name.
var TraceHeader = "X-Request-Id"

var welcome = "hello world"
