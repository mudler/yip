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
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/hashicorp/go-multierror"
	"github.com/mudler/yip/pkg/executor"
	"github.com/mudler/yip/pkg/schema"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/twpayne/go-vfs"
)

var entityFile string

func fail(s string) {
	log.Error(s)
	os.Exit(1)
}
func checkErr(err error) {
	if err != nil {
		fail("fatal error: " + err.Error())
	}
}

func init() {
	switch strings.ToLower(os.Getenv("LOGLEVEL")) {
	case "error":
		log.SetLevel(log.ErrorLevel)
	case "warning":
		log.SetLevel(log.WarnLevel)
	case "debug":
		log.SetLevel(log.DebugLevel)
	default:
		log.SetLevel(log.InfoLevel)
	}
}

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "yip",
	Short: "Modern go system configurator",
	Long: `yip loads cloud-init style yamls and applies them in the system.

For example:

	$> yip -s initramfs https://<yip.yaml> <definition.yaml> ...
	$> yip -s initramfs <yip.yaml> <yip2.yaml> ...
	$> cat def.yaml | yip -
`,
	Run: func(cmd *cobra.Command, args []string) {
		stage, _ := cmd.Flags().GetString("stage")
		exec, _ := cmd.Flags().GetString("executor")
		runner := executor.NewExecutor(exec)
		fromStdin := len(args) == 1 && args[0] == "-"

		var config *schema.YipConfig
		var errs error

		if len(args) == 0 {
			fail("yip needs at least one path or url as argument")
		}

		// Read yamls from STDIN
		if fromStdin {
			str, err := ioutil.ReadAll(os.Stdin)
			checkErr(err)

			config, err = schema.LoadFromYaml(str)
			checkErr(err)

			err = runner.Apply(stage, *config, vfs.OSFS)
			checkErr(err)
		}

		for _, source := range args {

			// Load yamls in a directory
			if f, err := vfs.OSFS.Stat(source); err == nil && f.IsDir() {
				err := filepath.Walk(source,
					func(path string, info os.FileInfo, err error) error {
						if err != nil {
							return err
						}
						if path == source {
							return nil
						}
						// Process only files
						if info.IsDir() {
							return nil
						}
						ext := filepath.Ext(path)
						if ext != ".yaml" && ext != ".yml" {
							return nil
						}
						config, err = schema.LoadFromFile(path)
						if err != nil {
							errs = multierror.Append(errs, err)
							return nil
						}
						log.Info("Executing", path)
						if err = runner.Apply(stage, *config, vfs.OSFS); err != nil {
							errs = multierror.Append(errs, err)
							return nil
						}

						return nil
					})
				if err != nil {
					errs = multierror.Append(errs, err)
				}

				continue
			}

			// Parse urls/file
			_, err := url.ParseRequestURI(source)
			if err != nil {
				config, err = schema.LoadFromFile(source)
			} else {
				config, err = schema.LoadFromUrl(source)
			}

			if err != nil {
				errs = multierror.Append(errs, err)
				continue
			}

			if err = runner.Apply(stage, *config, vfs.OSFS); err != nil {
				errs = multierror.Append(errs, err)
				continue
			}
		}
		checkErr(errs)
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	checkErr(err)
}

func init() {
	rootCmd.PersistentFlags().StringP("executor", "e", "default", "Executor which applies the config")
	rootCmd.PersistentFlags().StringP("stage", "s", "default", "Stage to apply")
}
