package model_test

import (
	"fmt"
	"path/filepath"
	"slices"
	"testing"

	"github.com/samuel-form3/deslop-go/internal/model"
)

func load(t *testing.T, fixture string) *model.Model {
	t.Helper()
	dir, err := filepath.Abs(filepath.Join("testdata", fixture))
	if err != nil {
		t.Fatal(err)
	}
	m, err := model.Load(dir, []string{"./..."})
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestFieldRefs(t *testing.T) {
	m := load(t, "refs")
	var got []string
	for _, r := range m.Refs {
		if r.Field == nil || r.Pkg.Short() != "user" {
			continue
		}
		how := "works on"
		switch {
		case r.Copy:
			how = "copies"
		case r.Init:
			how = "sets up"
		}
		got = append(got, fmt.Sprintf("%s %s %s", r.UserFunc.Name, how, r.Field.Name))
	}
	want := []string{
		"Read copies N",
		"Read copies Pos",
		"Write works on N",
		"Address works on N",
		"Index works on Xs",
		"Getter works on Xs",
		"Getter copies name",
		"Setter works on name",
		"Literal sets up N",
	}
	if !slices.Equal(got, want) {
		t.Errorf("field references:\n  got  %q\n  want %q", got, want)
	}
}

func TestFuncs(t *testing.T) {
	m := load(t, "refs")
	var got []string
	for _, th := range m.Things {
		for _, f := range th.Funcs {
			got = append(got, fmt.Sprintf("%s method=%t wrapper=%t satisfies=%t", f, f.Method, f.Wrapper, f.Satisfies))
		}
	}
	slices.Sort(got)
	want := []string{
		"box.Box.Close method=true wrapper=true satisfies=false",
		"box.Box.Error method=true wrapper=false satisfies=true",
		"box.Box.Name method=true wrapper=true satisfies=false",
		"box.Box.SetName method=true wrapper=true satisfies=false",
		"box.Box.Values method=true wrapper=true satisfies=false",
		"box.Box.close method=true wrapper=false satisfies=false",
		"box.New method=false wrapper=false satisfies=false", // a constructor belongs to its type
		"user.Address method=false wrapper=false satisfies=false",
		"user.Getter method=false wrapper=false satisfies=false",
		"user.Index method=false wrapper=false satisfies=false",
		"user.Literal method=false wrapper=false satisfies=false",
		"user.Read method=false wrapper=false satisfies=false",
		"user.Setter method=false wrapper=false satisfies=false",
		"user.Write method=false wrapper=false satisfies=false",
	}
	if !slices.Equal(got, want) {
		t.Errorf("funcs:\n  got  %q\n  want %q", got, want)
	}
}
