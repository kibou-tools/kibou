# Go style guide

This guide applies to non-forked folders only (for example, `common/`, `misc/`, and `.github/`-adjacent helper code).

## Imports

- Group order: stdlib, third-party, monorepo (`code.kibou.tools/...`).

## File organization

### Organizing library files

After the imports, the order of declarations in a non-test file
should be:

1. (Optional) Aliases for exported use.
2. One or more sections of:
   - Key type
   - Supporting types, such as for a field. These should not have any methods. If it does, it should generally go after the methods of the type it is supporting.
   - Type assertions for interfaces implemented deliberately. (exception: `fmt.Stringer`)
   - Initialization functions (usually `NewTypeName(...) T` or `NewTypeName(...) (T, error)`)
   - Builder functions (usually `WithXYZ(...)`)
   - Exported read-only methods
   - Exported read-write methods
   - Methods for implementing interfaces, grouped by interface.
   - Non-exported methods
3. Private helper functions
```

Sections should be organized in decreasing order of importance
of the corresponding type.

### Test organization

1. Prefer having one test per method.

   - Unit tests/table-driven tests should be organized as one sub-test.
   - Property-based tests should be organized as a second sub-test.

   The additional nesting is not necessary if only unit tests or only
   property-based tests are used.

2. If there are interaction tests across methods of a single type,
   prefer having one `TestTypeName` with subtests.

3. For large test groups with many subtests, factor related subtest groups
   into helpers that accept `check.Harness`; see `TestWalk` in
   `common/fsx/fsx_walk/walk_test.go` for the preferred organization style.

## Comments

### Field-level comments

1. Fields of nilable types (interfaces, pointers, slices etc.)
   type should have comments "Always non-nil" or a concise 
   description of when the field can be nil.
   - Prefer using `Option` for clarity.
2. For "hand-rolled" enums with a `kind` field, different fields
   should have information about which kinds they are applicable for.
3. If a `string`, slice or map typed field may be empty,
   document when it may be empty.

## Naming

### Avoid negation in names

```go
type ABC struct {
    xyzDisabled bool // ❌
}

func HasNoPathSeparators(p string) bool // ❌
```

Use positive naming, letting the caller handle negation.

```go
type ABC struct {
    xyzEnabled bool // ✔️
}

func HasPathSeparators(p string) bool // ✔️
```

**Q**: Won't this lead to potential bugs with default initialization
or higher verbosity when the more common desired setting is `true`?

**A**: If that kind of default initialization is desired, expose a helper
function. Higher verbosity is not a reason to reduce readability.
Overall the risk of confusing code due to double negations is higher.

## 'Type-unique' arguments bundled as first parameter

Examples:

- `context.Context`.
- `*logx.Logger`.
- `logx.LogCtx` (bundles `Logger` and `context.Context` together).

If you need to create more bundles, define a dedicated type by
embedding the relevant dependencies and pass that around, instead
of repeating several arguments at multiple sites in a call chain.

## Defaults and zero values

Zero values and default values are separate concepts.
Avoid mixing them, preferring `Option` and `Default`
functions, unless there's a strong performance reason
to do otherwise.

```go
type FormatSpec struct {
    // Empty means that the default formatting will be used. ❌
    layout string
}
```

```go
type FormatSpec struct {
    // layout may be empty if nothing is to be formatted
    layout string
}

func FormatSpecDefault() FormatSpec { // ✅
    return FormatSpec{layout: ...}
}
```

Default value functions should generally be named as
`TypeNameDefault` for better auto-complete support.

## Enum-like constants use `Type_Value` naming

```go
type ListProvenance int

const (
	ListProvenance_All        ListProvenance = iota + 1
	ListProvenance_FirstParty
	ListProvenance_Forked
)
```

Start at `iota + 1` so that the zero value is distinct from all valid cases.

## Optional customization points should go in a dedicated Options struct

```go
func RunThing(ctx logx.LogCtx, target string, options RunThingOptions) error
```

Bundling customization points allows passing the value through multiple
functions with less repetition, and provides a natural documentation point
(field definition) for the semantics of each options, instead of having
to repeat that at every function.

Related: <https://matklad.github.io/2026/02/11/programming-aphorisms.html>

Generally, the fields of an `Options` type will fall into one of 3 cases:
- The zero value for the field is a sensible default.
- Have type `common/core.Option`,
- The field is initialized to a sensible default by the matching
  `func New*Options` constructor.
