//go:build wireinject

package main

import (
	"github.com/google/wire"
)

func initOutRunner(args OutCmd) (*OutRunner, error) {
	wire.Build(wires)
	return nil, nil
}
