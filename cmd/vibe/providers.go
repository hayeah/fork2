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
	Mode             string
}

// DefaultContentLoader implements ContentLoader using render.LoadContentSources.
type DefaultContentLoader struct{}

func (DefaultContentLoader) LoadSources(ctx context.Context, specs []string) (string, error) {
	return render.LoadContentSources(ctx, specs)
}

func ProvideContentLoader() ContentLoader { return DefaultContentLoader{} }

// ProvideRootFS creates a filesystem abstraction for the repo root
func ProvideRootFS(env *AppEnv) (fs.FS, error) {
	return os.DirFS(string(env.RootPath)), nil
}

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
		Mode:             args.Mode,
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

func ProvideFileMapService(env *AppEnv, rfs fs.FS, m *metrics.OutputMetrics) *FileMapWriter {
	return NewWriteFileMap(rfs, string(env.RootPath), m)
}

func ProvideRenderer(resolver *render.Resolver, m *metrics.OutputMetrics) *render.Renderer {
	return render.NewRenderer(resolver, m)
}

func ProvideTemplate(env *AppEnv, resolver *render.Resolver, args OutCmd, fsList []fs.FS) (*render.Template, error) {
	// decide which file the user wants to render
	templatePath := args.Template
	if templatePath == "" && args.Select != "" {
		templatePath = "files"
	}
	if templatePath == "" {
		return nil, nil // nothing to inspect
	}

	// templatePath = strings.TrimPrefix(templatePath, "./")

	// Create a temporary resolver to load the template
	// tempResolver := render.NewResolver(env.Mode, fsList...)
	// renderer := render.NewRenderer(tempResolver, nil)
	tmpl, err := resolver.LoadTemplate(templatePath, nil)
	if err != nil {
		return nil, err
	}

	if env.Mode == "" && tmpl.FrontMatter.Mode != "" {
		resolver.Mode = tmpl.FrontMatter.Mode
	}

	// Apply command-line overrides
	if args.Layout != "" {
		tmpl.FrontMatter.Layout = args.Layout
	}
	if args.Select != "" {
		tmpl.FrontMatter.Select = args.Select
	}
	if args.SelectDirTree != "" {
		tmpl.FrontMatter.Dirtree = args.SelectDirTree
	}

	if tmpl.FrontMatter.Layout == "" && tmpl.FrontMatter.Select != "" {
		tmpl.FrontMatter.Layout = "files"
	}

	return tmpl, nil
}

func ProvideResolver(env *AppEnv, fsList []fs.FS) *render.Resolver {
	// NOTE: ProvideTemplate will destructively update some of the resolver
	// properties depending on template metadata.
	mode := env.Mode
	return render.NewResolver(mode, fsList...)
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
func ProvideFSList(env *AppEnv, args OutCmd) ([]fs.FS, error) {
	root := string(env.RootPath)
	partials := []fs.FS{os.DirFS(root)}

	// Add any additional template paths from args
	for _, path := range args.TemplatePaths {
		if fi, err := os.Stat(path); err == nil && fi.IsDir() {
			partials = append(partials, os.DirFS(path))
		}
	}

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
