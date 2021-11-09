package serviceevents

// IPKind decribes the kind of IP received by an IPsListener or and IPPortsListener.
type IPKind int

const (
	ClusterIP IPKind = iota
	ExternalIP
	LoadBalancerIP
)

//go:generate stringer -type=IPKind
// reminder: to get stringer: go install golang.org/x/tools/cmd/stringer
