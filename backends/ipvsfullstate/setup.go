package ipvsfullsate

import (
	IPVS "github.com/google/seesaw/ipvs"
	"github.com/vishvananda/netlink"
	"k8s.io/klog/v2"
	"net"
)

const dummyName = "kube-ipvs0"

func (b *backend) Setup() {
	var err error
	err = IPVS.Init()
	if err != nil {
		klog.Fatal("Unable to initialize ipvs interface")
	}

	dummy, err := createIPVSDummyInterface()
	if err != nil {
		klog.Fatal("failed to initialize dummy interface")
	}

	controller = NewIPVSController(dummy)
	klog.V(4).Info("IPVS controller initialized")
}

func createIPVSDummyInterface() (netlink.Link, error) {
	// populate dummyIPs

	dummy, err := netlink.LinkByName(dummyName)
	if err != nil {
		if _, ok := err.(netlink.LinkNotFoundError); !ok {
			klog.Fatal("failed to get dummy interface: ", err)
			return nil, err
		}

		// not found => create the dummy
		dummy = &netlink.Dummy{
			LinkAttrs: netlink.LinkAttrs{Name: dummyName},
		}

		klog.Info("creating dummy interface ", dummyName)
		if err = netlink.LinkAdd(dummy); err != nil {
			klog.Fatal("failed to create dummy interface: ", err)
			return nil, err
		}

		dummy, err = netlink.LinkByName(dummyName)
		if err != nil {
			klog.Fatal("failed to get link after create: ", err)
			return nil, err
		}
	}

	if dummy.Attrs().Flags&net.FlagUp == 0 {
		klog.Info("setting dummy interface ", dummyName, " up")
		if err = netlink.LinkSetUp(dummy); err != nil {
			klog.Fatal("failed to set dummy interface up: ", err)
			return nil, err
		}
	}
	return dummy, nil
}
