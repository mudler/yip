/*
Copyright © 2022 - 2023 SUSE LLC

Copyright © 2015-2017 Docker, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package providers

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"github.com/mudler/yip/pkg/logger"
)

const (
	project  = "http://metadata.google.internal/computeMetadata/v1/project/"
	instance = "http://metadata.google.internal/computeMetadata/v1/instance/"
)

// ProviderGCP is the type implementing the Provider interface for GCP
type ProviderGCP struct {
	l logger.Interface
}

// NewGCP returns a new ProviderGCP
func NewGCP(l logger.Interface) *ProviderGCP {
	return &ProviderGCP{l}
}

func (p *ProviderGCP) String() string {
	return "GCP"
}

// Probe checks if we are running on GCP
func (p *ProviderGCP) Probe() bool {
	// Getting the hostname should always work...
	_, err := gcpGet(instance + "hostname")
	return err == nil
}

// Extract gets both the GCP specific and generic userdata
func (p *ProviderGCP) Extract() ([]byte, error) {
	// Get host name. This must not fail
	hostname, err := gcpGet(instance + "hostname")
	if err != nil {
		return nil, err
	}
	err = os.WriteFile(path.Join(ConfigPath, Hostname), hostname, 0644)
	if err != nil {
		return nil, fmt.Errorf("GCP: Failed to write hostname: %s", err)
	}

	if err := p.handleSSH(); err != nil {
		p.l.Errorf("GCP: Failed to get ssh data: %s", err)
	}

	// Generic userdata
	userData, err := gcpGet(instance + "attributes/user-data")
	if err != nil {
		p.l.Errorf("GCP: Failed to get user-data: %s", err)
		// This is not an error
		return nil, nil
	}
	return userData, nil
}

// gcpGet requests and extracts the requested URL
func gcpGet(url string) ([]byte, error) {
	var client = &http.Client{
		Timeout: time.Second * 2,
	}

	req, err := http.NewRequest("", url, nil)
	if err != nil {
		return nil, fmt.Errorf("GCP: http.NewRequest failed: %s", err)
	}
	req.Header.Set("Metadata-Flavor", "Google")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GCP: Could not contact metadata service: %s", err)
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("GCP: Status not ok: %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("GCP: Failed to read http response: %s", err)
	}
	return body, nil
}

// SSH keys:
// TODO also retrieve the instance keys and respect block
//
//	project keys see:
//	https://cloud.google.com/compute/docs/instances/ssh-keys
//
// The keys have usernames attached, but as a simplification
// we are going to add them all to one root file
// TODO split them into individual user files and make the ssh
//
//	container construct those users
func (p *ProviderGCP) handleSSH() error {
	sshKeys, err := gcpGet(project + "attributes/ssh-keys")
	if err != nil {
		return fmt.Errorf("Failed to get sshKeys: %s", err)
	}

	if _, err := os.Stat(path.Join(ConfigPath, SSH)); os.IsNotExist(err) {
		if err := os.Mkdir(path.Join(ConfigPath, SSH), 0755); err != nil {
			return fmt.Errorf("Failed to create %s: %s", SSH, err)
		}
	}

	rootKeys := ""
	for _, line := range strings.Split(string(sshKeys), "\n") {
		parts := strings.SplitN(line, ":", 2)
		// ignoring username for now
		if len(parts) == 2 {
			rootKeys = rootKeys + parts[1] + "\n"
		}
	}
	err = os.WriteFile(path.Join(ConfigPath, SSH, "authorized_keys"), []byte(rootKeys), 0600)
	if err != nil {
		return fmt.Errorf("Failed to write ssh keys: %s", err)
	}
	return nil
}
