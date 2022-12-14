package ipvsfullsate

func (c *IpvsController) createServicePortInfo(servicePortInfo *ServicePortInfo) {
	c.proxier.addVirtualServer(servicePortInfo)
}

func (c *IpvsController) updateServicePortInfo(servicePortInfo *ServicePortInfo) {
	c.deleteServicePortInfo(servicePortInfo)
	c.createServicePortInfo(servicePortInfo)
}

func (c *IpvsController) deleteServicePortInfo(servicePortInfo *ServicePortInfo) {
	c.proxier.deleteVirtualServer(servicePortInfo)
}

func (c *IpvsController) createEndpointInfo(servicePortInfo *ServicePortInfo, endpointInfo *EndPointInfo) {
	c.proxier.addRealServer(servicePortInfo, endpointInfo)
}

func (c *IpvsController) updateEndpointInfo(servicePortInfo *ServicePortInfo, endpointInfo *EndPointInfo) {
	c.deleteEndpointInfo(servicePortInfo, endpointInfo)
	c.createEndpointInfo(servicePortInfo, endpointInfo)
}

func (c *IpvsController) deleteEndpointInfo(servicePortInfo *ServicePortInfo, endpointInfo *EndPointInfo) {
	c.proxier.deleteRealServer(servicePortInfo, endpointInfo)
}
