package lint_test

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/samuel-form3/deslop-go/internal/lint"
	"github.com/samuel-form3/deslop-go/internal/model"
)

func TestFixtures(t *testing.T) {
	for _, tc := range []struct {
		fixture string
		want    []string
	}{
		{"module_cycle", []string{
			"module-cycle example.com/mc/kinds,example.com/mc/socket",
		}},
		{"tangle", []string{
			"tangle example.com/tg/ring8:A,B,C,D,E,F,G,H",
		}},
		{"field_leak", []string{
			"field-leak example.com/fl/queue.RunQueue.Head",
			"field-leak example.com/fl/queue.RunQueue.Items",
			"field-leak example.com/fl/queue.RunQueue.buf",
		}},
		{"shallow", []string{
			"shallow-module example.com/sh/store",
		}},
		{"red_flags", []string{
			`comment-repeats-code example.com/rf/server`,
			`information-leakage example.com/rf/cache,example.com/rf/server:"X-Request-Id"`,
			`overexposure example.com/rf/server.dial(retries)`,
			`pass-through-method example.com/rf/cache.Cache`,
			`pass-through-variable example.com/rf/server.Serve(addr,verbose)`,
			`vague-name example.com/rf/util`,
		}},
	} {
		t.Run(tc.fixture, func(t *testing.T) {
			dir, err := filepath.Abs(filepath.Join("testdata", tc.fixture))
			if err != nil {
				t.Fatal(err)
			}
			m, err := model.Load(dir, []string{"./..."})
			if err != nil {
				t.Fatal(err)
			}
			findings := lint.Run(m)
			var out strings.Builder
			lint.Print(&out, findings, dir)
			t.Log("\n" + out.String())

			var got []string
			for _, f := range findings {
				got = append(got, f.Lint+" "+f.Key)
			}
			slices.Sort(got)
			if !slices.Equal(got, tc.want) {
				t.Errorf("findings:\n  got  %q\n  want %q", got, tc.want)
			}
		})
	}
}

func TestScope(t *testing.T) {
	dir, err := filepath.Abs(filepath.Join("testdata", "field_leak"))
	if err != nil {
		t.Fatal(err)
	}
	// A leak is reported from either end: the struct's package, or a user's.
	// report only reads a copy of Head, so Head does not leak through it.
	for pattern, want := range map[string]int{"./queue": 3, "./sched/policy": 2, "./sched/report": 2} {
		m, err := model.Load(dir, []string{pattern})
		if err != nil {
			t.Fatal(err)
		}
		if got := len(lint.Run(m)); got != want {
			t.Errorf("%s: got %d findings, want %d", pattern, got, want)
		}
	}
}
