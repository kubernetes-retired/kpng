package localnetv1

func (s *ServiceIPs) All() (all *IPSet) {
	all = NewIPSet()
	all.AddSet(s.ClusterIPs)
	all.AddSet(s.ExternalIPs)
	all.AddSet(s.LoadBalancerIPs)
	return
}

func (s *ServiceIPs) AllIngress() (all *IPSet) {
	all = NewIPSet()
	all.AddSet(s.ExternalIPs)
	all.AddSet(s.LoadBalancerIPs)
	return
}
