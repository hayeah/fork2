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

// DefaultContentLoader implements ContentLoader using render.LoadContentSources.
type DefaultContentLoader struct{}

func (DefaultContentLoader) LoadSources(ctx context.Context, specs []string) (string, error) {
    return render.LoadContentSources(ctx, specs)
}

func ProvideContentLoader() ContentLoader { return DefaultContentLoader{} }

// ProvideDirectoryTreeService constructs a DirectoryTree for the given root.
func ProvideDirectoryTreeService(root string) (*DirectoryTree, error) {
    return NewDirectoryTree(root), nil
}

// ProvideMetrics constructs OutputMetrics with the given counter.
func ProvideMetrics(counter metrics.Counter) *metrics.OutputMetrics {
    return metrics.NewOutputMetrics(counter, runtime.NumCPU())
}

func ProvideFileMapService(root string, m *metrics.OutputMetrics) *FileMapWriter {
    return NewWriteFileMap(root, m)
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
func ProvideFSList(root string) ([]fs.FS, error) {
    partials := []fs.FS{os.DirFS(root)}

    if env := os.Getenv("VIBE_PROMPTS"); env != "" {
        for _, dir := range strings.Split(env, string(os.PathListSeparator)) {
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
