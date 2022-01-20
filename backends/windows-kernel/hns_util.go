//go:build windows
// +build windows

/*
Copyright 2018-2022 The Kubernetes Authors.

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

package winkernel

import (
	"fmt"
	"time"
	"os"

	"github.com/Microsoft/hcsshim/hcn"
	"github.com/Microsoft/hcsshim"
	"k8s.io/klog/v2"
	netutils "k8s.io/utils/net"
)

const NETWORK_TYPE_OVERLAY = "overlay"

type hnsNetworkInfo struct {
        name          string
        id            string
        networkType   string
        remoteSubnets []*remoteSubnetInfo
}

func deleteAllHnsLoadBalancerPolicy() {
        plists, err := hcsshim.HNSListPolicyListRequest()
        if err != nil {
                return
        }
        for _, plist := range plists {
                klog.V(3).InfoS("Remove policy", "policies", plist)
                _, err = plist.Delete()
                if err != nil {
                        klog.ErrorS(err, "Failed to delete policy list")
                }
        }

}

func getHnsNetworkInfo(hnsNetworkName string) (*hnsNetworkInfo, error) {
        hnsnetwork, err := hcsshim.GetHNSNetworkByName(hnsNetworkName)
        if err != nil {
                klog.ErrorS(err, "Failed to get HNS Network by name")
                return nil, err
        }

        return &hnsNetworkInfo{
                id:          hnsnetwork.Id,
                name:        hnsnetwork.Name,
                networkType: hnsnetwork.Type,
        }, nil
}


func newHostNetworkService() (HostNetworkService, hcn.SupportedFeatures) {
        var hns HostNetworkService
        hns = hnsV1{}
        supportedFeatures := hcn.GetSupportedFeatures()
        if supportedFeatures.Api.V2 {
                hns = hnsV2{}
        }

        return hns, supportedFeatures
}

func getNetworkName(hnsNetworkName string) (string, error) {
        if len(hnsNetworkName) == 0 {
                klog.V(3).InfoS("Flag --network-name not set, checking environment variable")
                hnsNetworkName = os.Getenv("KUBE_NETWORK")
                if len(hnsNetworkName) == 0 {
                        return "", fmt.Errorf("Environment variable KUBE_NETWORK and network-flag not initialized")
                }
        }
        return hnsNetworkName, nil
}

func getNetworkInfo(hns HostNetworkService, hnsNetworkName string) (*hnsNetworkInfo, error) {
        hnsNetworkInfo, err := hns.getNetworkByName(hnsNetworkName)
        for err != nil {
                klog.ErrorS(err, "Unable to find HNS Network specified, please check network name and CNI deployment", "hnsNetworkName", hnsNetworkName)
                time.Sleep(1 * time.Second)
                hnsNetworkInfo, err = hns.getNetworkByName(hnsNetworkName)
        }
        return hnsNetworkInfo, err
}

func newSourceVIP(hns HostNetworkService, network string, ip string, mac string, providerAddress string) (*endpoints, error) {
        hnsEndpoint := &endpoints{
                ip:              ip,
                isLocal:         true,
                macAddress:      mac,
                providerAddress: providerAddress,

                ready:       true,
                serving:     true,
                terminating: false,
        }
        ep, err := hns.createEndpoint(hnsEndpoint, network)
        return ep, err
}

func isNetworkNotFoundError(err error) bool {
        if err == nil {
                return false
        }
        if _, ok := err.(hcn.NetworkNotFoundError); ok {
                return true
        }
        if _, ok := err.(hcsshim.NetworkNotFoundError); ok {
                return true
        }
        return false
}

func (network hnsNetworkInfo) findRemoteSubnetProviderAddress(ip string) string {
        var providerAddress string
        for _, rs := range network.remoteSubnets {
                _, ipNet, err := netutils.ParseCIDRSloppy(rs.destinationPrefix)
                if err != nil {
                        klog.ErrorS(err, "Failed to parse CIDR")
                }
                if ipNet.Contains(netutils.ParseIPSloppy(ip)) {
                        providerAddress = rs.providerAddress
                }
                if ip == rs.providerAddress {
                        providerAddress = rs.providerAddress
                }
        }

        return providerAddress
}