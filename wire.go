//go:build wireinject

package fork2

import (
	"github.com/google/wire"
	"github.com/hayeah/goo"
)

func InitMain() (goo.Main, error) {
	panic(wire.Build(Wires))
}
