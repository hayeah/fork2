//go:build wireinject

package main

import "github.com/google/wire"

func BuildOutPipeline(root string, args OutCmd) (*OutPipeline, error) {
    wire.Build(
        ProvideCounter,
        ProvideMetrics,
        ProvideFSList,
        ProvideResolver,
        ProvideRenderer,
        ProvideDirectoryTreeService,
        ProvideFileMapService,
        ProvideContentLoader,
        wire.Struct(new(OutPipeline), "DT", "Renderer", "FileMap", "Metrics", "Loader"),
    )
    return nil, nil
}
