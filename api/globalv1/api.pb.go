// Copyright 2021 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.28.1
// 	protoc        v3.21.9
// source: api/globalv1/api.proto

package globalv1

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	localv1 "sigs.k8s.io/kpng/api/localv1"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type ServiceInfo struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Hash    uint64           `protobuf:"varint,1,opt,name=Hash,proto3" json:"Hash,omitempty"`
	Service *localv1.Service `protobuf:"bytes,2,opt,name=Service,proto3" json:"Service,omitempty"`
}

func (x *ServiceInfo) Reset() {
	*x = ServiceInfo{}
	if protoimpl.UnsafeEnabled {
		mi := &file_api_globalv1_api_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ServiceInfo) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ServiceInfo) ProtoMessage() {}

func (x *ServiceInfo) ProtoReflect() protoreflect.Message {
	mi := &file_api_globalv1_api_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ServiceInfo.ProtoReflect.Descriptor instead.
func (*ServiceInfo) Descriptor() ([]byte, []int) {
	return file_api_globalv1_api_proto_rawDescGZIP(), []int{0}
}

func (x *ServiceInfo) GetHash() uint64 {
	if x != nil {
		return x.Hash
	}
	return 0
}

func (x *ServiceInfo) GetService() *localv1.Service {
	if x != nil {
		return x.Service
	}
	return nil
}

type EndpointInfo struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Hash        uint64              `protobuf:"varint,1,opt,name=Hash,proto3" json:"Hash,omitempty"`
	Namespace   string              `protobuf:"bytes,2,opt,name=Namespace,proto3" json:"Namespace,omitempty"`
	SourceName  string              `protobuf:"bytes,3,opt,name=SourceName,proto3" json:"SourceName,omitempty"`
	ServiceName string              `protobuf:"bytes,4,opt,name=ServiceName,proto3" json:"ServiceName,omitempty"`
	PodName     string              `protobuf:"bytes,8,opt,name=PodName,proto3" json:"PodName,omitempty"`
	Endpoint    *localv1.Endpoint   `protobuf:"bytes,6,opt,name=Endpoint,proto3" json:"Endpoint,omitempty"`
	Conditions  *EndpointConditions `protobuf:"bytes,7,opt,name=Conditions,proto3" json:"Conditions,omitempty"`
	Topology    *TopologyInfo       `protobuf:"bytes,9,opt,name=Topology,proto3" json:"Topology,omitempty"`
	Hints       *TopologyHints      `protobuf:"bytes,10,opt,name=Hints,proto3" json:"Hints,omitempty"`
}

func (x *EndpointInfo) Reset() {
	*x = EndpointInfo{}
	if protoimpl.UnsafeEnabled {
		mi := &file_api_globalv1_api_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *EndpointInfo) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*EndpointInfo) ProtoMessage() {}

func (x *EndpointInfo) ProtoReflect() protoreflect.Message {
	mi := &file_api_globalv1_api_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use EndpointInfo.ProtoReflect.Descriptor instead.
func (*EndpointInfo) Descriptor() ([]byte, []int) {
	return file_api_globalv1_api_proto_rawDescGZIP(), []int{1}
}

func (x *EndpointInfo) GetHash() uint64 {
	if x != nil {
		return x.Hash
	}
	return 0
}

func (x *EndpointInfo) GetNamespace() string {
	if x != nil {
		return x.Namespace
	}
	return ""
}

func (x *EndpointInfo) GetSourceName() string {
	if x != nil {
		return x.SourceName
	}
	return ""
}

func (x *EndpointInfo) GetServiceName() string {
	if x != nil {
		return x.ServiceName
	}
	return ""
}

func (x *EndpointInfo) GetPodName() string {
	if x != nil {
		return x.PodName
	}
	return ""
}

func (x *EndpointInfo) GetEndpoint() *localv1.Endpoint {
	if x != nil {
		return x.Endpoint
	}
	return nil
}

func (x *EndpointInfo) GetConditions() *EndpointConditions {
	if x != nil {
		return x.Conditions
	}
	return nil
}

func (x *EndpointInfo) GetTopology() *TopologyInfo {
	if x != nil {
		return x.Topology
	}
	return nil
}

func (x *EndpointInfo) GetHints() *TopologyHints {
	if x != nil {
		return x.Hints
	}
	return nil
}

type EndpointConditions struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Ready bool `protobuf:"varint,1,opt,name=Ready,proto3" json:"Ready,omitempty"`
}

