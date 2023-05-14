#!/bin/bash
# shellcheck disable=SC2181,SC2155,SC2128
#
# Copyright 2022 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#         http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# iptables specific skipped ginkgo tests

GINKGO_SKIP_ipv4_iptables_TEST="should be updated after adding or deleting ports\
|should serve multiport endpoints from pods\
|should check kube-proxy urls\
|should be rejected when no endpoints exist\
|should be able to preserve UDP traffic when server pod cycles for a NodePort service\
|should run through the lifecycle of Pods and PodStatus\
|should mirror a custom Endpoint with multiple subsets and same IP address\
|should support a Service with multiple ports specified in multiple EndpointSlices\
|should support a Service with multiple endpoint IPs specified in multiple EndpointSlices|" 


GINKGO_SKIP_ipv6_iptables_TEST="should be updated after adding or deleting ports\
|should serve multiport endpoints from pods\
|should check kube-proxy urls\
|should be able to change the type from ClusterIP to ExternalName\
|should create endpoints for unready pods\
|should be able to change the type from NodePort to ExternalName\
|should provide DNS for services\
|should provide DNS for the cluster\
|should provide DNS for pods for Subdomain\
|should function for endpoint-Service: udp\
|should be able to preserve UDP traffic when server pod cycles for a NodePort service\
|should mirror a custom Endpoint with multiple subsets and same IP address\
|should support a Service with multiple endpoint IPs specified in multiple EndpointSlices\
|should support a Service with multiple ports specified in multiple EndpointSlices" 

GINKGO_SKIP_dual_iptables_TEST="should be updated after adding or deleting ports\
|should serve multiport endpoints from pods\
|should be able to preserve UDP traffic when server pod cycles for a NodePort service\
|should check kube-proxy urls\
|should support a Service with multiple ports specified in multiple EndpointSlices\
|should support a Service with multiple endpoint IPs specified in multiple EndpointSlices\
|should mirror a custom Endpoint with multiple subsets and same IP address" 
    
GINKGO_SKIP_ipv4_ipvs_TEST="should check kube-proxy urls\
|should be able to switch session affinity for NodePort service\
|should have session affinity work for NodePort service\
|should have session affinity timeout work for NodePort service\
|should support a Service with multiple endpoint IPs specified in multiple EndpointSlices\
|should support a Service with multiple ports specified in multiple EndpointSlices\
|should mirror a custom Endpoint with multiple subsets and same IP address"

GINKGO_SKIP_ipv6_ipvs_TEST="should have session affinity work for NodePort service\
|should have session affinity timeout work for NodePort service\
|should be able to update service type to NodePort listening on same port number but different protocols\
|should be able to switch session affinity for NodePort service\
|should be able to create a functioning NodePort service\
|should be able to connect to terminating and unready endpoints if PublishNotReadyAddresses is true\
|should be able to change the type from ExternalName to NodePort\
|should be able to preserve UDP traffic when server pod cycles for a NodePort service\
|should function for endpoint-Service: udp\
|should function for service endpoints using hostNetwork\
|should support basic nodePort: udp functionality\
|should function for node-Service: http\
|should update nodePort: http\
|should function for pod-Service: udp\
|should function for node-Service: udp\
|should function for multiple endpoint-Services with same selector\
|should check kube-proxy urls\
|should work after the service has been recreated\
|should update nodePort: udp\
|should be able to change the type from ClusterIP to ExternalName\
|should create endpoints for unready pods\
|should be able to change the type from NodePort to ExternalName\
|should provide DNS for pods for Subdomain\
|should provide DNS for services\
|should provide DNS for the cluster\
|should run through the lifecycle of Pods and PodStatus\
|should function for endpoint-Service: http\
|should be able to connect to terminating and unready endpoints if PublishNotReadyAddresses is true\
|should support a Service with multiple endpoint IPs specified in multiple EndpointSlices\
|should serve endpoints on same port and different protocols\
|should support a Service with multiple ports specified in multiple EndpointSlices\
|should mirror a custom Endpoint with multiple subsets and same IP address\
|should function for pod-Service: http"

