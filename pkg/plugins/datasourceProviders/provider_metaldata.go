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

const (
	metaldataMetaDataURL = "http://metaldata/get/meta/"
	metaldataUserDataURL = "http://metaldata/get/user"
)

// ProviderMetaldata is the type implementing the Provider interface for Metaldata
type ProviderMetaldata struct {
	l logger.Interface
}

// NewMetalData returns a new ProviderMetaldata
func NewMetalData(l logger.Interface) *ProviderMetaldata {
	return &ProviderMetaldata{l}
}

func (p *ProviderMetaldata) String() string {
	return "Metaldata"
}

// Probe checks if we are running on Metaldata
func (p *ProviderMetaldata) Probe() bool {
	// Getting the hostname should always work...
	_, err := metaldataGet(metaldataMetaDataURL + "hostname")
	return err == nil
}

// Extract gets both the Metaldata specific and generic userdata
func (p *ProviderMetaldata) Extract() ([]byte, error) {
	// Get host name. This must not fail
	hostname, err := metaldataGet(metaldataMetaDataURL + "hostname")
	if err != nil {
		return nil, err
	}
	err = os.WriteFile(path.Join(ConfigPath, Hostname), hostname, 0644)
	if err != nil {
		return nil, fmt.Errorf("Metaldata: Failed to write hostname: %s", err)
	}

	// public ipv4
	p.metaldataMetaGet("public-ipv4", "public_ipv4", 0644)

	// private ipv4
	p.metaldataMetaGet("private-ipv4", "private_ipv4", 0644)

	// failure domain
	p.metaldataMetaGet("failure-domain", "failure_domain", 0644)

	// id
	p.metaldataMetaGet("machine-id", "machine_id", 0644)

	// type
	p.metaldataMetaGet("machine-type", "machine_type", 0644)

	// ssh
	if err := p.handleSSH(); err != nil {
		p.l.Errorf("Metaldata: Failed to get ssh data: %s", err)
	}

	// Generic userdata
	userData, err := metaldataGet(metaldataUserDataURL)
	if err != nil {
		p.l.Errorf("Metaldata: Failed to get user-data: %s", err)
		// This is not an error
		return nil, nil
	}
	return userData, nil
}

// lookup a value (lookupName) in Metaldata metaservice and store in given fileName
func (p *ProviderMetaldata) metaldataMetaGet(lookupName string, fileName string, fileMode os.FileMode) {
	if lookupValue, err := metaldataGet(metaldataMetaDataURL + lookupName); err == nil {
		// we got a value from the metadata server, now save to filesystem
		err = os.WriteFile(path.Join(ConfigPath, fileName), lookupValue, fileMode)
		if err != nil {
			// we couldn't save the file for some reason
			p.l.Errorf("Metaldata: Failed to write %s:%s %s", fileName, lookupValue, err)
		}
	} else {
		// we did not get a value back from the metadata server
		p.l.Errorf("Metaldata: Failed to get %s: %s", lookupName, err)
	}
}

// metaldataGet requests and extracts the requested URL
func metaldataGet(url string) ([]byte, error) {
	var client = &http.Client{
		Timeout: time.Second * 2,
	}

	req, err := http.NewRequest("", url, nil)
	if err != nil {
		return nil, fmt.Errorf("Metaldata: http.NewRequest failed: %s", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Metaldata: Could not contact metadata service: %s", err)
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Metaldata: Status not ok: %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("Metaldata: Failed to read http response: %s", err)
	}
	return body, nil
}

// SSH keys:
func (p *ProviderMetaldata) handleSSH() error {
	sshKeys, err := metaldataGet(metaldataMetaDataURL + "authorized-keys")
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
