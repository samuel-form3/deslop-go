# deslop-go

An opinionated linter for sloppy code structure in Go modules, after
[deslop-please](https://github.com/deterministic-machines/deslop-please) for
Rust. It follows [*A Philosophy of Software Design*](https://www.amazon.com/Philosophy-Software-Design-John-Ousterhout/dp/1732102201):
**deep** modules, **narrow** interfaces.

```bash
go install github.com/samuel-form3/deslop-go@latest
```

```text
> deslop-go

error[field-leak]: `queue.RunQueue.Items` is known outside `RunQueue`: 5 places in 2 packages
  --> sched/policy/policy.go:6:4
  = note: sched/policy/policy.go:6:4: `sched/policy.Admit`
  = note: sched/policy/policy.go:6:21: `sched/policy.Admit`
  = note: sched/policy/policy.go:10:9: `sched/policy.Next`
  = note: sched/report/report.go:6:15: `sched/report.Backlog`
  = note: sched/report/report.go:10:25: `sched/report.Drained`
    queue.RunQueue
      Items ◀━┳━ 3  sched/policy
              ┗━ 2  sched/report
      also known outside: Head, buf
      kept: limit

...

deslop-go: 3 errors
```

`deslop-go [packages]` takes package patterns, `./...` by default. It loads
every package of their modules so that all callers are known, but only reports
findings that touch the packages given. Test files are left out. It exits 1
when it has findings, and 2 when it fails, for example on code that does not
compile. `-skip comment-repeats-code,vague-name` turns lints off.

### It's not my fault

Sure, let's just mute them:

```bash
deslop-go -update-allow .deslop-go.allow
deslop-go -allow .deslop-go.allow
```

The allow file lists one finding per line as `<lint> <key>`. Keys hold no line
numbers, so they survive edits. When a muted finding is fixed, deslop-go says
so, so that the file can be updated.

### Fixing findings with an agent

[`skills/deslop-go`](skills/deslop-go/SKILL.md) is an Agent Skill. It walks
an agent through fixing findings: group them by cause, read how the code is
used, propose the narrower interface with a before/after diagram, and verify
that the findings are gone rather than moved. It also lists, for each rule,
the Go patterns that fix it and the moves that only hide it. For Claude Code,
copy the folder into `~/.claude/skills/`, or into `.claude/skills/` in your
repo.

For other agents, add to your `AGENTS.md`:

```markdown
- Run [`deslop-go`](https://github.com/samuel-form3/deslop-go) to check for sloppy code structure.
- Errors flagged by `deslop-go` must be fixed from first principles, don't do mechanical moves to hide them.
    - `module-cycle` & `tangle`: what is the intended layer of abstraction? Which package or type should be at which layer?
    - `field-leak` & `shallow-module`: Why do we have these fields/functions at all? What are the patterns of use? Can they be hidden behind a narrow interface? Don't fix each leak one at a time, fix them holistically.
    - `pass-through-method`, `pass-through-variable` & `overexposure`: which layer owns this decision? Pull the complexity down into it, rather than making every layer above know about it.
    - `information-leakage`: which package owns this decision? Give it one home, and make the others ask that package for it.
    - `comment-repeats-code` & `vague-name`: say what the code cannot: what it means, why it exists, what callers can rely on.
    - Draw a diagram to help the user understand the before and proposed after structure.
```

## The rules

They are really one idea: **deep** modules, **narrow** interfaces. Each rule
is one of the book's red flags. The first four are the ones of deslop-please;
the rest are red flags the book names that can be found in code with good
precision. All are translated to the way Go structures code.

| Lint | Red flag in the book |
|---|---|
| `module-cycle`, `tangle` | layers that know each other: no clear abstraction for each |
| `field-leak` | information leakage, through a struct's fields |
| `shallow-module` | shallow module |
| `pass-through-method` | pass-through method |
| `pass-through-variable` | pass-through variable (section 7.5) |
| `overexposure` | overexposure, and "pull complexity downwards" |
| `information-leakage` | information leakage, through literals |
| `comment-repeats-code` | comment repeats code |
| `vague-name` | vague name |

The other red flags (temporal decomposition, special-general mixture,
conjoined methods, hard to describe, nonobvious code...) need judgment about
intent, which no static check has. The [skill](skills/deslop-go/SKILL.md)
asks the agent to look for them while it fixes the rest.

deslop-go sees code the way `go doc` does: a *thing* is a package-level
declaration, and a type owns its fields, its methods and the functions of its
package that construct it (`func NewQueue() *Queue`). "Held in mind" is 7.

### module-cycle

Go forbids import cycles between packages, but not between the directories
that group them. When code under `internal/api` uses `internal/store/models`,
and code under `internal/store` uses `internal/api/types`, neither `api` nor
`store` is the layer below the other. deslop-go reports sibling directories
that use each other in a circle, with the items that tie each pair.

### tangle

Things that know each other in a circle can only be understood together.
Since Go packages cannot form circles, a tangle always lives in one package.
It is reported when it holds more than 7 things, with the shortest circle
that shows it.

### field-leak

A struct guards its fields when it has methods that work on them and some
field that nothing outside knows. Then every other field that code outside
works on is a leak. The struct's insiders are its methods, its constructors,
the types of its package that embed it, and whatever it is tangled with.
Visibility does not matter: a function in the same package is outside too.

Working on a field means writing it, taking its address, indexing it, ranging
into it, or calling its methods. Calling a getter or setter that does any of
these counts too. Reading a copy of plain data (numbers, strings, and structs
and arrays of them) does not. Neither does setting fields in a composite
literal, since options structs are how Go configures.

### shallow-module

A package is shallow when it shows the other packages of its module at least
as many functions as it hides, more than 7 of them, and more than 7 sit in one
place: on one type, on types that free functions tie together, or among free
functions. Wrappers (getters, setters, and functions that only pass their
parameters on), and methods that implement an interface, count for nothing.

### pass-through-method

A type passes a method through when its body only calls a method of one of
the type's fields, or a function of another package, with the same
parameters in the same order. A type is reported when at least two of its
methods, and at least half of them, pass through: it adds a layer but no
abstraction. Methods that implement an interface are left out, since
decorators and adapters must have the interface's signature.

### pass-through-variable

A parameter passes through a function when the function does nothing with it
but pass it, as it is, to other functions. It is reported when it passes
through 3 functions in a row: each has to know about a value only the end of
the chain needs. A `context.Context` is left out, as it is Go's answer to the
problem the book solves with a context object. So are functions whose
signature is not theirs to choose (forwarders, interface implementations,
functions used as values), and chains of methods of the same name, which hand
a value down the levels of a nested structure.

### overexposure

A parameter that every caller sets to the same constant, over at least 3
calls, is a decision that nobody makes: every caller has to know about it,
and the function should take it itself. Interface implementations and
functions used as values are left out.

### information-leakage

The same key, path, name or format spelled out as a string literal in
several packages (`"v1/pods"`, `"X-Request-Id"`, `".kube"`) is one design
decision known in several places: changing it means finding them all. Only
literals that look like such decisions count: no spaces, at least 3 letters,
some punctuation, and no format verbs. Literals shared by the same packages
are reported together.

### comment-repeats-code

A doc comment repeats its code when every word in it, once articles and
words like "returns" or "new" are set aside, is one of the names its
declaration shows: `// NewServer returns a new server.` It is reported per
package.

### vague-name

A package named `util`, `common`, `helpers`, `misc`, `shared` or the like
says nothing about what it provides, so it gathers anything. Go code reads
every name with its package (`version.Info`, `store.Open`), so a vague package
makes every name in it vague.
