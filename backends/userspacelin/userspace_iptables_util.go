package userspacelin

import (
	"bytes"
	"time"

	iptables "sigs.k8s.io/kpng/backends/iptables/util"
)

// Interface is an injectable interface for running iptables commands.  Implementations must be goroutine-safe.
type Interface interface {
	// EnsureChain checks if the specified chain exists and, if not, creates it.  If the chain existed, return true.
	EnsureChain(table iptables.Table, chain iptables.Chain) (bool, error)
	// FlushChain clears the specified chain.  If the chain did not exist, return error.
	FlushChain(table iptables.Table, chain iptables.Chain) error
	// DeleteChain deletes the specified chain.  If the chain did not exist, return error.
	DeleteChain(table iptables.Table, chain iptables.Chain) error
	// ChainExists tests whether the specified chain exists, returning an error if it
	// does not, or if it is unable to check.
	ChainExists(table iptables.Table, chain iptables.Chain) (bool, error)
	// EnsureRule checks if the specified rule is present and, if not, creates it.  If the rule existed, return true.
	EnsureRule(position iptables.RulePosition, table iptables.Table, chain iptables.Chain, args ...string) (bool, error)
	// DeleteRule checks if the specified rule is present and, if so, deletes it.
	DeleteRule(table iptables.Table, chain iptables.Chain, args ...string) error
	// IsIPv6 returns true if this is managing ipv6 tables.
	IsIPv6() bool
	// Protocol returns the IP family this instance is managing,
	Protocol() iptables.Protocol
	// SaveInto calls `iptables-save` for table and stores result in a given buffer.
	SaveInto(table iptables.Table, buffer *bytes.Buffer) error
	// Restore runs `iptables-restore` passing data through []byte.
	// table is the Table to restore
	// data should be formatted like the output of SaveInto()
	// flush sets the presence of the "--noflush" flag. see: FlushFlag
	// counters sets the "--counters" flag. see: RestoreCountersFlag
	Restore(table iptables.Table, data []byte, flush iptables.FlushFlag, counters iptables.RestoreCountersFlag) error
	// RestoreAll is the same as Restore except that no table is specified.
	RestoreAll(data []byte, flush iptables.FlushFlag, counters iptables.RestoreCountersFlag) error
	// Monitor detects when the given iptables tables have been flushed by an external
	// tool (e.g. a firewall reload) by creating canary chains and polling to see if
	// they have been deleted. (Specifically, it polls tables[0] every interval until
	// the canary has been deleted from there, then waits a short additional time for
	// the canaries to be deleted from the remaining tables as well. You can optimize
	// the polling by listing a relatively empty table in tables[0]). When a flush is
	// detected, this calls the reloadFunc so the caller can reload their own iptables
	// rules. If it is unable to create the canary chains (either initially or after
	// a reload) it will log an error and stop monitoring.
	// (This function should be called from a goroutine.)
	Monitor(canary iptables.Chain, tables []iptables.Table, reloadFunc func(), interval time.Duration, stopCh <-chan struct{})
	// HasRandomFully reveals whether `-j MASQUERADE` takes the
	// `--random-fully` option.  This is helpful to work around a
	// Linux kernel bug that sometimes causes multiple flows to get
	// mapped to the same IP:PORT and consequently some suffer packet
	// drops.
	HasRandomFully() bool
}
