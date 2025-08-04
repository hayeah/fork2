package render

import (
	"io/fs"
	"os"
	"path/filepath"
)

// CwdFS is a filesystem implementation that resolves paths relative to a base directory.
// Unlike os.DirFS, it does not validate paths, allowing .., absolute, or sibling references.
type CwdFS struct {
	Base string
}

// Compile-time check that CwdFS implements fs.FS
var _ fs.FS = (*CwdFS)(nil)

// Open opens the named file.
func (c *CwdFS) Open(name string) (fs.File, error) {
	full := filepath.Join(c.Base, name)
	full = filepath.Clean(full)
	return os.Open(full)
}
