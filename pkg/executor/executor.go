package executor

import (
	"strings"

	"github.com/mudler/yip/pkg/plugins"
	"github.com/twpayne/go-vfs"

	"github.com/mudler/yip/pkg/schema"
)

// Executor an executor applies a yip config
type Executor interface {
	Apply(string, schema.YipConfig, vfs.FS, plugins.Console) error
	Run(string, vfs.FS, plugins.Console, ...string) error
	Plugins([]Plugin)
	Conditionals([]Plugin)
	Modifier(m schema.Modifier)
}

type Plugin func(schema.Stage, vfs.FS, plugins.Console) error

// NewExecutor returns an executor from the stringified version of it.
func NewExecutor(s string) Executor {

	switch strings.ToLower(s) {
	default:
		return &DefaultExecutor{
			conditionals: []Plugin{
				plugins.NodeConditional,
				plugins.IfConditional,
			},
			plugins: []Plugin{
				plugins.DNS,
				plugins.Entities,
				plugins.EnsureDirectories,
				plugins.EnsureFiles,
				plugins.Commands,
				plugins.DeleteEntities,
				plugins.Hostname,
				plugins.Sysctl,
				plugins.SSH,
				plugins.User,
				plugins.LoadModules,
				plugins.Timesyncd,
				plugins.Systemctl,
				plugins.Environment,
				plugins.SystemdFirstboot,
				plugins.DataSources,
			},
		}
	}
}
