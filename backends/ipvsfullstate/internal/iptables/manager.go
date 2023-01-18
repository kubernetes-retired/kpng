package iptables

import (
	"fmt"
	"io"
	"k8s.io/utils/exec"
	"text/template"
)

const iptablesRestoreCmd = "iptables-restore"

var DefaultChains = []Chain{ChainPreRouting, ChainInput, ChainForward, ChainOutput, ChainPostRouting}

type Manager struct {
	data     map[Table]TableData
	template *template.Template
}

func NewManager() *Manager {
	data := map[Table]TableData{
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
	fmt.Println(err)

	iptTemplate, err = iptTemplate.New("TableTemplate").Parse(TableTemplate)
	fmt.Println(err)

	iptTemplate, err = iptTemplate.New("ChainTemplate").Parse(ChainTemplate)
	fmt.Println(err)

	iptTemplate, err = iptTemplate.New("RuleTemplate").Parse(RuleTemplate)
	fmt.Println(err)

	iptTemplate, err = iptTemplate.New("MatchTemplate").Parse(MatchTemplate)
	fmt.Println(err)

	iptTemplate, err = iptTemplate.New("ProtocolTemplate").Parse(ProtocolTemplate)
	fmt.Println(err)

	return &Manager{
		data:     data,
		template: iptTemplate,
	}
}

func (m *Manager) AddChain(chain Chain, table Table) {
	tableData, _ := m.data[table]
	tableData.Chains = append(tableData.Chains, chain)
	m.data[table] = tableData
}

func (m *Manager) AddRule(rule Rule, table Table) {
	tableData, _ := m.data[table]
	tableData.Rules = append(tableData.Rules, rule)
	m.data[table] = tableData
}

func (m *Manager) Apply() {
	data := make([]TableData, 0)
	for _, d := range m.data {
		data = append(data, d)
	}

	//########################################################################
	reader, writer := io.Pipe()
	errChan := make(chan error, 1)
	go func() {
		errChan <- m.template.ExecuteTemplate(writer, "Template", data)
		_ = writer.Close()
	}()

	//########################################################################

	runner := exec.New()
	cmd := runner.Command(iptablesRestoreCmd)
	cmd.SetStdin(reader)

	result, _ := cmd.CombinedOutput()
	fmt.Println(string(result))

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
