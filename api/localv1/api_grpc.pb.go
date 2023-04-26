// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.2.0
// - protoc             v3.21.9
// source: api/localv1/api.proto

package localv1

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.32.0 or later.
const _ = grpc.SupportPackageIsVersion7

// SetsClient is the client API for Sets service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type SetsClient interface {
	// Returns all the endpoints for this node.
	Watch(ctx context.Context, opts ...grpc.CallOption) (Sets_WatchClient, error)
}

type setsClient struct {
	cc grpc.ClientConnInterface
}

func NewSetsClient(cc grpc.ClientConnInterface) SetsClient {
	return &setsClient{cc}
}

func (c *setsClient) Watch(ctx context.Context, opts ...grpc.CallOption) (Sets_WatchClient, error) {
	stream, err := c.cc.NewStream(ctx, &Sets_ServiceDesc.Streams[0], "/localv1.Sets/Watch", opts...)
	if err != nil {
		return nil, err
	}
	x := &setsWatchClient{stream}
	return x, nil
}

type Sets_WatchClient interface {
	Send(*WatchReq) error
	Recv() (*OpItem, error)
	grpc.ClientStream
}

type setsWatchClient struct {
	grpc.ClientStream
}

func (x *setsWatchClient) Send(m *WatchReq) error {
	return x.ClientStream.SendMsg(m)
}

func (x *setsWatchClient) Recv() (*OpItem, error) {
	m := new(OpItem)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// SetsServer is the server API for Sets service.
// All implementations must embed UnimplementedSetsServer
// for forward compatibility
type SetsServer interface {
	// Returns all the endpoints for this node.
	Watch(Sets_WatchServer) error
	mustEmbedUnimplementedSetsServer()
}

// UnimplementedSetsServer must be embedded to have forward compatible implementations.
type UnimplementedSetsServer struct {
}

func (UnimplementedSetsServer) Watch(Sets_WatchServer) error {
	return status.Errorf(codes.Unimplemented, "method Watch not implemented")
}
func (UnimplementedSetsServer) mustEmbedUnimplementedSetsServer() {}

// UnsafeSetsServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to SetsServer will
// result in compilation errors.
type UnsafeSetsServer interface {
	mustEmbedUnimplementedSetsServer()
}

func RegisterSetsServer(s grpc.ServiceRegistrar, srv SetsServer) {
	s.RegisterService(&Sets_ServiceDesc, srv)
}

func _Sets_Watch_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(SetsServer).Watch(&setsWatchServer{stream})
}

type Sets_WatchServer interface {
	Send(*OpItem) error
	Recv() (*WatchReq, error)
	grpc.ServerStream
}

type setsWatchServer struct {
	grpc.ServerStream
}

func (x *setsWatchServer) Send(m *OpItem) error {
	return x.ServerStream.SendMsg(m)
}

func (x *setsWatchServer) Recv() (*WatchReq, error) {
	m := new(WatchReq)
	if err := x.ServerStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// Sets_ServiceDesc is the grpc.ServiceDesc for Sets service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var Sets_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "localv1.Sets",
	HandlerType: (*SetsServer)(nil),
	Methods:     []grpc.MethodDesc{},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "Watch",
			Handler:       _Sets_Watch_Handler,
			ServerStreams: true,
			ClientStreams: true,
		},
	},
	Metadata: "api/localv1/api.proto",
}
