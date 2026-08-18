package tools

import (
	"github.com/BlueDoraemon/kite-core/internal/core"
)

func init() {
	core.RegisterBuiltins(func(dir string) []core.Tool {
		set := &Set{Dir: dir}
		return set.All()
	})
}
