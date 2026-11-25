# Fixing deslop-go findings in Go

For each rule: what deslop-go measures, the fixes that change the structure,
and the moves that only hide the finding. A hiding move may make deslop-go
quiet, but it leaves the reader with as much to hold in mind as before.

## Contents

- [field-leak](#field-leak)
- [shallow-module](#shallow-module)
- [module-cycle](#module-cycle)
- [tangle](#tangle)
- [pass-through-method](#pass-through-method)
- [pass-through-variable](#pass-through-variable)
- [overexposure](#overexposure)
- [information-leakage](#information-leakage)
- [comment-repeats-code](#comment-repeats-code)
- [vague-name](#vague-name)

## field-leak

**Measures:** a struct that has methods working on its fields, and at least
one field nothing outside knows, has other fields that code outside works on.
"Outside" is anything but its methods, its constructors (`NewX` returning
`X`/`*X`), types of its package that embed it, and code tangled with it,
whatever the visibility. Working on a field: writing it, `&x.f`, indexing,
ranging, calling its methods, or calling a getter or setter that does. Reading
a copy of plain data, and setting fields in a composite literal, are fine.

**Decide first which the struct is:**

- **An object with invariants** (a queue, a cache, a connection): callers
  should not know any field. Every operation they do by hand becomes a method.
- **A value** (an event, a config, a row): there is nothing to guard. All
  fields can be known, and the methods should not pretend to keep invariants.

The finding fires because the struct is both at once. Pick one.

**Fixes:**

- *Tell, don't ask.* Replace the steps callers do on the fields with one
  method named after what they want:

  ```go
  // before, in two packages
  q.Items = append(q.Items, job)
  for q.Head < len(q.Items) { run(q.Items[q.Head]); q.Head++ }

  // after
  q.Push(job)
  for job, ok := q.Pop(); ok; job, ok = q.Pop() { run(job) }
  ```

- *Look for the repeated sequence.* When three callers each check, then
  update, then log, that sequence is one method.
- *Answer questions, don't hand out state.* `q.Backlog() int` instead of
  `len(q.Items) - q.Head` at every caller.
- *Return values, not internals.* If callers need a view, return a copy or
  an iterator (`func (q *Queue) All() iter.Seq[Job]`), not the slice or map
  itself.
- *Construct with a constructor or options.* Callers that set up state after
  construction want `New(opts)`.

**Hiding moves, don't:**

- Adding a getter that returns the field (`func (q *Q) Items() []Job`):
  deslop-go sees through getters and setters, and the caller still knows it.
- Unexporting the field and moving the caller into the same package as a free
  function: still outside.
- Turning each outside function into a method one by one, body unchanged:
  the type grows a method per caller, and the leak tends to come back as a
  `shallow-module`.
- Exposing the last kept field so that "nothing is kept": the struct still
  pretends to keep invariants.

## shallow-module

**Measures:** a package shows the other packages of its module at least as
many functions as it hides, more than 7, and more than 7 sit on one type, on
types tied together by free functions, or among free functions. Getters,
setters and forwarders count for nothing, and so do methods that implement an
interface. Uses from other modules are not seen: check with the user before
removing exported API.

**Fixes:**

- *Find the use cases.* List what the callers do with the package. Usually a
  handful of operations, each made of several calls in a fixed order. Make
  each one a method, and unexport the steps.
- *Pull general-purpose work down.* If callers combine `Get`, `Has`, `Put`
  and `Delete` into "update if present", the package should do that.
- *Unexport what only one caller, or only tests, uses.* Tests can sit in the
  package (`package store`, not `package store_test`).
- *Accept narrow interfaces in the consumer.* The Go idiom: the caller
  declares the two or three methods it needs (`type loader interface {
  Load(ctx, id) (Item, error) }`). It is a fix only if the interface is narrow
  and the consumer needs nothing else.
- *Merge a package into its only caller* when it is only a bag of steps for
  that caller.

**Hiding moves, don't:**

- Declaring an interface that lists every method, only so that the methods
  count as implementing one.
- Turning functions into one-line forwarders, or wrapping them in a struct
  of function fields.
- Adding unexported functions to make "hidden" larger.
- Splitting one type into several small ones that callers still use all of.

## module-cycle

**Measures:** two or more sibling directories of the module, such as
`internal/api` and `internal/store`, whose code uses each other's (through
any packages beneath them). Go already forbids package import cycles. This
finds the layering problem one level up.

**Fixes:**

- *Decide the layering.* Which directory is lower: it must not know the one
  above. Write it down in the diagram.
- *Move what the lower layer needs down.* Often a type or constant lives in
  the upper layer but belongs to the lower one (`BacklogMost` belongs with
  the kinds it limits, not in `socket/limits`). Moving one item is right when
  it really belongs there, so explain why.
- *Invert the dependency.* The lower layer declares the interface it needs,
  and the upper layer implements it and passes it in (constructor injection,
  no globals).
- *Extract the shared vocabulary* into a leaf package that both use and that
  uses neither, only if it is a real concept (domain types), not a
  `common`/`util` bucket.
- *Merge the directories* when they are really one module.

**Hiding moves, don't:**

- Re-exporting with a type alias or a `var` in a third package.
- Passing values as `any` and asserting their type on the other side.
- Registries filled in `init()` so the import disappears but the knowledge
  stays.
- Renaming or nesting directories so that one contains the other.

## tangle

**Measures:** more than 7 things in one package (types with their methods,
functions, vars, consts) that know each other in a circle: each can be
understood only with all the others. The finding shows the shortest circle.

**Fixes:**

- *Find the bottom.* Which things should know nothing of the others? Usually
  data types and small helpers. Remove their references to the things above,
  which are the back-edges in the circle.
- *Pass what is needed, not the whole parent.* A child that takes `*Server`
  to read one setting should take the setting.
- *Callbacks or interfaces owned by the lower thing,* when it really must
  call up: it declares `type notifier interface{ Notify(Event) }` and the
  upper thing implements it.
- *Split the package along the layers you found.* Go forbids import cycles,
  so once the parts sit in separate packages the compiler keeps the layering
  from coming back. Only split where the cut is real.

**Hiding moves, don't:**

- Moving code between files: a tangle is about who knows whom, not where it
  is written.
- Replacing types with `any` or `interface{}` to cut an edge.
- Adding an interface that mirrors a concrete type's full method set, only
  to cut the edge.

## pass-through-method

**Measures:** at least two, and at least half, of a type's methods only call
a method of one of its fields, or a function of another package, passing the
same parameters in the same order. Methods that implement an interface are
left out.

**Fixes:**

- *Decide what the outer type is for.* If it adds nothing, remove it and let
  callers use the inner one. If it should add something (validation, caching,
  translation between two vocabularies), make each method do that, and stop
  exposing the inner type's operations one by one.
- *Expose the inner value.* If callers need the inner type's operations, a
  method returning it, or embedding it, says so honestly. Embedding gives
  the operations without writing a method per call.
- *Merge the two layers* when they split one responsibility in two.

**Hiding moves, don't:**

- Adding a statement (a log line, a nil check) to each method so that it no
  longer only forwards.
- Declaring an interface with the forwarded methods so that they count as
  implementing one, when nothing calls through it.

## pass-through-variable

**Measures:** a parameter passed down at least 3 functions in a row that do
nothing with it but pass it on. `context.Context` is left out, and so are
forwarders, interface implementations, functions used as values and chains of
methods of the same name.

**Fixes:**

- *Store it where it is needed.* If the end of the chain is a method, give
  its type the value when it is built (a field set by its constructor), and
  drop the parameter from the chain.
- *Group the functions in a type.* When the middle functions are free
  functions that all carry the same values, they are the methods of a type
  that holds those values.
- *Let the end of the chain find it.* Configuration that only the bottom
  layer needs belongs to the bottom layer: load it there, or pass it once at
  setup.
- *Use a context object, sparingly.* The book's fix is a context shared by
  the layers. In Go, that is a struct built once and held by the types that
  need it. Putting request-scoped values in a `context.Context` is right for
  deadlines, cancellation and request IDs, not for configuration or
  dependencies.

**Hiding moves, don't:**

- Moving the value into a package-level variable.
- Bundling it with unrelated parameters in an `Options` struct that every
  layer still passes down.
- Putting it in a `context.Context` with `context.WithValue`.

## overexposure

**Measures:** every call (at least 3) passes the same constant for a
parameter. Interface implementations and functions used as values are left
out. Callers in other modules are not seen: ask before changing exported
API.

**Fixes:**

- *Pull the complexity down.* Remove the parameter, and let the function use
  the value. If a rare caller does need another value, give that caller a
  second function or an option, so that the common case stays simple.
- *Name the default.* If the value is a policy (a timeout, a retry count, a
  file mode), it belongs in one named place next to the code that applies it.
- *Replace a boolean flag with two functions,* when every caller passes the
  same one: the other branch may be dead code.

**Hiding moves, don't:**

- Changing one call to pass a different value, or passing a variable that
  holds the same constant.
- Wrapping the function in another that fixes the value, while keeping both.

## information-leakage

**Measures:** the same string literal, that looks like a key, path, name or
format, written in two or more packages. Literals that the same packages
share are reported together.

**Fixes:**

- *Find the owner.* The decision belongs to the package that defines what
  it means: the package that writes the file owns the file name, the
  package that talks to the API owns its paths and resource names.
- *Hide the decision, not just the string.* A constant in the owner is a
  start. Better is a function that does the work, so that others do not
  need the string at all: `config.Dir()` rather than
  `filepath.Join(home, config.DirName)` in every caller.
- *Use the types that already exist.* Kubernetes resource names have
  `schema.GroupVersionResource`. HTTP headers have the `net/http` package.
  Flags have one parser.

**Hiding moves, don't:**

- Building the string from parts (`"v1/" + "pods"`), or copying it into a
  constant in each package.
- Moving the literal into a `common`/`util` package that everyone imports:
  that has a home, but no owner.

## comment-repeats-code

**Measures:** a doc comment whose words, articles and verbs like "returns"
set aside, are all names its declaration already shows: `// NewServer
returns a new server.`

**Fixes:**

- *Say what the code cannot:* what a value means, its units and bounds, what
  happens on error, what callers can rely on, why the function exists.
  `// Timeout is how long a request may wait for a free connection; zero
  waits forever.`
- *If nothing is left to say, the comment can go.* Unexported declarations
  with an obvious job need no comment. Exported ones in Go conventionally
  have one, so make it say something.
- *If you cannot say what it does without repeating its name,* the
  abstraction may be unclear: that is the book's "hard to describe" red flag.

**Hiding moves, don't:**

- Adding filler words that say nothing new ("This function is used to...").

## vague-name

**Measures:** a package named `util`, `common`, `helpers`, `misc`, `shared`
or the like.

**Fixes:**

- *Move each thing to the package of what it works on.* A string helper used
  by the config loader belongs in `config`. Duplicating a three-line helper
  in two packages is often better than a shared bag.
- *Name packages after what they provide:* `retry`, `kubeconfig`,
  `pagination`, not `util`.

**Hiding moves, don't:**

- Renaming `util` to another word that still says nothing, such as `core`,
  `pkg`, `support` or `tools`.
