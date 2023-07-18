// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.2.0
// - protoc             v4.23.4
// source: proto/orca.proto

package orca

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

// OrcaClient is the client API for Orca service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type OrcaClient interface {
	Register(ctx context.Context, in *RegisterRequest, opts ...grpc.CallOption) (*RegisterReply, error)
	Play(ctx context.Context, in *PlayRequest, opts ...grpc.CallOption) (*PlayReply, error)
	Skip(ctx context.Context, in *SkipRequest, opts ...grpc.CallOption) (*SkipReply, error)
	Stop(ctx context.Context, in *StopRequest, opts ...grpc.CallOption) (*StopReply, error)
	Seek(ctx context.Context, in *SeekRequest, opts ...grpc.CallOption) (*SeekReply, error)
	GetTracks(ctx context.Context, in *GetTracksRequest, opts ...grpc.CallOption) (*GetTracksReply, error)
}

type orcaClient struct {
	cc grpc.ClientConnInterface
}

func NewOrcaClient(cc grpc.ClientConnInterface) OrcaClient {
	return &orcaClient{cc}
}

func (c *orcaClient) Register(ctx context.Context, in *RegisterRequest, opts ...grpc.CallOption) (*RegisterReply, error) {
	out := new(RegisterReply)
	err := c.cc.Invoke(ctx, "/orca.Orca/Register", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *orcaClient) Play(ctx context.Context, in *PlayRequest, opts ...grpc.CallOption) (*PlayReply, error) {
	out := new(PlayReply)
	err := c.cc.Invoke(ctx, "/orca.Orca/Play", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *orcaClient) Skip(ctx context.Context, in *SkipRequest, opts ...grpc.CallOption) (*SkipReply, error) {
	out := new(SkipReply)
	err := c.cc.Invoke(ctx, "/orca.Orca/Skip", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *orcaClient) Stop(ctx context.Context, in *StopRequest, opts ...grpc.CallOption) (*StopReply, error) {
	out := new(StopReply)
	err := c.cc.Invoke(ctx, "/orca.Orca/Stop", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *orcaClient) Seek(ctx context.Context, in *SeekRequest, opts ...grpc.CallOption) (*SeekReply, error) {
	out := new(SeekReply)
	err := c.cc.Invoke(ctx, "/orca.Orca/Seek", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *orcaClient) GetTracks(ctx context.Context, in *GetTracksRequest, opts ...grpc.CallOption) (*GetTracksReply, error) {
	out := new(GetTracksReply)
	err := c.cc.Invoke(ctx, "/orca.Orca/GetTracks", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// OrcaServer is the server API for Orca service.
// All implementations must embed UnimplementedOrcaServer
// for forward compatibility
type OrcaServer interface {
	Register(context.Context, *RegisterRequest) (*RegisterReply, error)
	Play(context.Context, *PlayRequest) (*PlayReply, error)
	Skip(context.Context, *SkipRequest) (*SkipReply, error)
	Stop(context.Context, *StopRequest) (*StopReply, error)
	Seek(context.Context, *SeekRequest) (*SeekReply, error)
	GetTracks(context.Context, *GetTracksRequest) (*GetTracksReply, error)
	mustEmbedUnimplementedOrcaServer()
}

// UnimplementedOrcaServer must be embedded to have forward compatible implementations.
type UnimplementedOrcaServer struct {
}

func (UnimplementedOrcaServer) Register(context.Context, *RegisterRequest) (*RegisterReply, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Register not implemented")
}
func (UnimplementedOrcaServer) Play(context.Context, *PlayRequest) (*PlayReply, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Play not implemented")
}
func (UnimplementedOrcaServer) Skip(context.Context, *SkipRequest) (*SkipReply, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Skip not implemented")
}
func (UnimplementedOrcaServer) Stop(context.Context, *StopRequest) (*StopReply, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Stop not implemented")
}
func (UnimplementedOrcaServer) Seek(context.Context, *SeekRequest) (*SeekReply, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Seek not implemented")
}
func (UnimplementedOrcaServer) GetTracks(context.Context, *GetTracksRequest) (*GetTracksReply, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetTracks not implemented")
}
func (UnimplementedOrcaServer) mustEmbedUnimplementedOrcaServer() {}

// UnsafeOrcaServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to OrcaServer will
// result in compilation errors.
type UnsafeOrcaServer interface {
	mustEmbedUnimplementedOrcaServer()
}

func RegisterOrcaServer(s grpc.ServiceRegistrar, srv OrcaServer) {
	s.RegisterService(&Orca_ServiceDesc, srv)
}

func _Orca_Register_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(RegisterRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(OrcaServer).Register(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/orca.Orca/Register",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(OrcaServer).Register(ctx, req.(*RegisterRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Orca_Play_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(PlayRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(OrcaServer).Play(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/orca.Orca/Play",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(OrcaServer).Play(ctx, req.(*PlayRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Orca_Skip_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(SkipRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(OrcaServer).Skip(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/orca.Orca/Skip",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(OrcaServer).Skip(ctx, req.(*SkipRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Orca_Stop_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(StopRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(OrcaServer).Stop(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/orca.Orca/Stop",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(OrcaServer).Stop(ctx, req.(*StopRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Orca_Seek_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(SeekRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(OrcaServer).Seek(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/orca.Orca/Seek",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(OrcaServer).Seek(ctx, req.(*SeekRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Orca_GetTracks_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetTracksRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(OrcaServer).GetTracks(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/orca.Orca/GetTracks",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(OrcaServer).GetTracks(ctx, req.(*GetTracksRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// Orca_ServiceDesc is the grpc.ServiceDesc for Orca service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var Orca_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "orca.Orca",
	HandlerType: (*OrcaServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Register",
			Handler:    _Orca_Register_Handler,
		},
		{
			MethodName: "Play",
			Handler:    _Orca_Play_Handler,
		},
		{
			MethodName: "Skip",
			Handler:    _Orca_Skip_Handler,
		},
		{
			MethodName: "Stop",
			Handler:    _Orca_Stop_Handler,
		},
		{
			MethodName: "Seek",
			Handler:    _Orca_Seek_Handler,
		},
		{
			MethodName: "GetTracks",
			Handler:    _Orca_GetTracks_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "proto/orca.proto",
}
