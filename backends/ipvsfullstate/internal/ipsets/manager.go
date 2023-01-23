package ipsets

import (
	"k8s.io/klog/v2"
	"k8s.io/utils/exec"
	"sigs.k8s.io/kpng/client/diffstore"
)

// Manager acts as a proxy between backend and IPSET operations, leverages diffstore to maintain
// state, executes only the changes when triggered by backend.
type Manager struct {
	ipsetMap map[string]*Set

	//setStore *diffstore.Store[string, *diffstore.AnyLeaf[*Set]]

	entryStore *diffstore.Store[string, *diffstore.AnyLeaf[*Entry]]
}

func NewManager() *Manager {
	return &Manager{
		// set store is not required, since all the sets will be created at
		// initialization only
		//setStore: diffstore.NewAnyStore[string, *Set](func(a, b *Set) bool { return true }),

		// hard coding the equality assertion to true, since for ipsets we are only
		// interested in creates and deletes only.
		entryStore: diffstore.NewAnyStore[string, *Entry](func(a, b *Entry) bool { return true }),

		// map with set name as key and set as value
		ipsetMap: make(map[string]*Set),
	}
}

func (m *Manager) Reset() {
	//m.setStore.Reset()
	m.entryStore.Reset()
}

// CreateSet doesn't use diffstore, straightaway creates the set and add it to ipsetMap.
func (m *Manager) CreateSet(name string, setType SetType, comment string) (*Set, error) {
	set := newIPSet(New(exec.New()), name, setType, ProtocolFamilyIPV4, comment)
	m.ipsetMap[name] = set
	return set, ensureIPSet(set)
}

// AddEntry instead of directly adding entry to ipset, adds it to entry
// diffstore, actions will be taken only in case of create and delete.
func (m *Manager) AddEntry(entry *Entry, set *Set) {
	entry.set = set

	// since we only need create and delete operations here,
	// setting the key to absolute value of entry.
	m.entryStore.Get(entry.String()).Set(entry)
}

// GetSetByName returns all sets by set name.
func (m *Manager) GetSetByName(setName string) *Set {
	if _, ok := m.ipsetMap[setName]; ok {
		return m.ipsetMap[setName]
	}
	return nil
}

// Done calls Done on all diffstores for computing diffs.
func (m *Manager) Done() {
	m.entryStore.Done()
}

// Apply has side effects. Apply should be called after processing fullstate callback, done will iterate
// over changes from all the diffstores and create, update and delete required objects accordingly.
func (m *Manager) Apply() {
	var err error
	var valid bool

	// add new entries to ipsets here.
	for _, item := range m.entryStore.Changed() {
		entry := item.Value().Get()

		klog.V(4).Infof("validating entry [%s] for set [%s]", entry.String(), entry.set.GetName())
		valid = entry.set.validateEntry(entry)

		if !valid {
			klog.V(2).ErrorS(err, "invalid entry for set",
				"entry", entry.String(), "set", entry.set.GetName())
		}

		klog.V(4).Infof("adding entry [%s] to set [%s]", entry.String(), entry.set.GetName())
		err = entry.set.addEntry(entry)
		if err != nil {
			klog.V(2).ErrorS(err, "failed to add entry to set",
				"entry", entry.String(), "set", entry.set.GetName())
		}

	}

	// remove entries from the ipsets here.
	for _, item := range m.entryStore.Deleted() {
		entry := item.Value().Get()

		klog.V(4).Infof("validating entry [%s] for set [%s]", entry.String(), entry.set.GetName())
		valid = entry.set.validateEntry(entry)
		if !valid {
			klog.V(2).ErrorS(err, "invalid entry for set",
				"entry", entry.String(), "set", entry.set.GetName())
		}

		klog.V(4).Infof("removing entry [%s] from set [%s]", entry.String(), entry.set.GetName())
		err = entry.set.delEntry(entry)
		if err != nil {
			klog.V(2).ErrorS(err, "failed to remove entry from set",
				"entry", entry.String(), "set", entry.set.GetName())
		}
	}
}
