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
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"log"
)

// ProviderAzure reads from Azure's Instance Metadata Service (IMDS) API.
type ProviderAzure struct {
	client *http.Client
}

// NewAzure factory
func NewAzure() *ProviderAzure {
	client := &http.Client{
		Timeout: time.Second * 2,
	}
	return &ProviderAzure{
		client: client,
	}
}

func (p *ProviderAzure) String() string {
	return "Azure"
}

// Probe checks if Azure IMDS API is available
func (p *ProviderAzure) Probe() bool {
	// "Poll" VM Unique ID
	// See: https://azure.microsoft.com/en-us/blog/accessing-and-using-azure-vm-unique-id/
	_, err := p.imdsGet("compute/vmId")
	return (err == nil)
}

// Extract user data via Azure IMDS.
func (p *ProviderAzure) Extract() ([]byte, error) {
	if err := p.saveHostname(); err != nil {
		return nil, fmt.Errorf("%s: %s", p.String(), err)
	}

	if err := p.saveSSHKeys(); err != nil {
		log.Printf("%s: Saving SSH keys failed: %s", p.String(), err)
	}

	p.imdsSave("network/interface/0/ipv4/ipAddress/0/publicIpAddress")
	p.imdsSave("network/interface/0/ipv4/ipAddress/0/privateIpAddress")
	p.imdsSave("compute/zone")
	p.imdsSave("compute/vmId")

	userData, err := p.getUserData()
	if err != nil {
		return nil, fmt.Errorf("%s: %s", p.String(), err)
	}
	return userData, nil
}

func (p *ProviderAzure) saveHostname() error {
	hostname, err := p.imdsGet("compute/name")
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(path.Join(ConfigPath, Hostname), hostname, 0644)
	if err != nil {
		return fmt.Errorf("%s: Failed to write hostname: %s", p.String(), err)
	}
	log.Printf("%s: Saved hostname: %s", p.String(), string(hostname))
	return nil
}

func (p *ProviderAzure) saveSSHKeys() error {
	// TODO support multiple keys
	sshKey, err := p.imdsGet("compute/publicKeys/0/keyData")
	if err != nil {
		return fmt.Errorf("Getting SSH key failed: %s", err)
	}
	if err := os.Mkdir(path.Join(ConfigPath, SSH), 0755); err != nil {
		return fmt.Errorf("Creating directory %s failed: %s", SSH, err)
	}
	err = ioutil.WriteFile(path.Join(ConfigPath, SSH, "authorized_keys"), sshKey, 0600)
	if err != nil {
		return fmt.Errorf("Writing SSH key failed: %s", err)
	}
	log.Printf("%s: Saved authorized_keys", p.String())
	return nil
}

// Get resource value from IMDS and write to file in ConfigPath
func (p *ProviderAzure) imdsSave(resourceName string) {
	if value, err := p.imdsGet(resourceName); err == nil {
		fileName := strings.Replace(resourceName, "/", "_", -1)
		err = ioutil.WriteFile(path.Join(ConfigPath, fileName), value, 0644)
		if err != nil {
			log.Printf("%s: Failed to write file %s:%s %s", p.String(), fileName, value, err)
		}
		log.Printf("%s: Saved resource %s: %s", p.String(), resourceName, string(value))
	} else {
		log.Printf("%s: Failed to get resource %s: %s", p.String(), resourceName, err)
	}
}

// Get IMDS resource value
func (p *ProviderAzure) imdsGet(resourceName string) ([]byte, error) {
	req, err := http.NewRequest("GET", imdsURL(resourceName), nil)
	if err != nil {
		return nil, fmt.Errorf("http.NewRequest failed: %s", err)
	}
	req.Header.Set("Metadata", "true")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("IMDS unavailable: %s", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("IMDS returned status code: %d", resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("Reading HTTP response failed: %s", err)
	}

	return body, nil
}

// Build Azure Instance Metadata Service (IMDS) URL
// For available nodes, see: https://docs.microsoft.com/en-us/azure/virtual-machines/windows/instance-metadata-service
func imdsURL(node string) string {
	const (
		baseURL    = "http://169.254.169.254/metadata/instance"
		apiVersion = "2021-01-01"
		// For leaf nodes in /metadata/instance, the format=json doesn't work.
		// For these queries, format=text needs to be explicitly specified
		// because the default format is JSON.
		params = "?api-version=" + apiVersion + "&format=text"
	)
	if len(node) > 0 {
		return baseURL + "/" + node + params
	}
	return baseURL + params
}

func (p *ProviderAzure) getUserData() ([]byte, error) {
	userDataBase64, err := p.imdsGet("compute/userData")
	if err != nil {
		log.Printf("Failed to get user data: %s", err)
		return nil, err
	}

	userData := make([]byte, base64.StdEncoding.DecodedLen(len(userDataBase64)))
	msgLen, err := base64.StdEncoding.Decode(userData, userDataBase64)
	if err != nil {
		log.Printf("Failed to base64-decode user data: %s", err)
		return nil, err
	}
	userData = userData[:msgLen]

	defer ReportReady(p.client)

	return userData, nil
}
