//   Copyright 2021 Ettore Di Giacinto <mudler@mocaccino.org>
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
//go:build gitbinary && !nogit

package plugins

import (
	"os"
	"os/exec"
	"path/filepath"

	"github.com/mudler/yip/pkg/logger"
	"github.com/mudler/yip/pkg/schema"
	"github.com/mudler/yip/pkg/utils"
	"github.com/twpayne/go-vfs/v4"
)

func Git(l logger.Interface, s schema.Stage, fs vfs.FS, console Console) error {
	if s.Git.URL == "" {
		return nil
	}

	branch := "master"
	if s.Git.Branch != "" {
		branch = s.Git.Branch
	}

	path, err := fs.RawPath(s.Git.Path)
	if err != nil {
		return err
	}
	l.Infof("Cloning git repository '%s' into %s", s.Git.URL, path)

	// Helper to build git command
	buildGitCmd := func(args ...string) []string {
		cmd := []string{"git"}
		cmd = append(cmd, args...)
		return cmd
	}

	// Helper to run command
	runCmd := func(cmd []string, env []string) error {
		command := exec.Command(cmd[0], cmd[1:]...)
		command.Env = append(os.Environ(), env...)
		out, err := command.CombinedOutput()
		if err != nil {
			l.Errorf("Command failed: %s\nOutput: %s", cmd, out)
			return err
		}
		return nil
	}

	// Prepare authentication
	var env []string
	var keyFile string
	if s.Git.Auth.Username != "" && s.Git.Auth.Password != "" {
		// Use username/password
		env = append(env, "GIT_ASKPASS=true", "GIT_USERNAME="+s.Git.Auth.Username, "GIT_PASSWORD="+s.Git.Auth.Password)
	}
	if s.Git.Auth.PrivateKey != "" {
		// Write private key to temp file
		f, err := utils.WriteTempFile([]byte(s.Git.Auth.PrivateKey), "yip_gitkey_")
		if err != nil {
			return err
		}
		keyFile = f
		defer func() {
			_ = utils.RemoveFile(keyFile)
		}()
		env = append(env, "GIT_SSH_COMMAND=ssh -i "+keyFile)
		if s.Git.Auth.Insecure {
			env = append(env, "GIT_SSH_COMMAND=ssh -o StrictHostKeyChecking=no -i "+keyFile)
		}
	}

	if utils.Exists(filepath.Join(path, ".git")) {
		l.Info("Repository already exists, updating it")
		// git fetch and reset
		// Move to the repo path so commands are executed in there
		currentDir, err := os.Getwd()
		if err != nil {
			return err
		}
		if err := os.Chdir(path); err != nil {
			return err
		}
		defer os.Chdir(currentDir)
		cmd := buildGitCmd("fetch", "origin", branch)
		if err := runCmd(cmd, env); err != nil {
			return err
		}
		cmd = buildGitCmd("reset", "--hard", "origin/"+branch)
		if err := runCmd(cmd, env); err != nil {
			return err
		}
		if s.Git.BranchOnly {
			cmd = buildGitCmd("checkout", branch)
			if err := runCmd(cmd, env); err != nil {
				return err
			}
		}
		return nil
	}
	l.Infof("Cloning git repo '%s' to %s", branch, path)
	cmd := buildGitCmd("clone", "--branch", branch)
	if s.Git.BranchOnly {
		cmd = append(cmd, "--single-branch")
	}
	cmd = append(cmd, s.Git.URL, path)
	if err := runCmd(cmd, env); err != nil {
		return err
	}

	return nil
}
