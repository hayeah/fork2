// File: render/contentloader.go
//
// Package render provides template rendering capabilities with support for
// content loading from a variety of “schemes” (stdin, file paths, clipboard,
// HTTP, literal text, …).  This rewrite introduces a clearer parsing
// algorithm and explicit factory functions for every built-in scheme.
package render

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/atotto/clipboard"
)

/* -------------------------------------------------------------------------- */
/*                              Loader interface                              */
/* -------------------------------------------------------------------------- */

// ContentLoader loads one logical “chunk” of text for a `--content` argument.
type ContentLoader interface {
	Load(ctx context.Context) (string, error)
}

// LoaderFactory turns a raw argument into a ContentLoader.
type LoaderFactory func(arg string) (ContentLoader, error)

/* -------------------------------------------------------------------------- */
/*                        Scheme registry (pluggable)                         */
/* -------------------------------------------------------------------------- */

var loaderRegistry = map[string]LoaderFactory{}

// RegisterScheme installs a factory under one or more scheme names.  The first
// name is considered canonical; the others are treated as aliases.
func RegisterScheme(factory LoaderFactory, names ...string) {
	for _, n := range names {
		loaderRegistry[strings.ToLower(n)] = factory
	}
}

/* -------------------------------------------------------------------------- */
/*                        Built-in loader implementations                     */
/* -------------------------------------------------------------------------- */

// ─── Stdin ────────────────────────────────────────────────────────────────────

type StdinLoader struct {
	Reader io.Reader // For testing, defaults to os.Stdin if nil
}

func (l *StdinLoader) Load(ctx context.Context) (string, error) {
	reader := l.Reader
	if reader == nil {
		reader = os.Stdin
	}
	b, err := io.ReadAll(reader)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// ─── Clipboard ───────────────────────────────────────────────────────────────

type ClipboardLoader struct{}

func (l *ClipboardLoader) Load(ctx context.Context) (string, error) {
	return clipboard.ReadAll()
}

func clipboardFactory(arg string) (ContentLoader, error) { return &ClipboardLoader{}, nil }

// ─── Literal text ────────────────────────────────────────────────────────────

type LiteralLoader struct{ Text string }

func (l *LiteralLoader) Load(ctx context.Context) (string, error) { return l.Text, nil }

func literalFactory(arg string) (ContentLoader, error) {
	// Trim  "<scheme>:" prefix before handing the body to the loader.
	if i := strings.IndexRune(arg, ':'); i >= 0 {
		arg = arg[i+1:]
	}
	return &LiteralLoader{Text: arg}, nil
}

// ─── File ────────────────────────────────────────────────────────────────────

type FileLoader struct{ Path string }

func (l *FileLoader) Load(ctx context.Context) (string, error) {
	b, err := os.ReadFile(l.Path)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func fileFactory(arg string) (ContentLoader, error) {
	u, _ := url.Parse(arg) // error ignored – arg may be a bare path
	path := u.Path

	// “~” expansion
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			path = filepath.Join(home, path[2:])
		}
	}

	return &FileLoader{Path: path}, nil
}

// ─── HTTP / HTTPS ────────────────────────────────────────────────────────────

type HTTPLoader struct{ URL string }

func (l *HTTPLoader) Load(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, l.URL, nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", errors.New("HTTP request failed: " + resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func httpFactory(arg string) (ContentLoader, error) { return &HTTPLoader{URL: arg}, nil }

// ─── Shell command ───────────────────────────────────────────────────────────

type ShellLoader struct {
	Command string
}

func (l *ShellLoader) Load(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "sh", "-c", l.Command)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("shell command failed: %w\nOutput:\n%s", err, output)
	}
	return string(output), nil
}

func shellFactory(arg string) (ContentLoader, error) {
	if i := strings.IndexRune(arg, ':'); i >= 0 {
		arg = arg[i+1:]
	}
	return &ShellLoader{Command: arg}, nil
}

/* -------------------------------------------------------------------------- */
/*                       Built-in scheme registrations                        */
/* -------------------------------------------------------------------------- */

func init() {
	// Clipboard aliases
	RegisterScheme(clipboardFactory, "clipboard", "paste")

	// Literal aliases  (support both legacy "literal:" and documented "text:")
	RegisterScheme(literalFactory, "text", "literal")

	// File (canonical “file”, but also consulted implicitly as a fallback)
	RegisterScheme(fileFactory, "file")

	// HTTP(S)
	RegisterScheme(httpFactory, "http", "https")

	// Shell commands
	RegisterScheme(shellFactory, "shell", "sh")
}

/* -------------------------------------------------------------------------- */
/*                          Public helper functions                           */
/* -------------------------------------------------------------------------- */

// LoadContentSources concatenates all resolved sources, inserting two newlines between
// each chunk (classic e-mail / Markdown style).
func LoadContentSources(ctx context.Context, specs []string) (string, error) {
	if len(specs) == 0 {
		return "", nil
	}

	var parts []string
	for _, raw := range specs {
		loader, err := pickLoader(raw)
		if err != nil {
			return "", err
		}
		text, err := loader.Load(ctx)
		if err != nil {
			return "", err
		}
		parts = append(parts, text)
	}
	return strings.Join(parts, "\n\n"), nil
}

/* -------------------------------------------------------------------------- */
/*                   Source-string → Loader dispatch logic                    */
/* -------------------------------------------------------------------------- */

// pickLoader decides which ContentLoader should handle one --content argument.
//
// Decision ladder (top-to-bottom):
//  1. "-"                       → stdin
//  2. "<scheme>:…"              → registry lookup on the part before the first ':'
//  3. bare alias ("clip", …)    → registry lookup on the whole word
//  4. existing file (~/ expanded) → file loader
//  5. none of the above         → error
func pickLoader(arg string) (ContentLoader, error) {
	// 1. stdin shorthand
	if arg == "-" {
		return &StdinLoader{}, nil
	}

	// 2.  "<scheme>:rest"
	if idx := strings.IndexRune(arg, ':'); idx > 0 {
		scheme := arg[:idx]
		if factory, ok := loaderRegistry[scheme]; ok { // scheme is known
			return factory(arg) // pass original arg *unchanged*
		}
	}

	// 3. bare alias  (no colon)
	if factory, ok := loaderRegistry[strings.ToLower(arg)]; ok {
		return factory("") // for bare scheme, pass empty string as arg
	}

	// 4. Treat as a file path  (~/ expansion, then os.Stat)
	path := arg
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			path = filepath.Join(home, path[2:])
		}
	}

	if _, err := os.Stat(path); err == nil {
		return &FileLoader{Path: path}, nil
	}

	// 5. Nothing matched – report a clean error.
	return nil, fmt.Errorf("unrecognised content source: %q", arg)
}
