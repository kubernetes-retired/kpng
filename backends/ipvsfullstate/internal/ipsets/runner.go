package ipsets

import (
	"bytes"
	"fmt"
	"k8s.io/utils/exec"
	"regexp"
	"strconv"
	"strings"
)

type runner struct {
	exec exec.Interface
}

// New returns a new Interface which will exec ipset.
func New(exec exec.Interface) Interface {
	return &runner{
		exec: exec,
	}
}

// CreateSet creates a new set, it will ignore error when the set already exists if ignoreExistErr=true.
func (runner *runner) CreateSet(set *IPSet, ignoreExistErr bool) error {
	// sets some IPSet fields if not present to their default values.
	set.setIPSetDefaults()

	// Validate ipset before creating
	valid := set.Validate()
	if !valid {
		return fmt.Errorf("error creating ipset since it's invalid")
	}
	return runner.createSet(set, ignoreExistErr)
}

// If ignoreExistErr is set to true, then the -exist option of ipset will be specified, ipset ignores the error
// otherwise raised when the same set (setname and create parameters are identical) already exists.
func (runner *runner) createSet(set *IPSet, ignoreExistErr bool) error {
	args := []string{"create", set.Name, string(set.SetType)}
	if set.SetType == HashIPPortIP || set.SetType == HashIPPort || set.SetType == HashIPPortNet {
		args = append(args,
			"family", set.HashFamily.String(),
			"hashsize", strconv.Itoa(set.HashSize),
			"maxelem", strconv.Itoa(set.MaxElem),
		)
	}
	if set.SetType == BitmapPort {
		args = append(args, "range", set.PortRange)
	}
	if ignoreExistErr {
		args = append(args, "-exist")
	}
	if _, err := runner.exec.Command(IPSetCmd, args...).CombinedOutput(); err != nil {
		return fmt.Errorf("error creating ipset %s, error: %v", set.Name, err)
	}
	return nil
}

// AddEntry adds a new entry to the named set.
// If the -exist option is specified, ipset ignores the error otherwise raised when
// the same set (setname and create parameters are identical) already exists.
func (runner *runner) AddEntry(entry string, set *IPSet, ignoreExistErr bool) error {
	args := []string{"add", set.Name, entry}
	if ignoreExistErr {
		args = append(args, "-exist")
	}
	if _, err := runner.exec.Command(IPSetCmd, args...).CombinedOutput(); err != nil {
		return fmt.Errorf("error adding entry %s, error: %v", entry, err)
	}
	return nil
}

// DelEntry is used to delete the specified entry from the set.
func (runner *runner) DelEntry(entry string, set string) error {
	if _, err := runner.exec.Command(IPSetCmd, "del", set, entry).CombinedOutput(); err != nil {
		return fmt.Errorf("error deleting entry %s: from set: %s, error: %v", entry, set, err)
	}
	return nil
}

// TestEntry is used to check whether the specified entry is in the set or not.
func (runner *runner) TestEntry(entry string, set string) (bool, error) {
	if out, err := runner.exec.Command(IPSetCmd, "test", set, entry).CombinedOutput(); err == nil {
		reg, e := regexp.Compile("is NOT in set " + set)
		if e == nil && reg.MatchString(string(out)) {
			return false, nil
		} else if e == nil {
			return true, nil
		} else {
			return false, fmt.Errorf("error testing entry: %s, error: %v", entry, e)
		}
	} else {
		return false, fmt.Errorf("error testing entry %s: %v (%s)", entry, err, out)
	}
}

// FlushSet deletes all entries from a named set.
func (runner *runner) FlushSet(set string) error {
	if _, err := runner.exec.Command(IPSetCmd, "flush", set).CombinedOutput(); err != nil {
		return fmt.Errorf("error flushing set: %s, error: %v", set, err)
	}
	return nil
}

// DestroySet is used to destroy a named set.
func (runner *runner) DestroySet(set string) error {
	if out, err := runner.exec.Command(IPSetCmd, "destroy", set).CombinedOutput(); err != nil {
		return fmt.Errorf("error destroying set %s, error: %v(%s)", set, err, out)
	}
	return nil
}

// DestroyAllSets is used to destroy all sets.
func (runner *runner) DestroyAllSets() error {
	if _, err := runner.exec.Command(IPSetCmd, "destroy").CombinedOutput(); err != nil {
		return fmt.Errorf("error destroying all sets, error: %v", err)
	}
	return nil
}

// ListSets list all set names from kernel
func (runner *runner) ListSets() ([]string, error) {
	out, err := runner.exec.Command(IPSetCmd, "list", "-n").CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("error listing all sets, error: %v", err)
	}
	return strings.Split(string(out), "\n"), nil
}

// ListEntries lists all the entries from a named set.
func (runner *runner) ListEntries(set string) ([]string, error) {
	if len(set) == 0 {
		return nil, fmt.Errorf("set name can't be nil")
	}
	out, err := runner.exec.Command(IPSetCmd, "list", set).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("error listing set: %s, error: %v", set, err)
	}
	memberMatcher := regexp.MustCompile(EntryMemberPattern)
	list := memberMatcher.ReplaceAllString(string(out[:]), "")
	strs := strings.Split(list, "\n")
	results := make([]string, 0)
	for i := range strs {
		if len(strs[i]) > 0 {
			results = append(results, strs[i])
		}
	}
	return results, nil
}

// GetVersion returns the version string.
func (runner *runner) GetVersion() (string, error) {
	return getIPSetVersionString(runner.exec)
}

// getIPSetVersionString runs "ipset --version" to get the version string
// in the form of "X.Y", i.e "6.19"
func getIPSetVersionString(exec exec.Interface) (string, error) {
	cmd := exec.Command(IPSetCmd, "--version")
	cmd.SetStdin(bytes.NewReader([]byte{}))
	bytes, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}
	versionMatcher := regexp.MustCompile(VersionPattern)
	match := versionMatcher.FindStringSubmatch(string(bytes))
	if match == nil {
		return "", fmt.Errorf("no ipset version found in string: %s", bytes)
	}
	return match[0], nil
}
