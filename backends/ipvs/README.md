# IPVS Implementation

This implementation simply requests the creation of all the network objects required to program Kubernetes services data path for every callback, leverages diffstore to maintain the states of all the network components, and operates only on the changes returned by the diffstore.

## 1. Controller
Controller on initialization set sysctls for IPVS, creates dummy interface, initializes IPSets, writes required IPTable rules and most importantly handles the full state callback.
On processing full state controller interacts with **Managers** which acts as a proxy to the actual IPVS and IPSet resource creation.

- **addServiceEndpointsForClusterIP**
  The logic for programing a ClusterIP service
- **addServiceEndpointsForNodePort**
  The logic for programing a NodePort service
- **addServiceEndpointsForLoadBalancer**
  The logic for programing a LoadBalancer service


## 2. Managers
Manager leverages diffstore for storing all the resource manipulation operations (create virtual server, add destination, add entry to ipset) required to render the full state and only acts on the changes in the store.
- **ipvs**

  Resource definitions and methods for IPVS manipulation.
  IPVS manager holds virtual server and destination definitions
- **ipsets**

  Resource definitions and methods for IPSets manipulation.

  IPSets manager
- **iptables**

  Resource definitions and methods for IPTables manipulation.
