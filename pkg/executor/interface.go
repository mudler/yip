package executor

import (
	"strings"

	"github.com/twpayne/go-vfs"

	"github.com/mudler/yip/pkg/schema"
)

type Executor interface {
	Apply(string, schema.EApplyConfig, vfs.FS) error
}

func NewExecutor(s string) Executor {
	switch strings.ToLower(s) {
	default:
		return &DefaultExecutor{}
	}
}
