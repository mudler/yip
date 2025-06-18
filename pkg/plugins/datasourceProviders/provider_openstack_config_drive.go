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
	"path/filepath"

	"github.com/mudler/yip/pkg/logger"
)

// ListConfigDrives lists all the cdroms in the system the fill the config-drive standard
func ListConfigDrives(l logger.Interface) []Provider {
	// UserdataFiles is where to find the user data
	var userdataFiles = []string{"/openstack/latest/user_data"}
	cdroms, err := filepath.Glob(cdromDevs)
	if err != nil {
		// Glob can only error on invalid pattern
		panic(fmt.Sprintf("Invalid glob pattern: %s", cdromDevs))
	}
	l.Debugf("cdrom devices to be checked: %v", cdroms)
	// get the devices that match the cloud-init spec
	cidevs := FindCIs("config-2", l)
	l.Debugf("CONFIG-2 devices to be checked: %v", cidevs)
	// merge the two, ensuring that the list is unique
	cdroms = append(cidevs, cdroms...)
	cdroms = uniqueString(cdroms)
	l.Debugf("unique devices to be checked: %v", cdroms)
	var providers []Provider
	for _, device := range cdroms {
		providers = append(providers, NewProviderCDROM(device, userdataFiles, "CONFIG_DRIVE", l))
	}
	return providers
}
