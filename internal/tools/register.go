package tools

import (
	"github.com/BlueDoraemon/kite-core/internal/core"
)

func init() {
	core.RegisterBuiltins(func(dir string, store core.SessionStore) []core.Tool {
		set := &Set{Dir: dir, Store: store}
		return set.All()
	})
}