func (x *EndpointConditions) Reset() {
	*x = EndpointConditions{}
	if protoimpl.UnsafeEnabled {
		mi := &file_api_globalv1_api_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *EndpointConditions) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*EndpointConditions) ProtoMessage() {}

func (x *EndpointConditions) ProtoReflect() protoreflect.Message {
	mi := &file_api_globalv1_api_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use EndpointConditions.ProtoReflect.Descriptor instead.
func (*EndpointConditions) Descriptor() ([]byte, []int) {
	return file_api_globalv1_api_proto_rawDescGZIP(), []int{2}
}

func (x *EndpointConditions) GetReady() bool {
	if x != nil {
		return x.Ready
	}
	return false
}

type TopologyInfo struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Node string `protobuf:"bytes,1,opt,name=Node,proto3" json:"Node,omitempty"`
	Zone string `protobuf:"bytes,2,opt,name=Zone,proto3" json:"Zone,omitempty"`
}

func (x *TopologyInfo) Reset() {
	*x = TopologyInfo{}
	if protoimpl.UnsafeEnabled {
		mi := &file_api_globalv1_api_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *TopologyInfo) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*TopologyInfo) ProtoMessage() {}

func (x *TopologyInfo) ProtoReflect() protoreflect.Message {
	mi := &file_api_globalv1_api_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use TopologyInfo.ProtoReflect.Descriptor instead.
func (*TopologyInfo) Descriptor() ([]byte, []int) {
	return file_api_globalv1_api_proto_rawDescGZIP(), []int{3}
}

func (x *TopologyInfo) GetNode() string {
	if x != nil {
		return x.Node
	}
	return ""
}

func (x *TopologyInfo) GetZone() string {
	if x != nil {
		return x.Zone
	}
	return ""
}

type TopologyHints struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Zones []string `protobuf:"bytes,1,rep,name=Zones,proto3" json:"Zones,omitempty"`
}

func (x *TopologyHints) Reset() {
	*x = TopologyHints{}
	if protoimpl.UnsafeEnabled {
		mi := &file_api_globalv1_api_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *TopologyHints) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*TopologyHints) ProtoMessage() {}

func (x *TopologyHints) ProtoReflect() protoreflect.Message {
	mi := &file_api_globalv1_api_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use TopologyHints.ProtoReflect.Descriptor instead.
func (*TopologyHints) Descriptor() ([]byte, []int) {
	return file_api_globalv1_api_proto_rawDescGZIP(), []int{4}
}

func (x *TopologyHints) GetZones() []string {
	if x != nil {
		return x.Zones
	}
	return nil
}

type NodeInfo struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Hash uint64 `protobuf:"varint,1,opt,name=Hash,proto3" json:"Hash,omitempty"`
	Node *Node  `protobuf:"bytes,2,opt,name=Node,proto3" json:"Node,omitempty"`
}

func (x *NodeInfo) Reset() {
	*x = NodeInfo{}
	if protoimpl.UnsafeEnabled {
		mi := &file_api_globalv1_api_proto_msgTypes[5]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *NodeInfo) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*NodeInfo) ProtoMessage() {}

