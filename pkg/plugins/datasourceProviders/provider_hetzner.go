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
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/mudler/yip/pkg/logger"
	"gopkg.in/yaml.v3"
)

// hetznerBaseURL is Hetzner's native metadata API. This provider used to read
// the EC2-compatible routes (/latest/meta-data/, /latest/user-data), which
// Hetzner removed on 2026-08-01, so Probe() failed on every Hetzner server and
// no cloud-config was applied at all.
// See https://docs.hetzner.cloud/changelog#2026-08-01-removed-metadata-routes
const hetznerBaseURL = "http://169.254.169.254/hetzner/v1"

// hetznerMetadata is the part of the metadata document this provider uses.
//
// The document is YAML, not JSON: instance-id is a bare integer and
// public-keys is a block sequence. That is why the whole document is fetched
// and parsed once, the way cloud-init's Hetzner datasource does, rather than
// requesting one key per field.
type hetznerMetadata struct {
	Hostname   string   `yaml:"hostname"`
	InstanceID int64    `yaml:"instance-id"`
	PublicIPv4 string   `yaml:"public-ipv4"`
	LocalIPv4  string   `yaml:"local-ipv4"`
	PublicKeys []string `yaml:"public-keys"`
}

// ProviderHetzner is the type implementing the Provider interface for Hetzner
type ProviderHetzner struct {
	l         logger.Interface
	client    *http.Client
	baseURL   string
	outputDir string
}

// HetznerOption configures a ProviderHetzner. Production code uses the
// defaults; tests use these to point the provider at a local server and a
// temporary output directory.
type HetznerOption func(*ProviderHetzner)

// WithHetznerBaseURL overrides the metadata API root (default
// http://169.254.169.254/hetzner/v1).
func WithHetznerBaseURL(u string) HetznerOption {
	return func(p *ProviderHetzner) { p.baseURL = strings.TrimRight(u, "/") }
}

// WithHetznerOutputDir overrides where the provider writes hostname, network
// facts and SSH keys (default ConfigPath).
func WithHetznerOutputDir(d string) HetznerOption {
	return func(p *ProviderHetzner) { p.outputDir = d }
}

// NewHetzner returns a new ProviderHetzner
func NewHetzner(l logger.Interface, opts ...HetznerOption) *ProviderHetzner {
	p := &ProviderHetzner{
		l:         l,
		client:    &http.Client{Timeout: time.Second * 2},
		baseURL:   hetznerBaseURL,
		outputDir: ConfigPath,
	}
	for _, o := range opts {
		o(p)
	}
	return p
}

func (p *ProviderHetzner) String() string {
	return "Hetzner"
}

// Probe checks if we are running on Hetzner
func (p *ProviderHetzner) Probe() bool {
	_, err := p.metadata()
	if err != nil {
		p.l.Debugf("Hetzner: probe failed: %s", err)
		return false
	}
	return true
}

// Extract gets both the Hetzner specific and generic userdata
func (p *ProviderHetzner) Extract() ([]byte, error) {
	md, err := p.metadata()
	if err != nil {
		return nil, err
	}

	if err := os.WriteFile(path.Join(p.outputDir, Hostname), []byte(md.Hostname), 0644); err != nil {
		return nil, fmt.Errorf("Hetzner: Failed to write hostname: %s", err)
	}

	p.write("public_ipv4", md.PublicIPv4)
	// Empty unless the server is attached to a private network.
	p.write("local_ipv4", md.LocalIPv4)
	if md.InstanceID != 0 {
		p.write("instance_id", strconv.FormatInt(md.InstanceID, 10))
	}

	if err := p.writeSSHKeys(md.PublicKeys); err != nil {
		p.l.Errorf("Hetzner: Failed to get ssh data: %s", err)
	}

	// Generic userdata
	userData, err := p.get("/userdata")
	if err != nil {
		p.l.Errorf("Hetzner: Failed to get user-data: %s", err)
		// This is not an error
		return nil, nil
	}
	return userData, nil
}

// metadata fetches and parses the metadata document. A document without a
// hostname is treated as not being Hetzner's, so that a foreign service
// answering on the link-local address cannot be mistaken for one.
func (p *ProviderHetzner) metadata() (hetznerMetadata, error) {
	body, err := p.get("/metadata")
	if err != nil {
		return hetznerMetadata{}, err
	}

	var md hetznerMetadata
	if err := yaml.Unmarshal(body, &md); err != nil {
		return hetznerMetadata{}, fmt.Errorf("Hetzner: Failed to parse metadata: %s", err)
	}
	if md.Hostname == "" {
		return hetznerMetadata{}, errors.New("Hetzner: metadata carries no hostname")
	}
	return md, nil
}

// write stores a single metadata value in the output directory, skipping
// values the metadata service left empty.
func (p *ProviderHetzner) write(fileName, value string) {
	if value == "" {
		return
	}
	if err := os.WriteFile(path.Join(p.outputDir, fileName), []byte(value), 0644); err != nil {
		p.l.Errorf("Hetzner: Failed to write %s:%s %s", fileName, value, err)
	}
}

// writeSSHKeys stores the server's Hetzner Cloud SSH keys where
// datasource.go picks them up. Nothing is written when the server has none,
// so an empty key set does not look like a provisioned one.
func (p *ProviderHetzner) writeSSHKeys(keys []string) error {
	if len(keys) == 0 {
		return nil
	}

	if err := os.MkdirAll(path.Join(p.outputDir, SSH), 0755); err != nil {
		return fmt.Errorf("Failed to create %s: %s", SSH, err)
	}

	var b strings.Builder
	for _, key := range keys {
		b.WriteString(key)
		b.WriteString("\n")
	}
	if err := os.WriteFile(path.Join(p.outputDir, SSH, "authorized_keys"), []byte(b.String()), 0600); err != nil {
		return fmt.Errorf("Failed to write ssh keys: %s", err)
	}
	return nil
}

// get requests and extracts a path below the metadata API root
func (p *ProviderHetzner) get(urlPath string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, p.baseURL+urlPath, nil)
	if err != nil {
		return nil, fmt.Errorf("Hetzner: http.NewRequest failed: %s", err)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Hetzner: Could not contact metadata service: %s", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Hetzner: Status not ok: %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("Hetzner: Failed to read http response: %s", err)
	}
	return body, nil
}
