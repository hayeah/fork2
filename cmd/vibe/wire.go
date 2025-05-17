//go:build wireinject

package main

import (
	"github.com/google/wire"
	"github.com/hayeah/fork2/render"
)

func BuildOutPipeline(root string, args OutCmd) (*OutPipeline, error) {
	wire.Build(
		ProvideCounter,
		ProvideMetrics,
		ProvideFSList,
		ProvideResolver,
		ProvideRenderer,
		wire.Bind(new(RendererService), new(*render.Renderer)),
		ProvideDirectoryTreeService,
		wire.Bind(new(DirectoryTreeService), new(*DirectoryTree)),
		ProvideFileMapService,
		wire.Bind(new(FileMapService), new(*FileMapWriter)),
		ProvideContentLoader,
		wire.Struct(new(OutPipeline), "DT", "Renderer", "FileMap", "Metrics", "Loader"),
	)
	return nil, nil
}
