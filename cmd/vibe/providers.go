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

	"github.com/google/wire"
	"github.com/hayeah/fork2/internal/metrics"
	"github.com/hayeah/fork2/render"
	"github.com/hayeah/fork2/vibe"
)

//go:embed templates
var systemTemplatesFS embed.FS

// DefaultContentLoader implements ContentLoader using render.LoadContentSources.
type DefaultContentLoader struct{}

func (DefaultContentLoader) LoadSources(ctx context.Context, specs []string) (string, error) {
	return render.LoadContentSources(ctx, specs)
}

func ProvideContentLoader() ContentLoader { return DefaultContentLoader{} }

// ProvideRootFS creates a filesystem abstraction for the repo root
func ProvideRootFS(config *vibe.AppConfig) (fs.FS, error) {
	return os.DirFS(config.RootPath), nil
}

// ProvideDirectoryTreeService constructs a DirectoryTree for the given root.
func ProvideDirectoryTreeService(config *vibe.AppConfig) (*DirectoryTree, error) {
	return NewDirectoryTree(config.RootPath), nil
}

// ProvideMetrics constructs OutputMetrics with the given counter.
func ProvideMetrics(counter metrics.Counter) *metrics.OutputMetrics {
	return metrics.NewOutputMetrics(counter, runtime.NumCPU())
}

func ProvideFileMapService(config *vibe.AppConfig, rfs fs.FS, m *metrics.OutputMetrics) *FileMapWriter {
	return NewWriteFileMap(rfs, config.RootPath, m)
}

func ProvideRenderer(resolver *render.Resolver, m *metrics.OutputMetrics) *render.Renderer {
	return render.NewRenderer(resolver, m)
}

// ProvideEntryTemplate loads the entry template based on OutCmd arguments.
func ProvideEntryTemplate(args OutCmd, fsList []fs.FS) (*render.Template, error) {
	// decide which file the user wants to render
	templatePath := args.Template
	if templatePath == "" && args.Select != "" {
		templatePath = "files"
	}
	if templatePath == "" {
		return nil, nil // nothing to inspect
	}

	// Create a resolver without mode to load the template
	resolver := render.NewResolver("", fsList...)
	tmpl, err := resolver.LoadTemplate(templatePath, nil)
	if err != nil {
		return nil, err
	}

	return tmpl, nil
}

func ProvideResolver(config *vibe.AppConfig, fsList []fs.FS) *render.Resolver {
	return render.NewResolver(config.Mode, fsList...)
}

func ProvideCounter(args OutCmd) (metrics.Counter, error) {
	switch args.TokenEstimator {
	case "tiktoken":
		c, err := metrics.NewTiktokenCounter("gpt-3.5-turbo")
		if err != nil {
			return &metrics.SimpleCounter{}, nil
		}
		return c, nil
	case "simple":
		return &metrics.SimpleCounter{}, nil
	default:
		return nil, fmt.Errorf("unknown token estimator: %s", args.TokenEstimator)
	}
}

// ProvideFSList builds the filesystem stack for templates.
// Search precedence: cwd, template paths, VIBE_PROMPTS, ~/.vibe, built-ins
func ProvideFSList(args OutCmd) ([]fs.FS, error) {
	cwd, _ := os.Getwd()

	// Start with current working directory
	partials := []fs.FS{&render.CwdFS{Base: cwd}}

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

// ProvideAppConfig merges template frontmatter with command-line arguments to create the final AppConfig.
func ProvideAppConfig(tmpl *render.Template, args OutCmd) (*vibe.AppConfig, error) {
	// Resolve root in order: args.Root, template root (if any), or cwd
	var root string
	if args.Root != "" {
		root = args.Root
	} else if tmpl != nil && tmpl.Config != nil && tmpl.Config.Root != "" {
		root = tmpl.Config.Root
	} else {
		var err error
		root, err = os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("failed to get current directory: %w", err)
		}
	}

	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}

	// Start with empty config
	config := &vibe.AppConfig{
		RootPath:         root,
		WorkingDirectory: abs,
		DataPairs:        args.Data,
	}

	// Merge template frontmatter if available
	if tmpl != nil && tmpl.Config != nil {
		config.Root = tmpl.Config.Root
		config.TemplatePaths = tmpl.Config.TemplatePaths
		config.Content = tmpl.Config.Content
		config.Mode = tmpl.Config.Mode
		config.Layout = tmpl.Config.Layout
		config.Select = tmpl.Config.Select
		config.Dirtree = tmpl.Config.Dirtree
		config.Before = tmpl.Config.Before
		config.After = tmpl.Config.After
	}

	// Override with command-line arguments
	if args.Mode != "" {
		config.Mode = args.Mode
	}
	if args.Layout != "" {
		config.Layout = args.Layout
	}
	if args.Select != "" {
		config.Select = args.Select
	}
	if args.SelectDirTree != "" {
		config.Dirtree = args.SelectDirTree
	}

	// Apply default layout if needed
	if config.Layout == "" && config.Select != "" {
		config.Layout = "files"
	}

	return config, nil
}

var wires = wire.NewSet(ProvideFSList,
	ProvideEntryTemplate,
	ProvideAppConfig,
	ProvideRootFS,
	ProvideCounter,
	ProvideMetrics,
	ProvideResolver,
	ProvideRenderer,
	ProvideDirectoryTreeService,
	ProvideFileMapService,
	ProvideContentLoader,
	wire.Struct(new(OutPipeline), "DT", "Renderer", "FileMap", "Metrics", "Loader", "Config", "Template"),
	wire.Struct(new(OutRunner), "Args", "Pipeline"),
)
