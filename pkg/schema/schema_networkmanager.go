//   Copyright 2022 Itxaka Serrano <itxakaserrano@gmail.com>
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

package schema

import (
	"github.com/google/uuid"
	"gopkg.in/ini.v1"
	"reflect"
)

type NetConnection struct {
	// Name used to store the file, if empty then we try to use the connection id,
	// if also empty use the connection type + interface
	Name         string
	Connection   []map[string]string
	Wifi         []map[string]string
	WifiSecurity []map[string]string
	Ipv4         []map[string]string
	Ipv6         []map[string]string
	Proxy        []map[string]string
	X8021        []map[string]string // Section name is 802-1x
	Adsl         []map[string]string
	Bluetooth    []map[string]string
	Bond         []map[string]string
	Bridge       []map[string]string
	Bridgeport   []map[string]string // Section name is bridge-port
	Cdma         []map[string]string
	Dcb          []map[string]string
	Ethtool      []map[string]string
	Gsm          []map[string]string
	Infiniband   []map[string]string
	Iptunnel     []map[string]string // Section name is ip-tunnel
	Macsec       []map[string]string
	Macvlan      []map[string]string
	Olpcmesh     []map[string]string // Section name is olpc-mesh or 802-11-olpc-mesh
	Ovsbridge    []map[string]string // Section name is ovs-bridge
	Ovsdpdk      []map[string]string // Section name is ovs-dpdk
	Ovsinterface []map[string]string // Section name is ovs-interface
	Ovspatch     []map[string]string // Section name is ovs-patch
	Ovsport      []map[string]string // Section name is ovs-port
	Ppp          []map[string]string
	Pppoe        []map[string]string
	Serial       []map[string]string
	Sriov        []map[string]string
	Tc           []map[string]string
	Team         []map[string]string
	Teamport     []map[string]string // Section name is team-port
	Tun          []map[string]string
	Vlan         []map[string]string
	Vpn          []map[string]string
	Vrf          []map[string]string
	Vxlan        []map[string]string
	Wifip2p      []map[string]string // Section name is wifi-p2p
	Wimax        []map[string]string
	Ethernet     []map[string]string
	Wireguard    []map[string]string
	Wpan         []map[string]string
	Bondport     []map[string]string // Section name is bond-port
	Hostname     []map[string]string
	Veth         []map[string]string
}

// substitute contains the map to transform the valid struct fields into the valid section names
// all section names are set to lower case by default with "gopkg.in/ini.v1" options
var substitute = map[string]string{
	"X8021":        "802-1x",
	"Bridgeport":   "bridge-port",
	"Iptunnel":     "ip-tunnel",
	"Olpcmesh":     "olpc-mesh",
	"Ovsbridge":    "ovs-bridge",
	"Ovsdpdk":      "ovs-dpdk",
	"Ovsinterface": "ovs-interface",
	"Ovspatch":     "ovs-patch",
	"Ovsport":      "ovs-port",
	"Teamport":     "team-port",
	"Wifip2p":      "wifi-p2p",
	"Bondport":     "bond-port",
}

// ToInifile transforms a NetConnection struct into an ini file compatible with NetworkManager
func (n NetConnection) ToInifile() (*ini.File, error) {
	var err error
	// Set PrettyFormat false to avoid adding spaces between key and value
	ini.PrettyFormat = false
	cfg := ini.Empty(ini.LoadOptions{
		Insensitive:         true,
		InsensitiveSections: true,
		InsensitiveKeys:     true,
	})

	// Set unique uuid per connection
	id, err := uuid.NewRandom()
	if err != nil {
		return nil, err
	}
	_, err = cfg.Section("connection").NewKey("uuid", id.String())
	if err != nil {
		return nil, err
	}

	// Some workaround to reduce code in here
	// reflect NetConnection
	// iterate over the number of fields
	// get the section name via the field name
	// substitute the section name if needed (config names and golang names are not compatible)
	// Check format of the field
	// if map[string]string, get the values of the field as key,value and store it under that section
	// This avoids having to iterate over each section manually
	v := reflect.ValueOf(n)
	v.NumField()
	typeOfS := v.Type()
	for i := 0; i < v.NumField(); i++ {
		sectionName := typeOfS.Field(i).Name
		// Check if we need to substitute the section name
		if substitute[typeOfS.Field(i).Name] != "" {
			sectionName = substitute[typeOfS.Field(i).Name]
		}
		switch values := v.Field(i).Interface().(type) {
		case []map[string]string:
			for _, kv := range values {
				for key, value := range kv {
					_, err = cfg.Section(sectionName).NewKey(key, value)
					if err != nil {
						return nil, err
					}
				}
			}
		default:
			continue
		}
	}
	return cfg, err
}
