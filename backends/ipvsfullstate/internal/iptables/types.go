package iptables

type Table string

const (
	TableNat    Table = "nat"
	TableFilter Table = "filter"
)

type TableData struct {
	Table  Table
	Chains []Chain
	Rules  []Rule
}
type Chain string

const (
	ChainPreRouting       Chain = "PREROUTING"
	ChainInput                  = "INPUT"
	ChainForward                = "FORWARD"
	ChainOutput                 = "OUTPUT"
	ChainPostRouting            = "POSTROUTING"
	ChainKubeFirewall           = "KUBE-FIREWALL"
	ChainKubeLoadBalancer       = "KUBE-LOAD-BALANCER"
	ChainKubeMarkDrop           = "KUBE-MARK-DROP"
	ChainKubeMarkMasq           = "KUBE-MARK-MASQ"
	ChainKubeNodePort           = "KUBE-NODE-PORT"
	ChainKubePostRouting        = "KUBE-POSTROUTING"
	ChainKubeServices           = "KUBE-SERVICES"
)

type TargetOption string

const (
	TargetMarkOptionSetMark               TargetOption = "--set-xmark"
	TargetMarkOptionXorMark                            = "--xor-mark"
	TargetMarkOptionOrMark                             = "--or-mark"
	TargetMasqueradeOptionFullyRandomized              = "--random-fully"
)

type Target string

const (
	TargetAccept     Target = "ACCEPT"
	TargetDrop              = "DROP"
	TargetReturn            = "RETURN"
	TargetMasquerade        = "MASQUERADE"
	TargetMark              = "MARK"
)

type MatchModule string

const (
	MatchModuleComment   MatchModule = "comment"
	MatchModuleAddrType              = "addrtype"
	MatchModuleSet                   = "set"
	MatchModuleMark                  = "mark"
	MatchModuleConnTrack             = "conntrack"
	MatchModulePhysDev               = "physdev"
)

type MatchModuleOption string

const (
	MatchModuleCommentOptionComment     MatchModuleOption = "--comment"
	MatchModuleMarkOptionMark                             = "--mark"
	MatchModuleSetOptionSet                               = "--match-set"
	MatchModuleConnTrackOptionConnState                   = "--ctstate"
	MatchModulePhysDevOptionPhysDevIsIn                   = "--physdev-is-in"
	MatchModuleAddrTypeOptionSrcType                      = "--src-type"
	MatchModuleAddrTypeOptionDstType                      = "--dst-type"
)

type MatchOption struct {
	Module       MatchModule
	ModuleOption MatchModuleOption
	Value        string
	Inverted     bool
}

type Rule struct {
	From              Chain
	To                Chain
	Target            Target
	TargetOption      TargetOption
	TargetOptionValue string
	Protocol          Protocol
	MatchOptions      []MatchOption
}

type IPTableRules struct {
	NatRules []Rule
}

type Protocol string

const (
	ProtocolTCP  Protocol = "tcp"
	ProtocolUDP           = "udp"
	ProtocolSCTP          = "sctp"
)
