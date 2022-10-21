/*
Copyright 2021 The Kubernetes Authors.

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

package ipvssink

import (
	"fmt"
	"strings"
	"testing"

	"github.com/lithammer/dedent"
	"github.com/stretchr/testify/assert"
)

// parseIPTablesData takes iptables-save output and returns a map of table name to array of lines.
func parseIPTablesData(ruleData string) (map[string][]string, error) {
	// Split ruleData at the "COMMIT" lines; given valid input, this will result in
	// one element for each table plus an extra empty element (since the ruleData
	// should end with a "COMMIT" line).
	rawTables := strings.Split(strings.TrimPrefix(ruleData, "\n"), "COMMIT\n")
	nTables := len(rawTables) - 1
	if nTables < 2 || rawTables[nTables] != "" {
		return nil, fmt.Errorf("bad ruleData (%d tables)\n%s", nTables, ruleData)
	}

	tables := make(map[string][]string, nTables)
	for i, table := range rawTables[:nTables] {
		lines := strings.Split(strings.Trim(table, "\n"), "\n")
		// The first line should be, eg, "*nat" or "*filter"
		if lines[0][0] != '*' {
			return nil, fmt.Errorf("bad ruleData (table %d starts with %q)", i+1, lines[0])
		}
		// add back the "COMMIT" line that got eaten by the strings.Split above
		lines = append(lines, "COMMIT")
		tables[lines[0][1:]] = lines
	}

	if tables["nat"] == nil {
		return nil, fmt.Errorf("bad ruleData (no %q table)", "nat")
	}
	if tables["filter"] == nil {
		return nil, fmt.Errorf("bad ruleData (no %q table)", "filter")
	}
	return tables, nil
}

func TestParseIPTablesData(t *testing.T) {
	for _, tc := range []struct {
		name   string
		input  string
		output map[string][]string
		error  string
	}{
		{
			name: "basic test",
			input: dedent.Dedent(`
				*filter
				:KUBE-SERVICES - [0:0]
				:KUBE-EXTERNAL-SERVICES - [0:0]
				:KUBE-FORWARD - [0:0]
				:KUBE-NODEPORTS - [0:0]
				-A KUBE-NODEPORTS -m comment --comment "ns2/svc2:p80 health check node port" -m tcp -p tcp --dport 30000 -j ACCEPT
				-A KUBE-FORWARD -m conntrack --ctstate INVALID -j DROP
				-A KUBE-FORWARD -m comment --comment "kubernetes forwarding rules" -m mark --mark 0x4000/0x4000 -j ACCEPT
				-A KUBE-FORWARD -m comment --comment "kubernetes forwarding conntrack rule" -m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT
				COMMIT
				*nat
				:KUBE-SERVICES - [0:0]
				:KUBE-NODEPORTS - [0:0]
				:KUBE-POSTROUTING - [0:0]
				:KUBE-MARK-MASQ - [0:0]
				:KUBE-SVC-XPGD46QRK7WJZT7O - [0:0]
				:KUBE-SEP-SXIVWICOYRO3J4NJ - [0:0]
				-A KUBE-POSTROUTING -m mark ! --mark 0x4000/0x4000 -j RETURN
				-A KUBE-POSTROUTING -j MARK --xor-mark 0x4000
				-A KUBE-POSTROUTING -m comment --comment "kubernetes service traffic requiring SNAT" -j MASQUERADE
				-A KUBE-MARK-MASQ -j MARK --or-mark 0x4000
				-A KUBE-SERVICES -m comment --comment "ns1/svc1:p80 cluster IP" -m tcp -p tcp -d 10.20.30.41 --dport 80 -j KUBE-SVC-XPGD46QRK7WJZT7O
				-A KUBE-SVC-XPGD46QRK7WJZT7O -m comment --comment "ns1/svc1:p80 cluster IP" -m tcp -p tcp -d 10.20.30.41 --dport 80 ! -s 10.0.0.0/24 -j KUBE-MARK-MASQ
				-A KUBE-SVC-XPGD46QRK7WJZT7O -m comment --comment ns1/svc1:p80 -j KUBE-SEP-SXIVWICOYRO3J4NJ
				-A KUBE-SEP-SXIVWICOYRO3J4NJ -m comment --comment ns1/svc1:p80 -s 10.180.0.1 -j KUBE-MARK-MASQ
				-A KUBE-SEP-SXIVWICOYRO3J4NJ -m comment --comment ns1/svc1:p80 -m tcp -p tcp -j DNAT --to-destination 10.180.0.1:80
				-A KUBE-SERVICES -m comment --comment "kubernetes service nodeports; NOTE: this must be the last rule in this chain" -m addrtype --dst-type LOCAL -j KUBE-NODEPORTS
				COMMIT
				`),
			output: map[string][]string{
				"filter": {
					`*filter`,
					`:KUBE-SERVICES - [0:0]`,
					`:KUBE-EXTERNAL-SERVICES - [0:0]`,
					`:KUBE-FORWARD - [0:0]`,
					`:KUBE-NODEPORTS - [0:0]`,
					`-A KUBE-NODEPORTS -m comment --comment "ns2/svc2:p80 health check node port" -m tcp -p tcp --dport 30000 -j ACCEPT`,
					`-A KUBE-FORWARD -m conntrack --ctstate INVALID -j DROP`,
					`-A KUBE-FORWARD -m comment --comment "kubernetes forwarding rules" -m mark --mark 0x4000/0x4000 -j ACCEPT`,
					`-A KUBE-FORWARD -m comment --comment "kubernetes forwarding conntrack rule" -m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT`,
					`COMMIT`,
				},
				"nat": {
					`*nat`,
					`:KUBE-SERVICES - [0:0]`,
					`:KUBE-NODEPORTS - [0:0]`,
					`:KUBE-POSTROUTING - [0:0]`,
					`:KUBE-MARK-MASQ - [0:0]`,
					`:KUBE-SVC-XPGD46QRK7WJZT7O - [0:0]`,
					`:KUBE-SEP-SXIVWICOYRO3J4NJ - [0:0]`,
					`-A KUBE-POSTROUTING -m mark ! --mark 0x4000/0x4000 -j RETURN`,
					`-A KUBE-POSTROUTING -j MARK --xor-mark 0x4000`,
					`-A KUBE-POSTROUTING -m comment --comment "kubernetes service traffic requiring SNAT" -j MASQUERADE`,
					`-A KUBE-MARK-MASQ -j MARK --or-mark 0x4000`,
					`-A KUBE-SERVICES -m comment --comment "ns1/svc1:p80 cluster IP" -m tcp -p tcp -d 10.20.30.41 --dport 80 -j KUBE-SVC-XPGD46QRK7WJZT7O`,
					`-A KUBE-SVC-XPGD46QRK7WJZT7O -m comment --comment "ns1/svc1:p80 cluster IP" -m tcp -p tcp -d 10.20.30.41 --dport 80 ! -s 10.0.0.0/24 -j KUBE-MARK-MASQ`,
					`-A KUBE-SVC-XPGD46QRK7WJZT7O -m comment --comment ns1/svc1:p80 -j KUBE-SEP-SXIVWICOYRO3J4NJ`,
					`-A KUBE-SEP-SXIVWICOYRO3J4NJ -m comment --comment ns1/svc1:p80 -s 10.180.0.1 -j KUBE-MARK-MASQ`,
					`-A KUBE-SEP-SXIVWICOYRO3J4NJ -m comment --comment ns1/svc1:p80 -m tcp -p tcp -j DNAT --to-destination 10.180.0.1:80`,
					`-A KUBE-SERVICES -m comment --comment "kubernetes service nodeports; NOTE: this must be the last rule in this chain" -m addrtype --dst-type LOCAL -j KUBE-NODEPORTS`,
					`COMMIT`,
				},
			},
		},
		{
			name: "not enough tables",
			input: dedent.Dedent(`
				*filter
				:KUBE-SERVICES - [0:0]
				:KUBE-EXTERNAL-SERVICES - [0:0]
				:KUBE-FORWARD - [0:0]
				:KUBE-NODEPORTS - [0:0]
				-A KUBE-NODEPORTS -m comment --comment "ns2/svc2:p80 health check node port" -m tcp -p tcp --dport 30000 -j ACCEPT
				-A KUBE-FORWARD -m conntrack --ctstate INVALID -j DROP
				-A KUBE-FORWARD -m comment --comment "kubernetes forwarding rules" -m mark --mark 0x4000/0x4000 -j ACCEPT
				-A KUBE-FORWARD -m comment --comment "kubernetes forwarding conntrack rule" -m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT
				COMMIT
				`),
			error: "bad ruleData (1 tables)",
		},
		{
			name: "trailing junk",
			input: dedent.Dedent(`
				*filter
				:KUBE-SERVICES - [0:0]
				:KUBE-EXTERNAL-SERVICES - [0:0]
				:KUBE-FORWARD - [0:0]
				:KUBE-NODEPORTS - [0:0]
				-A KUBE-NODEPORTS -m comment --comment "ns2/svc2:p80 health check node port" -m tcp -p tcp --dport 30000 -j ACCEPT
				-A KUBE-FORWARD -m conntrack --ctstate INVALID -j DROP
				-A KUBE-FORWARD -m comment --comment "kubernetes forwarding rules" -m mark --mark 0x4000/0x4000 -j ACCEPT
				-A KUBE-FORWARD -m comment --comment "kubernetes forwarding conntrack rule" -m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT
				COMMIT
				*nat
				:KUBE-SERVICES - [0:0]
				:KUBE-EXTERNAL-SERVICES - [0:0]
				:KUBE-FORWARD - [0:0]
				:KUBE-NODEPORTS - [0:0]
				-A KUBE-NODEPORTS -m comment --comment "ns2/svc2:p80 health check node port" -m tcp -p tcp --dport 30000 -j ACCEPT
				-A KUBE-FORWARD -m conntrack --ctstate INVALID -j DROP
				-A KUBE-FORWARD -m comment --comment "kubernetes forwarding rules" -m mark --mark 0x4000/0x4000 -j ACCEPT
				-A KUBE-FORWARD -m comment --comment "kubernetes forwarding conntrack rule" -m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT
				COMMIT
				junk
				`),
			error: "bad ruleData (2 tables)",
		},
		{
			name: "bad start line",
			input: dedent.Dedent(`
				*filter
				:KUBE-SERVICES - [0:0]
				:KUBE-EXTERNAL-SERVICES - [0:0]
				:KUBE-FORWARD - [0:0]
				:KUBE-NODEPORTS - [0:0]
				-A KUBE-NODEPORTS -m comment --comment "ns2/svc2:p80 health check node port" -m tcp -p tcp --dport 30000 -j ACCEPT
				-A KUBE-FORWARD -m conntrack --ctstate INVALID -j DROP
				-A KUBE-FORWARD -m comment --comment "kubernetes forwarding rules" -m mark --mark 0x4000/0x4000 -j ACCEPT
				-A KUBE-FORWARD -m comment --comment "kubernetes forwarding conntrack rule" -m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT
				COMMIT
				:KUBE-SERVICES - [0:0]
				:KUBE-EXTERNAL-SERVICES - [0:0]
				:KUBE-FORWARD - [0:0]
				:KUBE-NODEPORTS - [0:0]
				-A KUBE-NODEPORTS -m comment --comment "ns2/svc2:p80 health check node port" -m tcp -p tcp --dport 30000 -j ACCEPT
				-A KUBE-FORWARD -m conntrack --ctstate INVALID -j DROP
				-A KUBE-FORWARD -m comment --comment "kubernetes forwarding rules" -m mark --mark 0x4000/0x4000 -j ACCEPT
				-A KUBE-FORWARD -m comment --comment "kubernetes forwarding conntrack rule" -m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT
				COMMIT
				`),
			error: `bad ruleData (table 2 starts with ":KUBE-SERVICES - [0:0]")`,
		},
		{
			name: "no nat",
			input: dedent.Dedent(`
				*filter
				:KUBE-SERVICES - [0:0]
				:KUBE-EXTERNAL-SERVICES - [0:0]
				:KUBE-FORWARD - [0:0]
				:KUBE-NODEPORTS - [0:0]
				-A KUBE-NODEPORTS -m comment --comment "ns2/svc2:p80 health check node port" -m tcp -p tcp --dport 30000 -j ACCEPT
				-A KUBE-FORWARD -m conntrack --ctstate INVALID -j DROP
				-A KUBE-FORWARD -m comment --comment "kubernetes forwarding rules" -m mark --mark 0x4000/0x4000 -j ACCEPT
				-A KUBE-FORWARD -m comment --comment "kubernetes forwarding conntrack rule" -m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT
				COMMIT
				*mangle
				:KUBE-SERVICES - [0:0]
				:KUBE-EXTERNAL-SERVICES - [0:0]
				:KUBE-FORWARD - [0:0]
				:KUBE-NODEPORTS - [0:0]
				-A KUBE-NODEPORTS -m comment --comment "ns2/svc2:p80 health check node port" -m tcp -p tcp --dport 30000 -j ACCEPT
				-A KUBE-FORWARD -m conntrack --ctstate INVALID -j DROP
				-A KUBE-FORWARD -m comment --comment "kubernetes forwarding rules" -m mark --mark 0x4000/0x4000 -j ACCEPT
				-A KUBE-FORWARD -m comment --comment "kubernetes forwarding conntrack rule" -m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT
				COMMIT
				`),
			error: `bad ruleData (no "nat" table)`,
		},
		{
			name: "no filter",
			input: dedent.Dedent(`
				*mangle
				:KUBE-SERVICES - [0:0]
				:KUBE-EXTERNAL-SERVICES - [0:0]
				:KUBE-FORWARD - [0:0]
				:KUBE-NODEPORTS - [0:0]
				-A KUBE-NODEPORTS -m comment --comment "ns2/svc2:p80 health check node port" -m tcp -p tcp --dport 30000 -j ACCEPT
				-A KUBE-FORWARD -m conntrack --ctstate INVALID -j DROP
				-A KUBE-FORWARD -m comment --comment "kubernetes forwarding rules" -m mark --mark 0x4000/0x4000 -j ACCEPT
				-A KUBE-FORWARD -m comment --comment "kubernetes forwarding conntrack rule" -m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT
				COMMIT
				*nat
				:KUBE-SERVICES - [0:0]
				:KUBE-EXTERNAL-SERVICES - [0:0]
				:KUBE-FORWARD - [0:0]
				:KUBE-NODEPORTS - [0:0]
				-A KUBE-NODEPORTS -m comment --comment "ns2/svc2:p80 health check node port" -m tcp -p tcp --dport 30000 -j ACCEPT
				-A KUBE-FORWARD -m conntrack --ctstate INVALID -j DROP
				-A KUBE-FORWARD -m comment --comment "kubernetes forwarding rules" -m mark --mark 0x4000/0x4000 -j ACCEPT
				-A KUBE-FORWARD -m comment --comment "kubernetes forwarding conntrack rule" -m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT
				COMMIT
				`),
			error: `bad ruleData (no "filter" table)`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, err := parseIPTablesData(tc.input)
			if err == nil {
				if tc.error != "" {
					t.Errorf("unexpectedly did not get error")
				} else {
					assert.Equal(t, tc.output, out)
				}
			} else {
				if tc.error == "" {
					t.Errorf("got unexpected error: %v", err)
				} else if !strings.HasPrefix(err.Error(), tc.error) {
					t.Errorf("got wrong error: %v (expected %q)", err, tc.error)
				}
			}
		})
	}
}
