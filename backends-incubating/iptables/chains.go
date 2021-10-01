package iptables

type iptablesJumpChain struct {
table    Table
dstChain Chain
srcChain Chain
comment  string
extraArgs []string
}

var iptablesJumpChains = []iptablesJumpChain{
{TableFilter, kubeExternalServicesChain, ChainInput, "kubernetes externally-visible service portals", []string{"-m", "conntrack", "--ctstate", "NEW"}},
{TableFilter, kubeExternalServicesChain, ChainForward, "kubernetes externally-visible service portals", []string{"-m", "conntrack", "--ctstate", "NEW"}},
{TableFilter, kubeNodePortsChain, ChainInput, "kubernetes health check service ports", nil},
{TableFilter, kubeServicesChain, ChainForward, "kubernetes service portals", []string{"-m", "conntrack", "--ctstate", "NEW"}},
{TableFilter, kubeServicesChain, ChainOutput, "kubernetes service portals", []string{"-m", "conntrack", "--ctstate", "NEW"}},
{TableFilter, kubeForwardChain, ChainForward, "kubernetes forwarding rules", nil},
{TableNAT, kubeServicesChain, ChainOutput, "kubernetes service portals", nil},
{TableNAT, kubeServicesChain, ChainPrerouting, "kubernetes service portals", nil},
{TableNAT, kubePostroutingChain, ChainPostrouting, "kubernetes postrouting rules", nil},
}

var iptablesEnsureChains = []struct {
table Table
chain Chain
}{
{TableNAT, KubeMarkDropChain},
}

var iptablesCleanupOnlyChains = []iptablesJumpChain{
// Present in kube 1.13 - 1.19. Removed by #95252 in favor of adding reject rules for incoming/forwarding packets to kubeExternalServicesChain
{TableFilter, kubeServicesChain, ChainInput, "kubernetes service portals", []string{"-m", "conntrack", "--ctstate", "NEW"}},
}
