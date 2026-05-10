---
name: fmt-go
description: Format Go code using goimports with proper grouping.
---

<!-- FIXME(issue: kibou-tools/kibou#56): Remove this skill in favor of a simpler command. -->

# Format Go Code

Run goimports to format Go code files, grouping local imports under `code.kibou.tools`.

## Usage

From the repository root, run:

```bash
# Format a whole directory
GOROOT=$PWD/go ./go/bin/go run ./tools/cmd/goimports -local code.kibou.tools -w ./path/to/dir/...
# Format a specific file
GOROOT=$PWD/go ./go/bin/go run ./tools/cmd/goimports -local code.kibou.tools -w ./path/to/file.go
```
