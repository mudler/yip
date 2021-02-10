package executor

import (
	"strings"

	"github.com/twpayne/go-vfs"

	"github.com/mudler/yip/pkg/schema"
)

// Executor an executor applies a yip config
type Executor interface {
	Apply(string, schema.YipConfig, vfs.FS) error
	Walk(string, []string, vfs.FS) error
}

// NewExecutor returns an executor from the stringified version of it.
func NewExecutor(s string) Executor {
	switch strings.ToLower(s) {
	default:
		return &DefaultExecutor{}
	}
}
