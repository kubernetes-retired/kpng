package ipvs

import (
	"errors"
	"fmt"
	IPVSLib "github.com/google/seesaw/ipvs"
	"github.com/vishvananda/netlink"
	"k8s.io/klog/v2"
	"net"
	"sigs.k8s.io/kpng/client/diffstore"
)

// Manager acts as a proxy between backend and IPVS operations, leverages diffstore to maintain
// state, executes only the changes when triggered by backend.
type Manager struct {
	// scheduling method for virtual server
	schedulingMethod string

	// weight for ipvs destinations
	weight int32

	// interface on host where ips will bound
	ipInterface string

	// store for virtual servers
	serverStore *diffstore.Store[string, *diffstore.AnyLeaf[*VirtualServer]]

	// store for destinations
	destinationStore *diffstore.Store[string, *diffstore.AnyLeaf[*Destination]]

	// store of ip which are to be bound to host network interface
	ipBindStore *diffstore.Store[string, *diffstore.AnyLeaf[string]]
}

func NewManager(schedulingMethod string, weight int32, ipInterface string) *Manager {
	return &Manager{
		weight:           weight,
		schedulingMethod: schedulingMethod,
		ipInterface:      ipInterface,

		// create service diffstore with nil pointer safe equality assertion.
		serverStore: diffstore.NewAnyStore[string, *VirtualServer](func(a, b *VirtualServer) bool {
			// TODO this is fixed now, get rid of this
			if a != nil && b != nil {
				return a.Equal(b)
			}
			return true
		}),

		// create destination diffstore with nil pointer safe equality assertion.
		destinationStore: diffstore.NewAnyStore[string, *Destination](func(a, b *Destination) bool {
			if a != nil && b != nil {
				return a.Equal(b)
			}
			return true
		}),

		// create ip binding diffstore.
		ipBindStore: diffstore.NewAnyStore[string, string](func(a, b string) bool { return a == b }),
	}
}

// ApplyServer instead of directly creating the server, adds the server config to the
// service diffstore, actions will be taken only in case of create, update and delete.
func (m *Manager) ApplyServer(virtualServer *VirtualServer) {
	m.serverStore.Get(virtualServer.Key()).Set(virtualServer)
}

// AddDestination instead of directly adding destination to virtual server, adds destination to
// destination diffstore, actions will be taken only in case of create, update and delete.
func (m *Manager) AddDestination(destination *Destination, virtualServer *VirtualServer) {
	// attach server config to destination to avoid lookup search later
	destination.virtualServer = virtualServer
	m.destinationStore.Get(destination.Key()).Set(destination)
}

// BindServerToInterface instead of directly binding ip to interface, adds it to the ip store.
// actions will be taken only in case of create and delete.
func (m *Manager) BindServerToInterface(server *VirtualServer) {
	m.ipBindStore.Get(server.IP).Set(server.IP)
}

// Reset resets all the diffstores, should be called
// before processing the fullstate callback.
func (m *Manager) Reset() {
	m.serverStore.Reset()
	m.destinationStore.Reset()
	m.ipBindStore.Reset()
}

// Done calls Done on all diffstores for computing diffs.
func (m *Manager) Done() {

	m.serverStore.Done()
	m.destinationStore.Done()
	m.ipBindStore.Done()
}

