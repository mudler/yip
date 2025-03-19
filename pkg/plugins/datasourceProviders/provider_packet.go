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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strings"
	"time"
)

const PacketBaseURL = "https://metadata.platformequinix.com"

type PacketMetadata struct {
	Hostname string   `json:"hostname"`
	SSHKeys  []string `json:"ssh_keys"`
}

// ProviderPacket is the type implementing the Provider interface for Packet.net
type ProviderPacket struct {
	metadata *PacketMetadata
	err      error
}

// NewPacket returns a new ProviderPacket
func NewPacket() *ProviderPacket {
	return &ProviderPacket{}
}

func (p *ProviderPacket) String() string {
	return "Packet"
}

// Probe checks if we are running on Packet
func (p *ProviderPacket) Probe() bool {
	// Unfortunately the host is resolveable globally, so no easy test
	// No default timeout, so introduce one
	c1 := make(chan ProviderPacket)
	go func() {
		res := ProviderPacket{}
		res.metadata, res.err = GetMetadata()
		c1 <- res
	}()

	select {
	case res := <-c1:
		p.metadata = res.metadata
		p.err = res.err
	case <-time.After(2 * time.Second):
		p.err = fmt.Errorf("packet: timeout while connecting")
	}
	return p.err == nil
}

// Extract gets both the Packet specific and generic userdata
func (p *ProviderPacket) Extract() ([]byte, error) {
	// do not retrieve if we Probed
	if p.metadata == nil && p.err == nil {
		p.metadata, p.err = GetMetadata()
		if p.err != nil {
			return nil, p.err
		}
	} else if p.err != nil {
		return nil, p.err
	}

	if err := os.WriteFile(path.Join(ConfigPath, Hostname), []byte(p.metadata.Hostname), 0644); err != nil {
		return nil, fmt.Errorf("packet: Failed to write hostname: %s", err)
	}

	if err := os.MkdirAll(path.Join(ConfigPath, SSH), 0755); err != nil {
		return nil, fmt.Errorf("failed to create %s: %s", SSH, err)
	}

	sshKeys := strings.Join(p.metadata.SSHKeys, "\n")

	if err := os.WriteFile(path.Join(ConfigPath, SSH, "authorized_keys"), []byte(sshKeys), 0600); err != nil {
		return nil, fmt.Errorf("failed to write ssh keys: %s", err)
	}

	userData, err := GetUserData()
	if err != nil {
		return nil, fmt.Errorf("packet: failed to get userdata: %s", err)
	}

	if len(userData) == 0 {
		return nil, nil
	}

	if len(userData) > 6 && string(userData[0:6]) == "#!ipxe" {
		// if you use the userdata for ipxe boot, no use as userdata
		return nil, nil
	}

	return userData, nil
}

// GetMetadata gets the metadata from the Packet metadata service
func GetMetadata() (*PacketMetadata, error) {
	res, err := http.Get(PacketBaseURL + "/metadata")
	if err != nil {
		return nil, err
	}

	b, err := io.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		return nil, err
	}

	var result struct {
		Error string `json:"error"`
		*PacketMetadata
	}
	if err := json.Unmarshal(b, &result); err != nil {
		if res.StatusCode >= 400 {
			return nil, errors.New(res.Status)
		}
		return nil, err
	}
	if result.Error != "" {
		return nil, errors.New(result.Error)
	}
	return result.PacketMetadata, nil
}

// GetUserData gets the user data from the Packet metadata service
func GetUserData() ([]byte, error) {
	res, err := http.Get(PacketBaseURL + "/userdata")
	if err != nil {
		return nil, err
	}

	b, err := io.ReadAll(res.Body)
	res.Body.Close()
	return b, err
}
