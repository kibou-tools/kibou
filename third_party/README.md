# Third-party libraries

This folder contains forks which are depended
upon by some forked repository and need some patches
or build rewiring to work with the monorepo.

For example, the `build-tools` folder is a fork
of [go-delve/build-tools](https://github.com/go-delve/build-tools)
where we use `go.work` at the repo root to
point it to use our fork of `golang.org/x/tools`,
instead of using an older version based on its
own `go.mod`. In this case, the Delve tests
were literally invoking `go run` commands with
`@latest`, not using `build-tools` as a proper
dependency, so the existing `go.work` "make dependencies
use our workspace modules instead" didn't work.
