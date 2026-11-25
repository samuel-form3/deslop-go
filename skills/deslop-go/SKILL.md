---
name: deslop-go
description: Fix the findings of deslop-go, a linter for the red flags of A Philosophy of Software Design in Go modules (module-cycle, tangle, field-leak, shallow-module, pass-through-method, pass-through-variable, overexposure, information-leakage, comment-repeats-code, vague-name), from first principles, by redesigning interfaces rather than hiding findings. Use when the user asks to run deslop-go, to deslop Go code, to fix deslop-go errors, to review or clean up the structure of Go packages, or when deslop-go fails in CI or a hook.
---

# deslop-go

`deslop-go` reports sloppy structure in Go code, after *A Philosophy of
Software Design*: **deep** modules, **narrow** interfaces. Every finding is
the same problem: a reader has to hold too much in mind, because a package or
type exposes how it works instead of what it does.

Running it is easy. Fixing what it finds is the job, and the easy fixes are
wrong: they hide the finding and keep the structure.

## 1. Run it

```bash
deslop-go ./...                            # or the packages that changed
deslop-go -allow .deslop-go.allow ./...    # if the repo has an allow file
```

If it is missing, install it: `go install github.com/samuel-form3/deslop-go@latest`.
It exits 0 when clean, 1 with findings, 2 when it fails (most often: the code
does not compile; fix that first).

Each finding has a `-->` location, notes with every place involved, and for
field leaks a picture of who touches which field. Read the notes: they are the
evidence for the redesign.

## 2. Group the findings by cause

Never fix findings one at a time. Group them first:

- All `field-leak`s of one struct are one finding: the struct's interface is
  missing the operations its callers do by hand.
- A `shallow-module` and the `field-leak`s of the types in that package often
  share a cause: the package hands out its parts instead of doing the work.
- A `module-cycle` and the `tangle`s inside it often come from one misplaced
  layer.
- A `pass-through-method` type, a `pass-through-variable` running through it
  and an `overexposure` at the end of it are often one layer that adds
  nothing, or one decision that no layer owns.
- `information-leakage` across packages often shares a cause with the
  `module-cycle` or `field-leak` between those same packages.
- `comment-repeats-code` and `vague-name` are fixed last, and best as part of
  the redesign: once a type or package has a clear job, it is easy to name
  and to document.

Tell the user how many groups there are, and which you suggest to start with:
the one whose fix makes the most findings go away.

## 3. Understand the use before you design

For each group, read the code at every place in the notes and answer:

- What are the callers trying to do? Name the operations (`admit a job`,
  `drain the backlog`), not the fields they touch.
- Do several callers repeat the same steps? That repeated sequence is the
  missing method.
- Which layer should know this? Who is the lower layer, who is above it?
- Why does this field or function exist at all? Can it go away?

While reading, look for the book's red flags that no linter can find, and
mention them if they explain the group: **temporal decomposition** (code split
by the order things happen, not by what each part knows), **special-general
mixture** (a general mechanism that knows about one of its uses),
**conjoined methods** (two functions that can only be understood together),
**hard to describe** (no short comment can say what it does), and
**nonobvious code**.

## 4. Propose, with a diagram, before changing code

Show the user, for each group:

- A before/after diagram in plain text: packages or types as boxes, uses as
  arrows. Show which arrows disappear, and the new narrow interface.
- The new interface itself: the signatures of the methods callers will use.
- What moves where, and which call sites change.

Ask before you start. These changes span packages, and the user knows things
the code does not say, such as code in other modules that uses an exported
name. deslop-go only sees the module it runs in.

For the patterns that usually fix each rule in Go, and the moves that only
hide a finding, read [references/fixes.md](references/fixes.md).

## 5. Change, then verify

1. Make the change one group at a time. Keep behaviour the same: this is a
   refactor.
2. Run `go build ./...`, `go vet ./...` and the tests.
3. Run `deslop-go` again. Check that the group's findings are **gone**, not
   moved: a new leak on another field, a new cycle one directory up, or a
   new shallow package means the structure did not change.
4. Report what changed, with the after diagram, and the findings that remain.

## The allow file

The allow file mutes findings. Do not add to it, or run `-update-allow`, to
make findings go away, unless the user asks to. It is for adopting deslop-go
in a codebase with existing findings, not for silencing new ones. When
deslop-go says muted findings are gone, run `-update-allow` to forget them,
after checking that no new finding slipped into the file.
