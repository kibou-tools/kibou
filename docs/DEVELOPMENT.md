# Working on happygo

## Project structure

```
.
├─ common/
│  Meant for shared internal-use libraries for delve/,
│  tools/ and misc/. A proving ground for capability-based
│  standard library (for enhanced testability). 
│
├─ docs/
│  Project-wide docs. Docs specific to go/ live alongside
│  its docs, not here.
│
├─ go/
│  Tracking golang/go. The Go compiler and standard library.
│
├─ delve/
│  Tracking go-delve/delve. The debugger.
│
├─ tools/
│  Tracking golang/tools. Supplementary tools such as gopls,
│  gofmt etc.
│
└─ misc/
   Our own internal tools etc. The top-level tools/ is already
   taken so. :/
```

## Testing

Outside of the `go/` subdirectory:

```bash
go -C <subdir> test ./...
```

## Linting

```bash
go tool golangci-lint run ./common/... ./misc/...
```

See `.golangci.yml` at the repo root.

There are a fairly strict set of rules, so it's good
to familiarize yourself with them.

## Dos and Don'ts

### Do: Utilize SYNC comments for indicating non-obvious dependencies

When there is non-obvious coupling between different files,
it is useful to add a `SYNC(id: ...)` comment in one place,
and a `See SYNC(id: ...)` reference at the place which depends
on it.

Conversely, when a diff touches code near a SYNC comment, check
the other places where that comment is defined/referenced, to
see if they continue to be in sync or not.

### Don't: Just link to CI action results

Never add links to GitHub Actions runs in documentation, GitHub comments,
issues etc. The logs are only retained for 90 days.

Instead, if the full logs are important, then download them and attach them
in an appropriate place, and link to the attachment.

Alternately, if only a small part of the log is relevant, inline
the relevant context as a Markdown code block.

Linking to the logs is OK, but you should always inline the relevant
context (as a code block or an attachment).
