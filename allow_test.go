package main

import (
	"path/filepath"
	"testing"

	"github.com/samuel-form3/deslop-go/internal/lint"
)

func TestAllowRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".deslop-go.allow")
	muted := []lint.Finding{
		{Lint: lint.FieldLeak, Key: "example.com/m/queue.RunQueue.Items"},
		{Lint: lint.ShallowModule, Key: "example.com/m/store"},
	}
	if err := writeAllow(path, muted); err != nil {
		t.Fatal(err)
	}
	allowed, err := readAllow(path)
	if err != nil {
		t.Fatal(err)
	}

	fresh := lint.Finding{Lint: lint.FieldLeak, Key: "example.com/m/queue.RunQueue.Head"}
	kept, gone := allowed.filter([]lint.Finding{muted[0], fresh})
	if len(kept) != 1 || kept[0].Key != fresh.Key {
		t.Errorf("kept %v, want only %v", kept, fresh)
	}
	if gone != 1 {
		t.Errorf("gone = %d, want 1: the shallow store was fixed", gone)
	}
}
