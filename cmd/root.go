// Copyright © 2020 Ettore Di Giacinto <mudler@gentoo.org>
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/mudler/yip/pkg/console"
	"github.com/mudler/yip/pkg/executor"
	"github.com/mudler/yip/pkg/logger"
	"github.com/mudler/yip/pkg/schema"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/twpayne/go-vfs"
)

const (
	CLIVersion = "0.11.0"
)

// Build time and commit information.
//
// ⚠️ WARNING: should only be set by "-ldflags".
var (
	BuildTime   string
	BuildCommit string
)

func initLogger() logger.Interface {
	ll := log.New()
	switch strings.ToLower(os.Getenv("LOGLEVEL")) {
	case "error":
		ll.SetLevel(log.ErrorLevel)
	case "warning":
		ll.SetLevel(log.WarnLevel)
	case "debug":
		ll.SetLevel(log.DebugLevel)
	default:
		ll.SetLevel(log.InfoLevel)
	}
	return ll
}

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:     "yip",
	Short:   "Modern go system configurator",
	Version: fmt.Sprintf("%s-g%s %s", CLIVersion, BuildCommit, BuildTime),
	Long: `yip loads cloud-init style yamls and applies them in the system.

For example:

	$> yip -s initramfs https://<yip.yaml> /path/to/disk <definition.yaml> ...
	$> yip -s initramfs <yip.yaml> <yip2.yaml> ...
	$> cat def.yaml | yip -
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		stage, _ := cmd.Flags().GetString("stage")
		dot, _ := cmd.Flags().GetBool("dotnotation")

		ll := initLogger()
		runner := executor.NewExecutor(executor.WithLogger(ll))
		fromStdin := len(args) == 1 && args[0] == "-"

		ll.Infof("yip version %s", cmd.Version)
		if len(args) == 0 {
			ll.Fatal("yip needs at least one path or url as argument")
		}
		stdConsole := console.NewStandardConsole(console.WithLogger(ll))

		if dot {
			runner.Modifier(schema.DotNotationModifier)
		}

		if fromStdin {
			std, err := ioutil.ReadAll(os.Stdin)
			if err != nil {
				return err
			}

			args = []string{string(std)}
		}

		return runner.Run(stage, vfs.OSFS, stdConsole, args...)
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		log.Fatal(err)
	}
}

func init() {
	rootCmd.PersistentFlags().StringP("stage", "s", "default", "Stage to apply")
	rootCmd.PersistentFlags().BoolP("dotnotation", "d", false, "Parse input in dotnotation ( e.g. `stages.foo.name=..` ) ")
}
