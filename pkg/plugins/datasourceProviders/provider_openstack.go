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

// ProviderOpenstack is the type implementing the Provider interface for OpenStack
type ProviderOpenstack struct {
	l logger.Interface
}

// NewOpenstack returns a new ProviderOpenstack
func NewOpenstack(l logger.Interface) *ProviderOpenstack {
	return &ProviderOpenstack{l}
}

func (p *ProviderOpenstack) String() string {
	return "Openstack"
}

// Probe checks if we are running on OpenStack
func (p *ProviderOpenstack) Probe() bool {
	// Getting the hostname should always work...
	_, err := openstackGet(metaDataURL + "hostname")
	return err == nil
}

// Extract gets both the OpenStack specific and generic userdata
func (p *ProviderOpenstack) Extract() ([]byte, error) {
	// Get host name. This must not fail
	hostname, err := openstackGet(metaDataURL + "hostname")
	if err != nil {
		return nil, err
	}
	err = os.WriteFile(path.Join(ConfigPath, Hostname), hostname, 0644)
	if err != nil {
		return nil, fmt.Errorf("OpenStack: Failed to write hostname: %s", err)
	}

	// public ipv4
	p.openstackMetaGet("public-ipv4", "public_ipv4", 0644)

	// private ipv4
	p.openstackMetaGet("local-ipv4", "local_ipv4", 0644)

	// availability zone
	p.openstackMetaGet("placement/availability-zone", "availability_zone", 0644)

	// instance type
	p.openstackMetaGet("instance-type", "instance_type", 0644)

	// instance-id
	p.openstackMetaGet("instance-id", "instance_id", 0644)

	// local-hostname
	p.openstackMetaGet("local-hostname", "local_hostname", 0644)

	// ssh
	if err := p.handleSSH(); err != nil {
		p.l.Errorf("OpenStack: Failed to get ssh data: %s", err)
	}

	// Generic userdata
	userData, err := openstackGet(userDataURL)
	if err != nil {
		p.l.Errorf("OpenStack: Failed to get user-data: %s", err)
		// This is not an error
		return nil, nil
	}
	return userData, nil
}

// lookup a value (lookupName) in OpenStack's metaservice and store in given fileName
func (p *ProviderOpenstack) openstackMetaGet(lookupName string, fileName string, fileMode os.FileMode) {
	if lookupValue, err := openstackGet(metaDataURL + lookupName); err == nil {
		// we got a value from the metadata server, now save to filesystem
		err = os.WriteFile(path.Join(ConfigPath, fileName), lookupValue, fileMode)
		if err != nil {
			// we couldn't save the file for some reason
			p.l.Errorf("OpenStack: Failed to write %s:%s %s", fileName, lookupValue, err)
		}
	} else {
		// we did not get a value back from the metadata server
		p.l.Errorf("OpenStack: Failed to get %s: %s", lookupName, err)
	}
}

// openstackGet requests and extracts the requested URL
func openstackGet(url string) ([]byte, error) {
	var client = &http.Client{
		Timeout: time.Second * 2,
	}

	req, err := http.NewRequest("", url, nil)
	if err != nil {
		return nil, fmt.Errorf("OpenStack: http.NewRequest failed: %s", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("OpenStack: Could not contact metadata service: %s", err)
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("OpenStack: Status not ok: %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("OpenStack: Failed to read http response: %s", err)
	}
	return body, nil
}

// SSH keys:
func (p *ProviderOpenstack) handleSSH() error {
	sshKeys, err := openstackGet(metaDataURL + "public-keys/0/openssh-key")
	if err != nil {
		return fmt.Errorf("Failed to get sshKeys: %s", err)
	}

	if err := os.Mkdir(path.Join(ConfigPath, SSH), 0755); err != nil {
		return fmt.Errorf("Failed to create %s: %s", SSH, err)
	}

	err = os.WriteFile(path.Join(ConfigPath, SSH, "authorized_keys"), sshKeys, 0600)
	if err != nil {
		return fmt.Errorf("Failed to write ssh keys: %s", err)
	}
	return nil
}
