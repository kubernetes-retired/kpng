package serviceevents

// TrafficPolicyKind decribes the type of traffic policy received by TrafficPolicyListener
type TrafficPolicyKind int

const (
	TrafficPolicyInternal TrafficPolicyKind = iota
	TrafficPolicyExternal
)

//go:generate stringer -type=TrafficPolicyKind
// reminder: to get stringer: go install golang.org/x/tools/cmd/stringer