func (x *NodeInfo) ProtoReflect() protoreflect.Message {
	mi := &file_api_globalv1_api_proto_msgTypes[5]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use NodeInfo.ProtoReflect.Descriptor instead.
func (*NodeInfo) Descriptor() ([]byte, []int) {
	return file_api_globalv1_api_proto_rawDescGZIP(), []int{5}
}

func (x *NodeInfo) GetHash() uint64 {
	if x != nil {
		return x.Hash
	}
	return 0
}

func (x *NodeInfo) GetNode() *Node {
	if x != nil {
		return x.Node
	}
	return nil
}

type Node struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Name        string            `protobuf:"bytes,1,opt,name=Name,proto3" json:"Name,omitempty"`
	Topology    *TopologyInfo     `protobuf:"bytes,4,opt,name=Topology,proto3" json:"Topology,omitempty"`
	Labels      map[string]string `protobuf:"bytes,2,rep,name=Labels,proto3" json:"Labels,omitempty" protobuf_key:"bytes,1,opt,name=key,proto3" protobuf_val:"bytes,2,opt,name=value,proto3"`
	Annotations map[string]string `protobuf:"bytes,3,rep,name=Annotations,proto3" json:"Annotations,omitempty" protobuf_key:"bytes,1,opt,name=key,proto3" protobuf_val:"bytes,2,opt,name=value,proto3"`
}

func (x *Node) Reset() {
	*x = Node{}
	if protoimpl.UnsafeEnabled {
		mi := &file_api_globalv1_api_proto_msgTypes[6]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Node) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Node) ProtoMessage() {}

func (x *Node) ProtoReflect() protoreflect.Message {
	mi := &file_api_globalv1_api_proto_msgTypes[6]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Node.ProtoReflect.Descriptor instead.
func (*Node) Descriptor() ([]byte, []int) {
	return file_api_globalv1_api_proto_rawDescGZIP(), []int{6}
}

func (x *Node) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *Node) GetTopology() *TopologyInfo {
	if x != nil {
		return x.Topology
	}
	return nil
}

func (x *Node) GetLabels() map[string]string {
	if x != nil {
		return x.Labels
	}
	return nil
}

func (x *Node) GetAnnotations() map[string]string {
	if x != nil {
		return x.Annotations
	}
	return nil
}

type GlobalWatchReq struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *GlobalWatchReq) Reset() {
	*x = GlobalWatchReq{}
	if protoimpl.UnsafeEnabled {
		mi := &file_api_globalv1_api_proto_msgTypes[7]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GlobalWatchReq) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GlobalWatchReq) ProtoMessage() {}

