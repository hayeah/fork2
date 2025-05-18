package main

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/hayeah/fork2/internal/metrics"
	"github.com/hayeah/fork2/render"
)

//go:embed templates
var systemTemplatesFS embed.FS

// RootPath is the path from which prompts and files are loaded.
type RootPath string

// WorkingDirectory is the absolute directory vibe was run from.
type WorkingDirectory string

// AppEnv groups runtime environment values used by providers.
type AppEnv struct {
	RootPath         RootPath
	WorkingDirectory WorkingDirectory
	DataPairs        []string
}

// DefaultContentLoader implements ContentLoader using render.LoadContentSources.
type DefaultContentLoader struct{}

func (DefaultContentLoader) LoadSources(ctx context.Context, specs []string) (string, error) {
	return render.LoadContentSources(ctx, specs)
}

func ProvideContentLoader() ContentLoader { return DefaultContentLoader{} }

// ProvideAppEnv builds the runtime environment used by other providers.
func ProvideAppEnv(root string, args OutCmd) (*AppEnv, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	return &AppEnv{
		RootPath:         RootPath(root),
		WorkingDirectory: WorkingDirectory(abs),
		DataPairs:        args.Data,
	}, nil
}

// ProvideDirectoryTreeService constructs a DirectoryTree for the given root.
func ProvideDirectoryTreeService(env *AppEnv) (*DirectoryTree, error) {
	return NewDirectoryTree(string(env.RootPath)), nil
}

// ProvideMetrics constructs OutputMetrics with the given counter.
func ProvideMetrics(counter metrics.Counter) *metrics.OutputMetrics {
	return metrics.NewOutputMetrics(counter, runtime.NumCPU())
}

func ProvideFileMapService(env *AppEnv, m *metrics.OutputMetrics) *FileMapWriter {
	return NewWriteFileMap(string(env.RootPath), m)
}

func ProvideRenderer(resolver *render.Resolver, m *metrics.OutputMetrics) *render.Renderer {
	return render.NewRenderer(resolver, m)
}

func ProvideResolver(fsList []fs.FS) *render.Resolver {
	return render.NewResolver(fsList...)
}

func ProvideCounter(args OutCmd) (metrics.Counter, error) {
	switch args.TokenEstimator {
	case "tiktoken":
		c, err := metrics.NewTiktokenCounter("gpt-3.5-turbo")
		if err != nil {
			return &metrics.SimpleCounter{}, nil
		}
		return c, nil
	default:
		return &metrics.SimpleCounter{}, nil
	}
}

// ProvideFSList builds the filesystem stack for templates.
func ProvideFSList(env *AppEnv) ([]fs.FS, error) {
	root := string(env.RootPath)
	partials := []fs.FS{os.DirFS(root)}

	if envVar := os.Getenv("VIBE_PROMPTS"); envVar != "" {
		for _, dir := range strings.Split(envVar, string(os.PathListSeparator)) {
			dir = strings.TrimSpace(dir)
			if dir == "" {
				continue
			}
			if fi, err := os.Stat(dir); err == nil && fi.IsDir() {
				partials = append(partials, os.DirFS(dir))
			}
		}
	}

	if home, err := os.UserHomeDir(); err == nil {
		userVibe := filepath.Join(home, ".vibe")
		if fi, err := os.Stat(userVibe); err == nil && fi.IsDir() {
			partials = append(partials, os.DirFS(userVibe))
		}
	}

	systemFS, err := fs.Sub(systemTemplatesFS, "templates")
	if err != nil {
		return nil, fmt.Errorf("failed to create system prompts fs: %v", err)
	}
	partials = append(partials, systemFS)

	return partials, nil
}
