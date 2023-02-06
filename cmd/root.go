//   Copyright 2020 Ettore Di Giacinto <mudler@mocaccino.org>
//
//   Licensed under the Apache License, Version 2.0 (the "License");
//   you may not use this file except in compliance with the License.
//   You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
//   Unless required by applicable law or agreed to in writing, software
//   distributed under the License is distributed on an "AS IS" BASIS,
//   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//   See the License for the specific language governing permissions and
//   limitations under the License.

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

// Build time and commit information.
//
// ⚠️ WARNING: should only be set by "-ldflags".
var (
	BuildTime   string
	BuildCommit string
	CLIVersion  string
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
		analyze, _ := cmd.Flags().GetBool("analyze")

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

		if analyze {
			runner.Analyze(stage, vfs.OSFS, stdConsole, args...)
			return nil
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
	rootCmd.PersistentFlags().BoolP("analyze", "a", false, "Analize execution graph")
	rootCmd.PersistentFlags().BoolP("dotnotation", "d", false, "Parse input in dotnotation ( e.g. `stages.foo.name=..` ) ")
}
