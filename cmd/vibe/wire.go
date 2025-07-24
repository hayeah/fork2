//go:build wireinject

package main

import (
	"github.com/google/wire"
)

func BuildOutPipeline(root string, args OutCmd) (*OutPipeline, error) {
	wire.Build(
		ProvideAppEnv,
		ProvideRootFS,
		ProvideTemplate,
		ProvideCounter,
		ProvideMetrics,
		ProvideFSList,
		ProvideResolver,
		ProvideRenderer,
		ProvideDirectoryTreeService,
		ProvideFileMapService,
		ProvideContentLoader,
		wire.Struct(new(OutPipeline), "DT", "Renderer", "FileMap", "Metrics", "Loader", "Env", "Template"),
	)
	return nil, nil
}
