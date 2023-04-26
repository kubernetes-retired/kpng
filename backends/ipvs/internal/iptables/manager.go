package iptables

import (
	"io"
	"k8s.io/klog/v2"
	"k8s.io/utils/exec"
	"text/template"
)

const ip4tablesRestoreCmd = "iptables-restore"
const ip6tablesRestoreCmd = "ip6tables-restore"

var DefaultChains = []Chain{ChainPreRouting, ChainInput, ChainForward, ChainOutput, ChainPostRouting}

type Manager struct {
	dataV4   map[Table]TableData
	dataV6   map[Table]TableData
	template *template.Template
}

func getIptablesRestoreCmd(protocolFamily ProtocolFamily) string {
	if protocolFamily == ProtocolFamilyIPv4 {
		return ip4tablesRestoreCmd
	}
	return ip6tablesRestoreCmd
}

func NewManager() *Manager {
	dataV4 := map[Table]TableData{
		TableNat: {
			Table:  TableNat,
			Chains: []Chain{ChainPreRouting, ChainInput, ChainOutput, ChainPostRouting},
			Rules:  []Rule{},
		},
		TableFilter: {
			Table:  TableFilter,
			Chains: []Chain{ChainInput, ChainForward, ChainOutput},
			Rules:  []Rule{},
		},
	}

	dataV6 := map[Table]TableData{
		TableNat: {
			Table:  TableNat,
			Chains: []Chain{ChainPreRouting, ChainInput, ChainOutput, ChainPostRouting},
			Rules:  []Rule{},
		},
		TableFilter: {
			Table:  TableFilter,
			Chains: []Chain{ChainInput, ChainForward, ChainOutput},
			Rules:  []Rule{},
		},
	}

	funcMap := template.FuncMap{
		"is_default_chain": IsDefaultChain,
		"need_quotes":      NeedQuotes,
	}

	iptTemplate, err := template.New("Template").Funcs(funcMap).Parse(Template)
	if err != nil {
		klog.Fatalf("error parsing iptables template: Template, error: %e", err)
	}

	iptTemplate, err = iptTemplate.New("TableTemplate").Parse(TableTemplate)
	if err != nil {
		klog.Fatalf("error parsing iptables template: TableTemplate, error: %e", err)
	}

	iptTemplate, err = iptTemplate.New("ChainTemplate").Parse(ChainTemplate)
	if err != nil {
		klog.Fatalf("error parsing iptables template: ChainTemplate, error: %e", err)
	}

	iptTemplate, err = iptTemplate.New("RuleTemplate").Parse(RuleTemplate)
	if err != nil {
		klog.Fatalf("error parsing iptables template: RuleTemplate, error: %e", err)
	}

	iptTemplate, err = iptTemplate.New("MatchTemplate").Parse(MatchTemplate)
	if err != nil {
		klog.Fatalf("error parsing iptables template: MatchTemplate, error: %e", err)
	}

	iptTemplate, err = iptTemplate.New("ProtocolTemplate").Parse(ProtocolTemplate)
	if err != nil {
		klog.Fatalf("error parsing iptables template: ProtocolTemplate, error: %e", err)
	}

	return &Manager{
		dataV4:   dataV4,
		dataV6:   dataV6,
		template: iptTemplate,
	}
}

func (m *Manager) AddChain(chain Chain, table Table, protocolFamily ProtocolFamily) {
	if protocolFamily == ProtocolFamilyIPv4 {
		tableData, _ := m.dataV4[table]
		tableData.Chains = append(tableData.Chains, chain)
		m.dataV4[table] = tableData
	} else {
		tableData, _ := m.dataV6[table]
		tableData.Chains = append(tableData.Chains, chain)
		m.dataV6[table] = tableData
	}
}

func (m *Manager) AddRule(rule Rule, table Table, protocolFamily ProtocolFamily) {
	if protocolFamily == ProtocolFamilyIPv4 {
		tableData, _ := m.dataV4[table]
		tableData.Rules = append(tableData.Rules, rule)
		m.dataV4[table] = tableData
	} else {
		tableData, _ := m.dataV6[table]
		tableData.Rules = append(tableData.Rules, rule)
		m.dataV6[table] = tableData
	}

}

func (m *Manager) Apply() {
	var data []TableData

	// render & restore ipv4 table data
	data = make([]TableData, 0)
	for _, d := range m.dataV4 {
		data = append(data, d)
	}
	m.renderAndRestoreTable(data, ProtocolFamilyIPv4)

	// render & restore ipv6 table data
	data = make([]TableData, 0)
	for _, d := range m.dataV6 {
		data = append(data, d)
	}
	m.renderAndRestoreTable(data, ProtocolFamilyIPv6)
}

func (m *Manager) renderAndRestoreTable(data []TableData, protocolFamily ProtocolFamily) {
	//########################################################################
	reader, writer := io.Pipe()
	errChan := make(chan error, 1)
	go func() {
		errChan <- m.template.ExecuteTemplate(writer, "Template", data)
		_ = writer.Close()
	}()

	//########################################################################

	runner := exec.New()
	cmd := runner.Command(getIptablesRestoreCmd(protocolFamily))
	cmd.SetStdin(reader)

	output, err := cmd.CombinedOutput()
	if err != nil {
		klog.Fatalf("unable to write iptable rules output: %s error: %e", string(output), err)
	}
	_ = reader.Close()

}

func NeedQuotes(option MatchModuleOption) bool {
	return option == MatchModuleCommentOptionComment
}

func IsDefaultChain(chain Chain) bool {
	for i := 0; i < len(DefaultChains); i++ {
		if chain == DefaultChains[i] {
			return true
		}
	}
	return false
}
