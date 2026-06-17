# Quirks in the Afero library

## `File.Name()` does not return the base name

Afero has a `BasePathFs` which represents the concept of
a "filesystem rooted at a path R", which has a corresponding
`BasePathFile` type returned from methods like `Open`.

`BasePathFile` implements the `afero.File.Name() string`
method which is [implemented like this](https://github.com/spf13/afero/blob/master/basepath.go#L38C1-L41C2):

```go
type BasePathFs struct {
	source Fs
	path   string
}

type BasePathFile struct {
	File
	path string // same as corresponding BasePathFs.path (not documented upstream)
}

func (f *BasePathFile) Name() string {
	sourcename := f.File.Name()
	return strings.TrimPrefix(sourcename, filepath.Clean(f.path))
}
```

Despite having the same signature as the stdlib's
`io/fs.File.Name() string`, which is guaranteed to
return the _base name_ of the file, here, `Name()`
tracks the FS-root-relative path. There is zero documentation
of this behavior in the [afero.File docs](https://pkg.go.dev/github.com/spf13/afero#File)

This led to a confusing CI failure with a `Name()` call
which returned `\foo.txt` on Windows.