func (x *GlobalWatchReq) ProtoReflect() protoreflect.Message {
	mi := &file_api_globalv1_api_proto_msgTypes[7]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GlobalWatchReq.ProtoReflect.Descriptor instead.
func (*GlobalWatchReq) Descriptor() ([]byte, []int) {
	return file_api_globalv1_api_proto_rawDescGZIP(), []int{7}
}

var File_api_globalv1_api_proto protoreflect.FileDescriptor

var file_api_globalv1_api_proto_rawDesc = []byte{
	0x0a, 0x16, 0x61, 0x70, 0x69, 0x2f, 0x67, 0x6c, 0x6f, 0x62, 0x61, 0x6c, 0x76, 0x31, 0x2f, 0x61,
	0x70, 0x69, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x08, 0x67, 0x6c, 0x6f, 0x62, 0x61, 0x6c,
	0x76, 0x31, 0x1a, 0x15, 0x61, 0x70, 0x69, 0x2f, 0x6c, 0x6f, 0x63, 0x61, 0x6c, 0x76, 0x31, 0x2f,
	0x61, 0x70, 0x69, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0x4d, 0x0a, 0x0b, 0x53, 0x65, 0x72,
	0x76, 0x69, 0x63, 0x65, 0x49, 0x6e, 0x66, 0x6f, 0x12, 0x12, 0x0a, 0x04, 0x48, 0x61, 0x73, 0x68,
	0x18, 0x01, 0x20, 0x01, 0x28, 0x04, 0x52, 0x04, 0x48, 0x61, 0x73, 0x68, 0x12, 0x2a, 0x0a, 0x07,
	0x53, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x10, 0x2e,
	0x6c, 0x6f, 0x63, 0x61, 0x6c, 0x76, 0x31, 0x2e, 0x53, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x52,
	0x07, 0x53, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x22, 0xf2, 0x02, 0x0a, 0x0c, 0x45, 0x6e, 0x64,
	0x70, 0x6f, 0x69, 0x6e, 0x74, 0x49, 0x6e, 0x66, 0x6f, 0x12, 0x12, 0x0a, 0x04, 0x48, 0x61, 0x73,
	0x68, 0x18, 0x01, 0x20, 0x01, 0x28, 0x04, 0x52, 0x04, 0x48, 0x61, 0x73, 0x68, 0x12, 0x1c, 0x0a,
	0x09, 0x4e, 0x61, 0x6d, 0x65, 0x73, 0x70, 0x61, 0x63, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09,
	0x52, 0x09, 0x4e, 0x61, 0x6d, 0x65, 0x73, 0x70, 0x61, 0x63, 0x65, 0x12, 0x1e, 0x0a, 0x0a, 0x53,
	0x6f, 0x75, 0x72, 0x63, 0x65, 0x4e, 0x61, 0x6d, 0x65, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52,
	0x0a, 0x53, 0x6f, 0x75, 0x72, 0x63, 0x65, 0x4e, 0x61, 0x6d, 0x65, 0x12, 0x20, 0x0a, 0x0b, 0x53,
	0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x4e, 0x61, 0x6d, 0x65, 0x18, 0x04, 0x20, 0x01, 0x28, 0x09,
	0x52, 0x0b, 0x53, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x4e, 0x61, 0x6d, 0x65, 0x12, 0x18, 0x0a,
	0x07, 0x50, 0x6f, 0x64, 0x4e, 0x61, 0x6d, 0x65, 0x18, 0x08, 0x20, 0x01, 0x28, 0x09, 0x52, 0x07,
	0x50, 0x6f, 0x64, 0x4e, 0x61, 0x6d, 0x65, 0x12, 0x2d, 0x0a, 0x08, 0x45, 0x6e, 0x64, 0x70, 0x6f,
	0x69, 0x6e, 0x74, 0x18, 0x06, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x11, 0x2e, 0x6c, 0x6f, 0x63, 0x61,
	0x6c, 0x76, 0x31, 0x2e, 0x45, 0x6e, 0x64, 0x70, 0x6f, 0x69, 0x6e, 0x74, 0x52, 0x08, 0x45, 0x6e,
	0x64, 0x70, 0x6f, 0x69, 0x6e, 0x74, 0x12, 0x3c, 0x0a, 0x0a, 0x43, 0x6f, 0x6e, 0x64, 0x69, 0x74,
	0x69, 0x6f, 0x6e, 0x73, 0x18, 0x07, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1c, 0x2e, 0x67, 0x6c, 0x6f,
	0x62, 0x61, 0x6c, 0x76, 0x31, 0x2e, 0x45, 0x6e, 0x64, 0x70, 0x6f, 0x69, 0x6e, 0x74, 0x43, 0x6f,
	0x6e, 0x64, 0x69, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x52, 0x0a, 0x43, 0x6f, 0x6e, 0x64, 0x69, 0x74,
	0x69, 0x6f, 0x6e, 0x73, 0x12, 0x32, 0x0a, 0x08, 0x54, 0x6f, 0x70, 0x6f, 0x6c, 0x6f, 0x67, 0x79,
	0x18, 0x09, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x16, 0x2e, 0x67, 0x6c, 0x6f, 0x62, 0x61, 0x6c, 0x76,
	0x31, 0x2e, 0x54, 0x6f, 0x70, 0x6f, 0x6c, 0x6f, 0x67, 0x79, 0x49, 0x6e, 0x66, 0x6f, 0x52, 0x08,
	0x54, 0x6f, 0x70, 0x6f, 0x6c, 0x6f, 0x67, 0x79, 0x12, 0x2d, 0x0a, 0x05, 0x48, 0x69, 0x6e, 0x74,
	0x73, 0x18, 0x0a, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x17, 0x2e, 0x67, 0x6c, 0x6f, 0x62, 0x61, 0x6c,
	0x76, 0x31, 0x2e, 0x54, 0x6f, 0x70, 0x6f, 0x6c, 0x6f, 0x67, 0x79, 0x48, 0x69, 0x6e, 0x74, 0x73,
	0x52, 0x05, 0x48, 0x69, 0x6e, 0x74, 0x73, 0x4a, 0x04, 0x08, 0x05, 0x10, 0x06, 0x22, 0x2a, 0x0a,
	0x12, 0x45, 0x6e, 0x64, 0x70, 0x6f, 0x69, 0x6e, 0x74, 0x43, 0x6f, 0x6e, 0x64, 0x69, 0x74, 0x69,
	0x6f, 0x6e, 0x73, 0x12, 0x14, 0x0a, 0x05, 0x52, 0x65, 0x61, 0x64, 0x79, 0x18, 0x01, 0x20, 0x01,
	0x28, 0x08, 0x52, 0x05, 0x52, 0x65, 0x61, 0x64, 0x79, 0x22, 0x36, 0x0a, 0x0c, 0x54, 0x6f, 0x70,
	0x6f, 0x6c, 0x6f, 0x67, 0x79, 0x49, 0x6e, 0x66, 0x6f, 0x12, 0x12, 0x0a, 0x04, 0x4e, 0x6f, 0x64,
	0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x4e, 0x6f, 0x64, 0x65, 0x12, 0x12, 0x0a,
	0x04, 0x5a, 0x6f, 0x6e, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x5a, 0x6f, 0x6e,
	0x65, 0x22, 0x25, 0x0a, 0x0d, 0x54, 0x6f, 0x70, 0x6f, 0x6c, 0x6f, 0x67, 0x79, 0x48, 0x69, 0x6e,
	0x74, 0x73, 0x12, 0x14, 0x0a, 0x05, 0x5a, 0x6f, 0x6e, 0x65, 0x73, 0x18, 0x01, 0x20, 0x03, 0x28,
	0x09, 0x52, 0x05, 0x5a, 0x6f, 0x6e, 0x65, 0x73, 0x22, 0x42, 0x0a, 0x08, 0x4e, 0x6f, 0x64, 0x65,
	0x49, 0x6e, 0x66, 0x6f, 0x12, 0x12, 0x0a, 0x04, 0x48, 0x61, 0x73, 0x68, 0x18, 0x01, 0x20, 0x01,
	0x28, 0x04, 0x52, 0x04, 0x48, 0x61, 0x73, 0x68, 0x12, 0x22, 0x0a, 0x04, 0x4e, 0x6f, 0x64, 0x65,
	0x18, 0x02, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x0e, 0x2e, 0x67, 0x6c, 0x6f, 0x62, 0x61, 0x6c, 0x76,
	0x31, 0x2e, 0x4e, 0x6f, 0x64, 0x65, 0x52, 0x04, 0x4e, 0x6f, 0x64, 0x65, 0x22, 0xc0, 0x02, 0x0a,
	0x04, 0x4e, 0x6f, 0x64, 0x65, 0x12, 0x12, 0x0a, 0x04, 0x4e, 0x61, 0x6d, 0x65, 0x18, 0x01, 0x20,
	0x01, 0x28, 0x09, 0x52, 0x04, 0x4e, 0x61, 0x6d, 0x65, 0x12, 0x32, 0x0a, 0x08, 0x54, 0x6f, 0x70,
	0x6f, 0x6c, 0x6f, 0x67, 0x79, 0x18, 0x04, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x16, 0x2e, 0x67, 0x6c,
	0x6f, 0x62, 0x61, 0x6c, 0x76, 0x31, 0x2e, 0x54, 0x6f, 0x70, 0x6f, 0x6c, 0x6f, 0x67, 0x79, 0x49,
	0x6e, 0x66, 0x6f, 0x52, 0x08, 0x54, 0x6f, 0x70, 0x6f, 0x6c, 0x6f, 0x67, 0x79, 0x12, 0x32, 0x0a,
	0x06, 0x4c, 0x61, 0x62, 0x65, 0x6c, 0x73, 0x18, 0x02, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x1a, 0x2e,
	0x67, 0x6c, 0x6f, 0x62, 0x61, 0x6c, 0x76, 0x31, 0x2e, 0x4e, 0x6f, 0x64, 0x65, 0x2e, 0x4c, 0x61,
	0x62, 0x65, 0x6c, 0x73, 0x45, 0x6e, 0x74, 0x72, 0x79, 0x52, 0x06, 0x4c, 0x61, 0x62, 0x65, 0x6c,
	0x73, 0x12, 0x41, 0x0a, 0x0b, 0x41, 0x6e, 0x6e, 0x6f, 0x74, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x73,
	0x18, 0x03, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x1f, 0x2e, 0x67, 0x6c, 0x6f, 0x62, 0x61, 0x6c, 0x76,
	0x31, 0x2e, 0x4e, 0x6f, 0x64, 0x65, 0x2e, 0x41, 0x6e, 0x6e, 0x6f, 0x74, 0x61, 0x74, 0x69, 0x6f,
	0x6e, 0x73, 0x45, 0x6e, 0x74, 0x72, 0x79, 0x52, 0x0b, 0x41, 0x6e, 0x6e, 0x6f, 0x74, 0x61, 0x74,
	0x69, 0x6f, 0x6e, 0x73, 0x1a, 0x39, 0x0a, 0x0b, 0x4c, 0x61, 0x62, 0x65, 0x6c, 0x73, 0x45, 0x6e,
	0x74, 0x72, 0x79, 0x12, 0x10, 0x0a, 0x03, 0x6b, 0x65, 0x79, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09,
	0x52, 0x03, 0x6b, 0x65, 0x79, 0x12, 0x14, 0x0a, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x18, 0x02,
	0x20, 0x01, 0x28, 0x09, 0x52, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x3a, 0x02, 0x38, 0x01, 0x1a,
	0x3e, 0x0a, 0x10, 0x41, 0x6e, 0x6e, 0x6f, 0x74, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x45, 0x6e,
	0x74, 0x72, 0x79, 0x12, 0x10, 0x0a, 0x03, 0x6b, 0x65, 0x79, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09,
	0x52, 0x03, 0x6b, 0x65, 0x79, 0x12, 0x14, 0x0a, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x18, 0x02,
	0x20, 0x01, 0x28, 0x09, 0x52, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x3a, 0x02, 0x38, 0x01, 0x22,
	0x10, 0x0a, 0x0e, 0x47, 0x6c, 0x6f, 0x62, 0x61, 0x6c, 0x57, 0x61, 0x74, 0x63, 0x68, 0x52, 0x65,
	0x71, 0x32, 0x3e, 0x0a, 0x04, 0x53, 0x65, 0x74, 0x73, 0x12, 0x36, 0x0a, 0x05, 0x57, 0x61, 0x74,
	0x63, 0x68, 0x12, 0x18, 0x2e, 0x67, 0x6c, 0x6f, 0x62, 0x61, 0x6c, 0x76, 0x31, 0x2e, 0x47, 0x6c,
	0x6f, 0x62, 0x61, 0x6c, 0x57, 0x61, 0x74, 0x63, 0x68, 0x52, 0x65, 0x71, 0x1a, 0x0f, 0x2e, 0x6c,
	0x6f, 0x63, 0x61, 0x6c, 0x76, 0x31, 0x2e, 0x4f, 0x70, 0x49, 0x74, 0x65, 0x6d, 0x28, 0x01, 0x30,
	0x01, 0x42, 0x1f, 0x5a, 0x1d, 0x73, 0x69, 0x67, 0x73, 0x2e, 0x6b, 0x38, 0x73, 0x2e, 0x69, 0x6f,
	0x2f, 0x6b, 0x70, 0x6e, 0x67, 0x2f, 0x61, 0x70, 0x69, 0x2f, 0x67, 0x6c, 0x6f, 0x62, 0x61, 0x6c,
	0x76, 0x31, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_api_globalv1_api_proto_rawDescOnce sync.Once
	file_api_globalv1_api_proto_rawDescData = file_api_globalv1_api_proto_rawDesc
)

func file_api_globalv1_api_proto_rawDescGZIP() []byte {
	file_api_globalv1_api_proto_rawDescOnce.Do(func() {
		file_api_globalv1_api_proto_rawDescData = protoimpl.X.CompressGZIP(file_api_globalv1_api_proto_rawDescData)
	})
	return file_api_globalv1_api_proto_rawDescData
}

var file_api_globalv1_api_proto_msgTypes = make([]protoimpl.MessageInfo, 10)
var file_api_globalv1_api_proto_goTypes = []interface{}{
	(*ServiceInfo)(nil),        // 0: globalv1.ServiceInfo
	(*EndpointInfo)(nil),       // 1: globalv1.EndpointInfo
	(*EndpointConditions)(nil), // 2: globalv1.EndpointConditions
	(*TopologyInfo)(nil),       // 3: globalv1.TopologyInfo
	(*TopologyHints)(nil),      // 4: globalv1.TopologyHints
	(*NodeInfo)(nil),           // 5: globalv1.NodeInfo
	(*Node)(nil),               // 6: globalv1.Node
	(*GlobalWatchReq)(nil),     // 7: globalv1.GlobalWatchReq
	nil,                        // 8: globalv1.Node.LabelsEntry
	nil,                        // 9: globalv1.Node.AnnotationsEntry
	(*localv1.Service)(nil),    // 10: localv1.Service
	(*localv1.Endpoint)(nil),   // 11: localv1.Endpoint
	(*localv1.OpItem)(nil),     // 12: localv1.OpItem
}
var file_api_globalv1_api_proto_depIdxs = []int32{
	10, // 0: globalv1.ServiceInfo.Service:type_name -> localv1.Service
	11, // 1: globalv1.EndpointInfo.Endpoint:type_name -> localv1.Endpoint
	2,  // 2: globalv1.EndpointInfo.Conditions:type_name -> globalv1.EndpointConditions
	3,  // 3: globalv1.EndpointInfo.Topology:type_name -> globalv1.TopologyInfo
	4,  // 4: globalv1.EndpointInfo.Hints:type_name -> globalv1.TopologyHints
	6,  // 5: globalv1.NodeInfo.Node:type_name -> globalv1.Node
	3,  // 6: globalv1.Node.Topology:type_name -> globalv1.TopologyInfo
	8,  // 7: globalv1.Node.Labels:type_name -> globalv1.Node.LabelsEntry
	9,  // 8: globalv1.Node.Annotations:type_name -> globalv1.Node.AnnotationsEntry
	7,  // 9: globalv1.Sets.Watch:input_type -> globalv1.GlobalWatchReq
	12, // 10: globalv1.Sets.Watch:output_type -> localv1.OpItem
	10, // [10:11] is the sub-list for method output_type
	9,  // [9:10] is the sub-list for method input_type
	9,  // [9:9] is the sub-list for extension type_name
	9,  // [9:9] is the sub-list for extension extendee
	0,  // [0:9] is the sub-list for field type_name
}

func init() { file_api_globalv1_api_proto_init() }
func file_api_globalv1_api_proto_init() {
	if File_api_globalv1_api_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_api_globalv1_api_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ServiceInfo); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_api_globalv1_api_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*EndpointInfo); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_api_globalv1_api_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*EndpointConditions); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_api_globalv1_api_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*TopologyInfo); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_api_globalv1_api_proto_msgTypes[4].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*TopologyHints); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_api_globalv1_api_proto_msgTypes[5].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*NodeInfo); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_api_globalv1_api_proto_msgTypes[6].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Node); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_api_globalv1_api_proto_msgTypes[7].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GlobalWatchReq); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_api_globalv1_api_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   10,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_api_globalv1_api_proto_goTypes,
		DependencyIndexes: file_api_globalv1_api_proto_depIdxs,
		MessageInfos:      file_api_globalv1_api_proto_msgTypes,
	}.Build()
	File_api_globalv1_api_proto = out.File
	file_api_globalv1_api_proto_rawDesc = nil
	file_api_globalv1_api_proto_goTypes = nil
	file_api_globalv1_api_proto_depIdxs = nil
}
