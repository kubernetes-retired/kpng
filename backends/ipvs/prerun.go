/*
Copyright 2021 The Kubernetes Authors.

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

package ipvs

import (
	"fmt"

	"github.com/vishvananda/netlink"
)

func PreRun() error {
	err := NewDummyInterface("kube-ipvs0")
	if err != nil {
		return err
	}
	return nil
}

func NewDummyInterface(dummyInterface string) error {
	// TODO: Turn configurable
	_, err := netlink.LinkByName(dummyInterface)
	if err != nil {
		_, ok := err.(netlink.LinkNotFoundError)
		if !ok {
			return fmt.Errorf("unable to get dummy interface: %s", err)
		}
		dummy := &netlink.Dummy{
			LinkAttrs: netlink.LinkAttrs{Name: dummyInterface},
		}
		err = netlink.LinkAdd(dummy)
		if err != nil {
			return fmt.Errorf("unable to add dummy interface: %s", err)
		}
		return nil
	}

	return nil

}
