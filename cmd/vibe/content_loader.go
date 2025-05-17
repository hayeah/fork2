package main

import "context"

// ContentLoader loads text from a set of sources.
type ContentLoader interface {
    LoadSources(ctx context.Context, specs []string) (string, error)
}