GINKGO_SKIP_dual_ipvs_TEST="should work after the service has been recreated\
|should be able to update service type to NodePort listening on same port number but different protocols\
|should not be able to connect to terminating and unready endpoints if PublishNotReadyAddresses is false\
|should be able to preserve UDP traffic when server pod cycles for a NodePort service\
|should be able to preserve UDP traffic when server pod cycles for a ClusterIP service\
|should implement service.kubernetes.io/service-proxy-name\
|should have session affinity timeout work for service with type clusterIP\
|should have session affinity work for NodePort service\
|should be able to preserve UDP traffic when server pod cycles for a ClusterIP service\
|should have session affinity timeout work for service with type clusterIP\
|should be able to preserve UDP traffic when initial unready endpoints\
|should be able to switch session affinity for service with type clusterIP\
|should create endpoints for unready pods\
|should have session affinity timeout work for NodePort service\
|should be able to connect to terminating and unready endpoints if PublishNotReadyAddresses is true\
|should drop INVALID conntrack entries\
|should implement service.kubernetes.io/headless\
|should check kube-proxy urls\
|should be able to switch session affinity for NodePort service\
|should function for multiple endpoint-Services with same selector\
|should be able to create a functioning NodePort service\
|should be able to connect to terminating and unready endpoints if PublishNotReadyAddresses is true\
|should support a Service with multiple endpoint IPs specified in multiple EndpointSlices\
|should serve endpoints on same port and different protocols\
|should support a Service with multiple ports specified in multiple EndpointSlices\
|should mirror a custom Endpoint with multiple subsets and same IP address\
|should be able to change the type from ExternalName to NodePort"

GINKGO_SKIP_ipv4_nft_TEST="should check kube-proxy urls\
|should work with the pod containing more than 6 DNS search paths and longer than 256 search list characters\
|should run through the lifecycle of Pods and PodStatus\
|should support a Service with multiple endpoint IPs specified in multiple EndpointSlices\
|should mirror a custom Endpoint with multiple subsets and same IP address\
|should support a Service with multiple ports specified in multiple EndpointSlices"


GINKGO_SKIP_ipv6_nft_TEST="should work after the service has been recreated\
|ServiceAccountIssuerDiscovery should support OIDC discovery of service account issuer\
|should be able to change the type from ClusterIP to ExternalName\
|should be able to change the type from ExternalName to ClusterIP\
|should be able to change the type from ExternalName to NodePort\
|should be able to change the type from NodePort to ExternalName\
|should be able to connect to terminating and unready endpoints if PublishNotReadyAddresses is true\
|should be able to create a functioning NodePort service\
|should be able to handle large requests: http\
|should be able to handle large requests: udp\
|should be able to preserve UDP traffic when server pod cycles for a NodePort service\
|should be able to switch session affinity for NodePort service\
|should be able to switch session affinity for service with type clusterIP\
|should be able to up and down services\
|should be able to update service type to NodePort listening on same port number but different protocols\
|should be possible to connect to a service via ExternalIP when the external IP is not assigned to a node\
|should be updated after adding or deleting port\
|should check kube-proxy urls\
|should create endpoints for unready pods\
|should function for client IP based session affinity: http\
|should function for client IP based session affinity: udp\
|should function for endpoint-Service: http\
|should function for endpoint-Service: udp\
|should function for multiple endpoint-Services with same selector\
|should function for node-Service: http\
|should function for node-Service: udp\
|should function for pod-Service: http\
|should function for pod-Service: udp\
|should function for service endpoints using hostNetwork\
|should have session affinity timeout work for NodePort service\
|should have session affinity timeout work for service with type clusterIP\
|should have session affinity work for NodePort service\
|should have session affinity work for service with type clusterIP\
|should implement service.kubernetes.io/headless\
|should implement service.kubernetes.io/service-proxy-name\
|should provide DNS for pods for Subdomain\
|should provide DNS for services\
|should provide DNS for the cluster\
|should serve multiport endpoints from pods\
|should support basic nodePort: udp functionality\
|should update endpoints: http\
|should update nodePort: udp\
|should work after the service has been recreated\
|work after the service has been recreated\
|should update endpoints: udp\
|should update nodePort: http\
|should support a Service with multiple endpoint IPs specified in multiple EndpointSlices\
|should support a Service with multiple ports specified in multiple EndpointSlices\
|should mirror a custom Endpoint with multiple subsets and same IP address"

GINKGO_SKIP_dual_nft_TEST="should check kube-proxy urls\
|should be rejected when no endpoints exist\
|should check kube-proxy urls\
|should be rejected when no endpoints exist\
|should create a LimitRange with defaults and ensure pod has those defaults applied.\
|should work with the pod containing more than 6 DNS search paths and longer than 256 search list characters\
|should support a Service with multiple ports specified in multiple EndpointSlices\
|should support a Service with multiple endpoint IPs specified in multiple EndpointSlices\
|should mirror a custom Endpoint with multiple subsets and same IP address" 

