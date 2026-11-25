package server

import (
	"context"
	"net/http"

	"example.com/rf/cache"
)

type Server struct{ c *cache.Cache }

// NewServer returns a new server.
func NewServer(c *cache.Cache) *Server { return &Server{c: c} }

// Lookup answers a request from the cache, tagging the reply for tracing.
func (s *Server) Lookup(w http.ResponseWriter, r *http.Request) {
	v, _ := s.c.Get(r.URL.Path)
	w.Header().Set("X-Request-Id", r.Header.Get("X-Request-Id"))
	_, _ = w.Write([]byte(v + "hello world"))
}

// Serve listens on addr until ctx is done. addr and verbose go down three
// functions untouched before open uses them; ctx does too, but a context is
// how Go carries values through calls.
func Serve(ctx context.Context, addr string, verbose bool) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return start(ctx, addr, verbose)
}

func start(ctx context.Context, addr string, verbose bool) error {
	backlog := 128
	return listen(ctx, addr, verbose, backlog)
}

func listen(ctx context.Context, addr string, verbose bool, backlog int) error {
	if backlog < 0 {
		return nil
	}
	return open(ctx, addr, verbose)
}

func open(ctx context.Context, addr string, verbose bool) error {
	if verbose {
		println("listening on", addr)
	}
	return ctx.Err()
}

// dial connects to host, trying again up to retries times. Every caller
// passes 3: the choice belongs in dial.
func dial(host string, retries int) error {
	for i := 0; i <= retries; i++ {
		if host != "" {
			return nil
		}
	}
	return nil
}

// Connect dials each replica in turn.
func Connect() error {
	if err := dial("a", 3); err != nil {
		return err
	}
	if err := dial("b", 3); err != nil {
		return err
	}
	return dial("c", 3)
}
