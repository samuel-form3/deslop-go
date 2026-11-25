package main

import (
	"bufio"
	"os"
	"strings"

	"github.com/samuel-form3/deslop-go/internal/lint"
)

// An allow file mutes findings, one per line, as "<lint> <key>". Lines that
// are blank or start with # are ignored.
const allowHeader = "# deslop-go allow file: one muted finding per line, \"<lint> <key>\".\n"

// allowList is the set of muted findings, by allowKey.
type allowList map[string]bool

func allowKey(f lint.Finding) string { return f.Lint + " " + f.Key }

// filter returns the findings that are not muted, and how many muted
// findings no longer turn up.
func (a allowList) filter(findings []lint.Finding) (kept []lint.Finding, gone int) {
	matched := map[string]bool{}
	for _, f := range findings {
		if key := allowKey(f); a[key] {
			matched[key] = true
		} else {
			kept = append(kept, f)
		}
	}
	return kept, len(a) - len(matched)
}

func readAllow(path string) (allowList, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	allowed := allowList{}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(nil, 1<<20)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" && !strings.HasPrefix(line, "#") {
			allowed[line] = true
		}
	}
	return allowed, scanner.Err()
}

func writeAllow(path string, findings []lint.Finding) error {
	var b strings.Builder
	b.WriteString(allowHeader)
	for _, f := range findings {
		b.WriteString(allowKey(f))
		b.WriteByte('\n')
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}
