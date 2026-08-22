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
	"time"

	"github.com/mudler/yip/pkg/logger"
)

// Hetzner native metadata API endpoints.
// The EC2-compatible routes (/latest/meta-data/, /latest/user-data) were removed
// by Hetzner on 2026-08-01. See https://docs.hetzner.cloud/changelog#2026-08-01-removed-metadata-routes
const (
	hetznerMetaDataURL = "http://169.254.169.254/hetzner/v1/metadata/"
	hetznerUserDataURL = "http://169.254.169.254/hetzner/v1/userdata"
)

// ProviderHetzner is the type implementing the Provider interface for Hetzner
type ProviderHetzner struct {
	l logger.Interface
}

// NewHetzner returns a new ProviderHetzner
func NewHetzner(l logger.Interface) *ProviderHetzner {
	return &ProviderHetzner{l}
}

func (p *ProviderHetzner) String() string {
	return "Hetzner"
}

// Probe checks if we are running on Hetzner
func (p *ProviderHetzner) Probe() bool {
	_, err := hetznerGet(hetznerMetaDataURL + "hostname")
	return err == nil
}

// Extract gets both the Hetzner specific and generic userdata
func (p *ProviderHetzner) Extract() ([]byte, error) {
	// Get host name. This must not fail
	hostname, err := hetznerGet(hetznerMetaDataURL + "hostname")
	if err != nil {
		return nil, err
	}
	err = os.WriteFile(path.Join(ConfigPath, Hostname), hostname, 0644)
	if err != nil {
		return nil, fmt.Errorf("Hetzner: Failed to write hostname: %s", err)
	}

	// public ipv4
	p.hetznerMetaGet("public-ipv4", "public_ipv4", 0644)

	// instance-id
	p.hetznerMetaGet("instance-id", "instance_id", 0644)

	// Generic userdata
	userData, err := hetznerGet(hetznerUserDataURL)
	if err != nil {
		p.l.Errorf("Hetzner: Failed to get user-data: %s", err)
		// This is not an error
		return nil, nil
	}
	return userData, nil
}

// lookup a value (lookupName) in hetzner metaservice and store in given fileName
func (p *ProviderHetzner) hetznerMetaGet(lookupName string, fileName string, fileMode os.FileMode) {
	if lookupValue, err := hetznerGet(hetznerMetaDataURL + lookupName); err == nil {
		err = os.WriteFile(path.Join(ConfigPath, fileName), lookupValue, fileMode)
		if err != nil {
			p.l.Errorf("Hetzner: Failed to write %s:%s %s", fileName, lookupValue, err)
		}
	} else {
		p.l.Errorf("Hetzner: Failed to get %s: %s", lookupName, err)
	}
}

// hetznerGet requests and extracts the requested URL
func hetznerGet(url string) ([]byte, error) {
	var client = &http.Client{
		Timeout: time.Second * 2,
	}

	req, err := http.NewRequest("", url, nil)
	if err != nil {
		return nil, fmt.Errorf("Hetzner: http.NewRequest failed: %s", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Hetzner: Could not contact metadata service: %s", err)
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Hetzner: Status not ok: %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("Hetzner: Failed to read http response: %s", err)
	}
	return body, nil
}