// Apply has side effects. Apply should be called after processing fullstate callback, done will iterate
// over changes from all the diffstores and create, update and delete required objects accordingly.
func (m *Manager) Apply() {
	var err error

	// unbind IPs from the network interface.
	for _, item := range m.ipBindStore.Deleted() {
		ip := item.Value().Get()
		klog.V(4).Infof("removing IP %s from %s interface", ip, m.ipInterface)
		err = m.unbindIpFromInterface(ip)

		if err != nil {
			klog.V(2).ErrorS(err, "failed to remove IP from interface",
				"ip", ip, "interface", m.ipInterface)
		}
	}

	// remove destinations which are no longer part of virtual server
	for _, item := range m.destinationStore.Deleted() {
		destination := item.Value().Get()
		virtualServer := destination.virtualServer

		klog.V(4).Infof("deleting destination [%s] from the server [%s]",
			destination.IPPort(), virtualServer.IPPort())

		err = IPVSLib.DeleteDestination(
			virtualServer.asIPVSLibService(m.schedulingMethod),
			destination.asIPVSLibDestination(m.weight),
		)

		if err != nil {
			klog.V(2).ErrorS(err, "failed to remove destination from server",
				"server", virtualServer.IPPort(), "destination", destination.IPPort())
		}
	}

	// delete virtual servers which are no longer required
	for _, item := range m.serverStore.Deleted() {
		virtualServer := item.Value().Get()
		klog.V(4).Infof("deleting server [%s]", virtualServer.IPPort())

		err = IPVSLib.DeleteService(virtualServer.asIPVSLibService(m.schedulingMethod))

		if err != nil {
			klog.V(2).ErrorS(err, "failed to delete server", "server", virtualServer.IPPort())
		}
	}

	// handled new and updated servers
	for _, item := range m.serverStore.Changed() {
		virtualServer := item.Value().Get()

		if item.Created() {
			klog.V(4).Infof("creating server [%s]", virtualServer.Key())

			// create virtual server
			err = IPVSLib.AddService(virtualServer.asIPVSLibService(m.schedulingMethod))

			if err != nil {
				klog.V(2).ErrorS(err, "failed to create server", "server", virtualServer.IPPort())
			}
		} else if item.Updated() {

			klog.V(4).Infof("updating server [%s]", virtualServer.IPPort())

			// update virtual server
			err = IPVSLib.UpdateService(virtualServer.asIPVSLibService(m.schedulingMethod))

			if err != nil {
				klog.V(2).ErrorS(err, "failed to update server", "server", virtualServer.IPPort())
			}

		}
	}

	// handled new and updated destinations
	for _, item := range m.destinationStore.Changed() {
		destination := item.Value().Get()
		virtualServer := destination.virtualServer

		if item.Created() {
			// add destination to virtual server
			klog.V(4).Infof("adding destination [%s] to server [%s]",
				destination.IPPort(), virtualServer.IPPort())

			err = IPVSLib.AddDestination(
				virtualServer.asIPVSLibService(m.schedulingMethod),
				destination.asIPVSLibDestination(m.weight),
			)

			if err != nil {
				klog.V(2).ErrorS(err, "failed to add destination to server",
					"server", virtualServer.IPPort(), "destination", destination.IPPort())
			}
		} else if item.Updated() {
			// update destination of virtual server
			klog.V(4).Infof("updating destination [%s] of server [%s]",
				destination.IPPort(), virtualServer.IPPort())

			err = IPVSLib.UpdateDestination(
				virtualServer.asIPVSLibService(m.schedulingMethod),
				destination.asIPVSLibDestination(m.weight),
			)

			if err != nil {
				klog.V(2).ErrorS(err, "failed to update destination of server",
					"server", virtualServer.IPPort(), "destination", destination.IPPort())
			}

		}
	}

	// bind IPs to network interface
	for _, item := range m.ipBindStore.Changed() {
		ip := item.Value().Get()
		klog.V(4).Infof("adding IP [%s] to %s interface", m.ipInterface, ip)
		err = m.bindIpToInterface(ip)

		if err != nil {
			klog.V(2).ErrorS(err, "failed to add ip to interface",
				"ip", ip, "interface", m.ipInterface)
		}
	}

}

func asDummyIP(ip string) string {
	return ip + "/32"
}

// bindIpToInterface adds IP address to the network interface.
func (m *Manager) bindIpToInterface(ip string) error {
	_, ipNet, _ := net.ParseCIDR(asDummyIP(ip))

	ipInterface, err := netlink.LinkByName(m.ipInterface)

	if err != nil {
		return err
	}

	if ipInterface != nil {
		return netlink.AddrAdd(ipInterface, &netlink.Addr{IPNet: ipNet})
	}

	return errors.New(fmt.Sprintf("interface %s not found", m.ipInterface))
}

// bindIpToInterface removes IP address from the network interface.
func (m *Manager) unbindIpFromInterface(ip string) error {
	_, ipNet, _ := net.ParseCIDR(asDummyIP(ip))

	ipInterface, err := netlink.LinkByName(m.ipInterface)

	if err != nil {
		return err
	}

	if ipInterface != nil {
		return netlink.AddrDel(ipInterface, &netlink.Addr{IPNet: ipNet})
	}

	return errors.New(fmt.Sprintf("interface %s not found", m.ipInterface))
}
