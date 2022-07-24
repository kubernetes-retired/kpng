package localnetv1

func (p *PortMapping) SrcPorts() []int32 {
	switch {
	case p.Port == 0 && p.NodePort == 0:
		return []int32{}
	case p.Port != 0 && p.NodePort == 0:
		return []int32{p.Port}
	case p.Port == 0 && p.NodePort != 0:
		return []int32{p.NodePort}
	case p.Port != 0 && p.NodePort != 0:
		return []int32{p.Port, p.NodePort}
	}
	panic("unreachable")
}