GINKGO_SKIP_ipv4_ebpf_TEST="should serve multiport endpoints from pods\
|should have session affinity work for service with type clusterIP\
|should have session affinity work for NodePort service\
|should have session affinity timeout work for service with type clusterIP\
|should have session affinity timeout work for NodePort service\
|should be updated after adding or deleting ports\
|should be rejected when no endpoints exist\
|should be rejected for evicted pods\
|should be possible to connect to a service via ExternalIP when the external IP is not assigned to a node\
|should be able to update service type to NodePort listening on same port number but different protocols\
|should be able to switch session affinity for service with type clusterIP\
|should be able to switch session affinity for NodePort service\
|should be able to create a functioning NodePort service\
|should be able to connect to terminating and unready endpoints if PublishNotReadyAddresses is true\
|should be able to change the type from ExternalName to NodePort\
|should be able to change the type from ExternalName to ClusterIP\
|should allow pods to hairpin back to themselves through services\
|should drop INVALID conntrack entries\
|should be able to preserve UDP traffic when server pod cycles for a NodePort service\
|should be able to preserve UDP traffic when initial unready endpoints get ready\
|should work after the service has been recreated\
|should serve a basic endpoint from pods\
|should preserve source pod IP for traffic thru service cluster IP\
|should implement service.kubernetes.io/service-proxy-name\
|should implement service.kubernetes.io/headless\
|should create endpoints for unready pods\
|should be able to up and down services\
|should be able to preserve UDP traffic when server pod cycles for a ClusterIP service\
|should update endpoints: http\
|should support basic nodePort: udp functionality\
|should update nodePort: udp\
|should function for node-Service: udp\
|should create and stop a working application\
|should be able to change the type from NodePort to ExternalName\
|should be able to change the type from ClusterIP to ExternalName \
|should resolve DNS of partial qualified names for services\
|should provide DNS for services\
|should provide DNS for pods for Subdomain\
|should provide DNS for ExternalName services\
|ServiceAccountIssuerDiscovery should support OIDC discovery of service account issuer\
|Networking Granular Checks: Services [It] should function for node-Service: http\
|should check kube-proxy urls\
|should resolve DNS of partial qualified names for services on hostNetwork pods with dnsPolicy: ClusterFirstWithHostNet\
|should work with the pod containing more than 6 DNS search paths and longer than 256 search list characters\
|should resolve DNS of partial qualified names for the cluster\
|should provide DNS for the cluster\
|should function for service endpoints using hostNetwork\
|should be able to handle large requests: udp\
|should update endpoints: udp\
|should function for pod-Service: udp\
|should function for pod-Service: http\
|should function for client IP based session affinity: http\
|should function for multiple endpoint-Services with same selector\
|should function for node-Service: http\
|should function for endpoint-Service: http\
|should function for pod-Service: udp\
|should function for client IP based session affinity: udp\
|should function for endpoint-Service: udp\
|should run through the lifecycle of Pods and PodStatus\
|should update nodePort: http\
|should be able to handle large requests: http\
|should mirror a custom Endpoint with multiple subsets and same IP address\
|should support a Service with multiple ports specified in multiple EndpointSlices\
|should support a Service with multiple endpoint IPs specified in multiple EndpointSlices\
|should serve endpoints on same port and different protocols"

GINKGO_SKIP_ipv4_userspacelin_TEST="should preserve source pod IP for traffic thru service cluster IP\
|should be rejected when no endpoints exist\
|should be rejected for evicted pods\
|should be able to switch session affinity for service with type clusterIP\
|should be able to switch session affinity for NodePort service\
|should be able to preserve UDP traffic when initial unready endpoints get ready\
|should have session affinity timeout work for service with type clusterIP\
|should be able to preserve UDP traffic when server pod cycles for a ClusterIP service\
|should be able to preserve UDP traffic when server pod cycles for a NodePort service\
|should check kube-proxy urls\
|should preserve source pod IP for traffic thru service cluster IP\
|should have session affinity timeout work for NodePort service\
|should run through the lifecycle of Pods and PodStatus\
|should support a Service with multiple ports specified in multiple EndpointSlices\
|should support a Service with multiple endpoint IPs specified in multiple EndpointSlices\
|should mirror a custom Endpoint with multiple subsets and same IP address"