# Structure of the IPTables Backend

The IPTables backend for KPNG is derived from the upstream kubernetes
iptables implementation.  

To make the upstream implementation work in `kpng`, we implement the `decoder` interface.

The decoder sends new Kubernetes events down to the iptables backend and
the job of the IPTables backend is then to write iptables proxying rules using
the same logic as Kubernetes upstream kube-proxy does.

## The Sink object sends information to the iptables backend
The Sink object is defined in the kpng decoder package.

Its job is to tell the iptables backend, which implements the Decoder interface,
to "do stuff".

A decoder is an object which recieves information about changes to the networking
topology in Kubernetes (services, and endpoints) and then acts on that topology.  The specific 
functions implemented by a decoder are:
- `SetService`
- `DeleteService`
- `SetEndpoint`
- `DeleteEndpoint`
- `Setup`
- `WaitRequest`
- `Reset`

The decoder package has a `Sink` object, which is responsible for calling these
functions when different events happen in the Kubernetes API.

Make sure not to confuse `Sink`, the machinery which processes upstream Kubernetes events and
sends them to backends, with `Sync`, the downstream backends which ultimately need to get
synchronized periodically for implementing the service routing rules using (i.e. iptables implementation
is done in the `Sync` call, whereas the `Sink` object is the "thing" that recieves events
over GRPC from the Kpng server and acts on them).

## Domain specific logic for managing IPTables routing rules: iptables.go 

Since every backend in KPNG is independent of the 'frontend' which tracks changes in the apiserver.

Thus the `Backend` interface in KPNG has a Setup implementation which allows a Kpng backend to set itself
up, one time, when it is being created.

Someone calling iptables needs to make an `EndpointChangeTracker` and a `ServiceChangeTracker`.  
These objects then write the internal data of the `iptables` struct.   Periodically, the changes
are read in during the `sync()` method.

## Implementation of the Decoder interface: sink.go

- Methods for the KPNG `Backend` include 
    - `Sink`: Creates a decoder, and providers it to a new filterreset, with the iptables backend as the `Decoder` implementation.
    - `BindFlags`: not implmented, but binds any configuration we send in.
    - `Setup`: Creates ipv4 and ip6 implementations of the `Iptables` proxier, and `serviceChange` and `endpointChange` objects.
      - `serviceChange` and `endpointChange` both make NewServiceChangeTracker and EndpointChangeTracker objects.
      - Ultimately it writes to the array of implementations : `IptablesImpl[protocol] = iptable`
    - `Reset`: not implemented 
    - `Sync`: runs `sync()` on each of the IPtables implementations (v4, v6)
    - Endpoint and Service management 
    - Any KPNG backend must ultimately deal with two events: creation of services and endpoints.  The Backend struct 
    for iptables thus has Set/Delete functions which are triggered by the KPNG control server, for these two types.
    These can be thought of as the interface between a Kubernetes watch and the iptables backend.
      - `SetService`/`DeleteService`: Calling of the `Update`/`Delete` functions on the `serviceChanges` datastructure
      - `SetEndpoint`/`DeleteEndpoint`: Same as above, but for Endpoints 