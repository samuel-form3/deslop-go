package lint

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/samuel-form3/deslop-go/internal/model"
)

// commentsRepeatCode finds doc comments that say nothing the declaration does
// not already say, such as "// GetUser gets the user." A comment should give
// what the code cannot: what a value means, its units and bounds, why a
// function exists, what callers can rely on.
//
// A comment repeats its code when, once short words are set aside, every word
// in it is one of the names its declaration shows.
func commentsRepeatCode(m *model.Model) []Finding {
	byPkg := map[*model.Package][]*model.Doc{}
	for _, d := range m.Docs {
		if d.Pkg.InScope && repeats(d) {
			byPkg[d.Pkg] = append(byPkg[d.Pkg], d)
		}
	}
	var found []Finding
	for _, pkg := range m.Packages {
		docs := byPkg[pkg]
		if len(docs) == 0 {
			continue
		}
		var notes []Note
		for _, d := range docs[:min(len(docs), heldInMind)] {
			notes = append(notes, Note{Pos: d.Pos, Text: fmt.Sprintf("`%s`: %q", d.Name, strings.TrimSpace(d.Text))})
		}
		if more := len(docs) - heldInMind; more > 0 {
			notes = append(notes, Note{Text: fmt.Sprintf("and %d more", more)})
		}
		verb := "repeat"
		if len(docs) == 1 {
			verb = "repeats"
		}
		found = append(found, Finding{
			Lint:    CommentRepeatsCode,
			Key:     pkg.Path,
			Message: fmt.Sprintf("%s in `%s` only %s the code", plural(len(docs), "doc comment"), pkg.Short(), verb),
			Pos:     docs[0].Pos,
			Notes:   notes,
		})
	}
	return found
}

func repeats(d *model.Doc) bool {
	known := map[string]bool{}
	for _, name := range d.Names {
		for _, w := range words(name) {
			known[stem(w)] = true
		}
	}
	content := 0
	for _, w := range words(d.Text) {
		s := stem(w)
		if fillers[s] {
			continue
		}
		if !known[s] {
			return false
		}
		content++
	}
	return content > 0
}

// words splits text into lower case words, splitting identifiers written in
// camelCase or snake_case too.
func words(text string) []string {
	var out []string
	for field := range strings.FieldsFuncSeq(text, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) }) {
		runes := []rune(field)
		start := 0
		for i := 1; i < len(runes); i++ {
			// A word starts at an upper case letter after a lower case one,
			// or at the last upper case letter of an acronym: "HTTPServer".
			if unicode.IsUpper(runes[i]) && (unicode.IsLower(runes[i-1]) ||
				i+1 < len(runes) && unicode.IsUpper(runes[i-1]) && unicode.IsLower(runes[i+1])) {
				out = append(out, strings.ToLower(string(runes[start:i])))
				start = i
			}
		}
		out = append(out, strings.ToLower(string(runes[start:])))
	}
	return out
}

// stem cuts the common English endings off a word, so that "gets", "getting"
// and "get" are the same word.
func stem(w string) string {
	for _, end := range []string{"ies", "ing", "ed", "es", "s"} {
		if cut, ok := strings.CutSuffix(w, end); ok && len(cut) >= 3 {
			if end == "ies" {
				cut += "y"
			}
			w = cut
			break
		}
	}
	return strings.TrimSuffix(w, "e")
}

// fillers are the words that say nothing about the code: articles, linking
// words, and the verbs every comment starts with.
var fillers = func() map[string]bool {
	set := map[string]bool{}
	for w := range strings.FieldsSeq(`a an the this that these those it its is are be been was
		of for to in on at by with from into as and or if when then
		returns return gets get sets set creates create makes make new
		given specified provided passed current
		func function method type struct value field variable constant const var`) {
		set[stem(w)] = true
	}
	return set
}()
