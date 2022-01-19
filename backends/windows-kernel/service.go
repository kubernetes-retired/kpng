package winkernel

import (
	"k8s.io/kubernetes/pkg/proxy"
	"k8s.io/klog/v2"
)

// internal struct for string service information
type serviceInfo struct {
        *proxy.BaseServiceInfo
        targetPort             int
        externalIPs            []*externalIPInfo
        loadBalancerIngressIPs []*loadBalancerIngressInfo
        hnsID                  string
        nodePorthnsID          string
        policyApplied          bool
        remoteEndpoint         *endpoints
        hns                    HostNetworkService
        preserveDIP            bool
        localTrafficDSR        bool
}


func (svcInfo *serviceInfo) deleteAllHnsLoadBalancerPolicy() {
        // Remove the Hns Policy corresponding to this service
        hns := svcInfo.hns
        hns.deleteLoadBalancer(svcInfo.hnsID)
        svcInfo.hnsID = ""

        hns.deleteLoadBalancer(svcInfo.nodePorthnsID)
        svcInfo.nodePorthnsID = ""

        for _, externalIP := range svcInfo.externalIPs {
                hns.deleteLoadBalancer(externalIP.hnsID)
                externalIP.hnsID = ""
        }
        for _, lbIngressIP := range svcInfo.loadBalancerIngressIPs {
                hns.deleteLoadBalancer(lbIngressIP.hnsID)
                lbIngressIP.hnsID = ""
        }
}

func (svcInfo *serviceInfo) cleanupAllPolicies(proxyEndpoints []proxy.Endpoint) {
        klog.V(3).InfoS("Service cleanup", "serviceInfo", svcInfo)
        // Skip the svcInfo.policyApplied check to remove all the policies
        svcInfo.deleteAllHnsLoadBalancerPolicy()
        // Cleanup Endpoints references
        for _, ep := range proxyEndpoints {
                epInfo, ok := ep.(*endpoints)
                if ok {
                        epInfo.Cleanup()
                }
        }
        if svcInfo.remoteEndpoint != nil {
                svcInfo.remoteEndpoint.Cleanup()
        }

        svcInfo.policyApplied = false
}

