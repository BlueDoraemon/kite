package core

// installBuiltins is set by the tools package (via RegisterBuiltins) so that
// core does not import tools directly, avoiding an import cycle. When nil,
// NewSession leaves the tools list empty.
var installBuiltins func(dir string) []Tool

// RegisterBuiltins installs the built-in tool factory. It is called from the
// tools package's init.
func RegisterBuiltins(fn func(dir string) []Tool) {
	installBuiltins = fn
}

// builtinTools installs the built-in tools for a working directory.
func builtinTools(dir string) []Tool {
	if installBuiltins == nil {
		return nil
	}
	return installBuiltins(dir)
}
