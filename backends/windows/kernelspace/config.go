package kernelspace

// KubeProxyWinkernelConfiguration contains Windows/HNS settings for
// the Kubernetes proxy server.
type KubeProxyWinkernelConfiguration struct {
	// networkName is the name of the network kube-proxy will use
	// to create endpoints and policies
	NetworkName string
	// sourceVip is the IP address of the source VIP endpoint used for
	// NAT when loadbalancing
	SourceVip string
	// enableDSR tells kube-proxy whether HNS policies should be created
	// with DSR
	EnableDSR bool
}
