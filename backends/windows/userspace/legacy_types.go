package userspace

import (
	"fmt"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/kpng/api/localnetv1"
)

// ServicePortPortalName carries a namespace + name + portname + portalip.  This is the unique
// identifier for a windows service port portal.
type ServicePortPortalName struct {
	types.NamespacedName
	Port         string
	PortalIPName string
}

func (spn ServicePortPortalName) String() string {
	return fmt.Sprintf("%s:%s:%s", spn.NamespacedName.String(), spn.Port, spn.PortalIPName)
}

// ServicePortName carries a namespace + name + portname.  This is the unique
// identifier for a load-balanced service.
type ServicePortName struct {
	types.NamespacedName
	Port     string
	Protocol localnetv1.Protocol
}

func (spn ServicePortName) String() string {
	return fmt.Sprintf("%s%s", spn.NamespacedName.String(), fmtPortName(spn.Port))
}

func fmtPortName(in string) string {
	if in == "" {
		return ""
	}
	return fmt.Sprintf(":%s", in)
}
