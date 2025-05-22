package render

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

// -----------------------------------------------------------------------------
// helpers
// -----------------------------------------------------------------------------

func withTempFile(t *testing.T, contents string) string {
	t.Helper()
	dir := t.TempDir()
	fp := filepath.Join(dir, "tmp.txt")
	if err := os.WriteFile(fp, []byte(contents), 0o644); err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	return fp
}

// -----------------------------------------------------------------------------
// pickLoader dispatch
// -----------------------------------------------------------------------------

func TestPickLoader_Dispatch(t *testing.T) {
	ctx := context.Background()
	assert := assert.New(t)

	// ── 1. stdin  ────────────────────────────────────────────────────────────
	testInput := bytes.NewBufferString("hello stdin")
	ld, err := pickLoader("-")
	assert.NoError(err)

	// Cast to StdinLoader and set the test Reader
	stdinLoader, ok := ld.(*StdinLoader)
	assert.True(ok, "expected loader to be *StdinLoader")
	stdinLoader.Reader = testInput

	got, err := ld.Load(ctx)
	assert.NoError(err)
	assert.Equal("hello stdin", got)

	// ── 2. literal (both aliases) ────────────────────────────────────────────
	for _, spec := range []string{"text:alpha", "literal:bravo"} {
		ld, err = pickLoader(spec)
		assert.NoError(err)
		out, _ := ld.Load(ctx)
		if spec == "text:alpha" {
			assert.Equal("alpha", out)
		} else {
			assert.Equal("bravo", out)
		}
	}

	// ── 3. file  ─────────────────────────────────────────────────────────────
	fp := withTempFile(t, "file-content")
	ld, err = pickLoader(fp)
	assert.NoError(err)
	out, err := ld.Load(ctx)
	assert.NoError(err)
	assert.Equal("file-content", out)

	// ── 4. HTTP(S)  ──────────────────────────────────────────────────────────
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hello over http"))
	}))
	defer srv.Close()

	ld, err = pickLoader(srv.URL)
	assert.NoError(err)
	out, err = ld.Load(ctx)
	assert.NoError(err)
	assert.Equal("hello over http", out)

	// ── 5. unknown scheme should error  ──────────────────────────────────────
	_, err = pickLoader("noscheme://foo")
	assert.Error(err)
}

// -----------------------------------------------------------------------------
// LoadAll behaviour
// -----------------------------------------------------------------------------

func TestLoadAll_Concatenation(t *testing.T) {
	assert := assert.New(t)
	ctx := context.Background()

	fp := withTempFile(t, "from-file")

	specs := []string{
		"text:first",
		fp,
		"text:last",
	}
	got, err := LoadContentSources(ctx, specs)
	assert.NoError(err)

	want := "first\n\nfrom-file\n\nlast"
	assert.Equal(want, got)
}

// -----------------------------------------------------------------------------
// Home-dir expansion (“~/…”) for file paths
// -----------------------------------------------------------------------------

func TestPickLoader_HomeExpansion(t *testing.T) {
	assert := assert.New(t)
	ctx := context.Background()

	home, err := os.UserHomeDir()
	assert.NoError(err)

	tmp := filepath.Join(home, "tmp-pickloader-test.txt")
	t.Cleanup(func() { _ = os.Remove(tmp) })
	assert.NoError(os.WriteFile(tmp, []byte("home-file"), 0o644))

	ld, err := pickLoader("~/tmp-pickloader-test.txt")
	assert.NoError(err)

	out, err := ld.Load(ctx)
	assert.NoError(err)
	assert.Equal("home-file", out)
}

func TestPickLoader_Shell(t *testing.T) {
	ctx := context.Background()
	assert := assert.New(t)

	ld, err := pickLoader("sh:echo shell works")
	assert.NoError(err)

	out, err := ld.Load(ctx)
	assert.NoError(err)
	assert.Equal("shell works\n", out)
}
