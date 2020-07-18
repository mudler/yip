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
	"errors"
	"fmt"
	"net/url"
	"os"

	"github.com/twpayne/go-vfs"

	"github.com/mudler/yip/pkg/executor"
	"github.com/mudler/yip/pkg/schema"
	"github.com/spf13/cobra"
)

var entityFile string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "yip",
	Short: "Modern go system configurator",
	Long: `Eapply loads distro-agnostic yaml in a cloud-init and applies them.

For example:

	$> yip -s initramfs https://<yip.yaml>
	$> yip -s initramfs <yip.yaml>
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		stage, _ := cmd.Flags().GetString("stage")
		exec, _ := cmd.Flags().GetString("executor")
		runner := executor.NewExecutor(exec)
		var config *schema.EApplyConfig
		var err error

		if len(args) != 1 {
			return errors.New("yip accepts only one argument a url or a path")
		}

		source := args[0]
		_, err = url.ParseRequestURI(source)
		if err != nil {
			config, err = schema.LoadFromFile(source)
		} else {
			config, err = schema.LoadFromUrl(source)
		}

		if err != nil {
			return err
		}

		return runner.Apply(stage, *config, vfs.OSFS)
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringP("executor", "e", "default", "Executor which applies the config")
	rootCmd.PersistentFlags().StringP("stage", "s", "default", "Stage to apply")
}
