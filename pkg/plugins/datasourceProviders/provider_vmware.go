//go:build (linux && 386) || (linux && amd64)

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
	"compress/gzip"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"

	"github.com/mudler/yip/pkg/logger"
	"github.com/vmware/vmw-guestinfo/rpcvmx"
	"github.com/vmware/vmw-guestinfo/vmcheck"
)

const (
	guestMetaData = "guestinfo.metadata"

	guestUserData = "guestinfo.userdata"

	guestVendorData = "guestinfo.vendordata"
)

// ProviderVMware implements the Provider interface for VMware guestinfo api
type ProviderVMware struct {
	l logger.Interface
}

// NewVMware returns a new VMware Provider
func NewVMware(l logger.Interface) *ProviderVMware {
	return &ProviderVMware{l}
}

// String returns provider name
func (p *ProviderVMware) String() string {
	return "VMWARE"
}

// Probe checks if we are running on VMware and either userdata or metadata is set
func (p *ProviderVMware) Probe() bool {
	isVM, err := vmcheck.IsVirtualWorld(true)
	if err != nil || !isVM {
		return false
	}

	md, merr := p.vmwareGet(guestMetaData)
	ud, uerr := p.vmwareGet(guestUserData)

	return ((merr == nil) && len(md) > 1 && string(md) != "---") || ((uerr == nil) && len(ud) > 1 && string(ud) != "---")
}

// Extract gets the host specific metadata, generic userdata and if set vendordata
// This function returns error if it fails to write metadata or vendordata to disk
func (p *ProviderVMware) Extract() ([]byte, error) {
	// Get vendor data, if empty do not fail
	vendorData, err := p.vmwareGet(guestVendorData)
	if err != nil {
		p.l.Errorf("VMWare: Failed to get vendordata: %v", err)
	} else {
		err = os.WriteFile(path.Join(ConfigPath, "vendordata"), vendorData, 0644)
		if err != nil {
			p.l.Errorf("VMWare: Failed to write vendordata: %v", err)
		}
	}

	// Get metadata
	metaData, err := p.vmwareGet(guestMetaData)
	if err != nil {
		p.l.Errorf("VMWare: Failed to get metadata: %v", err)
	} else {
		err = os.WriteFile(path.Join(ConfigPath, "metadata"), metaData, 0644)
		if err != nil {
			return nil, fmt.Errorf("VMWare: Failed to write metadata: %s", err)
		}
	}

	// Get userdata
	userData, err := p.vmwareGet(guestUserData)
	if err != nil {
		p.l.Errorf("VMware: Failed to get userdata: %v", err)
		// This is not an error
		return nil, nil
	}

	return userData, nil
}

// vmwareGet gets and extracts the guestinfo data
func (p *ProviderVMware) vmwareGet(name string) ([]byte, error) {
	config := rpcvmx.NewConfig()

	// get the gusest info value
	out, err := config.String(name, "")
	if err != nil {
		p.l.Errorf("Getting guest info %s failed: error %s", name, err)
		return nil, err
	}

	enc, err := config.String(name+".encoding", "")
	if err != nil {
		p.l.Errorf("Getting guest info %s.encoding failed: error %s", name, err)
		return nil, err
	}

	switch strings.TrimSuffix(enc, "\n") {
	case " ":
		return []byte(strings.TrimSuffix(out, "\n")), nil
	case "base64":
		r := base64.NewDecoder(base64.StdEncoding, strings.NewReader(out))

		dst, err := ioutil.ReadAll(r)
		if err != nil {
			p.l.Errorf("Decoding base64 of '%s' failed %v", name, err)
			return nil, err
		}

		return dst, nil
	case "gzip+base64":
		r := base64.NewDecoder(base64.StdEncoding, strings.NewReader(out))

		zr, err := gzip.NewReader(r)
		if err != nil {
			p.l.Errorf("New gzip reader from '%s' failed %v", name, err)
			return nil, err
		}

		dst, err := ioutil.ReadAll(zr)
		if err != nil {
			p.l.Errorf("Read '%s' failed %v", name, err)
			return nil, err
		}

		return dst, nil
	default:
		return nil, fmt.Errorf("Unknown encoding %s", string(enc))
	}
}
