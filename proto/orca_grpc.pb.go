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
	emptypb "google.golang.org/protobuf/types/known/emptypb"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.32.0 or later.
const _ = grpc.SupportPackageIsVersion7

// OrcaClient is the client API for Orca service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type OrcaClient interface {
	Join(ctx context.Context, in *JoinRequest, opts ...grpc.CallOption) (*emptypb.Empty, error)
	Leave(ctx context.Context, in *GuildOnlyRequest, opts ...grpc.CallOption) (*emptypb.Empty, error)
	Play(ctx context.Context, in *PlayRequest, opts ...grpc.CallOption) (*PlayReply, error)
	Skip(ctx context.Context, in *GuildOnlyRequest, opts ...grpc.CallOption) (*emptypb.Empty, error)
	Stop(ctx context.Context, in *GuildOnlyRequest, opts ...grpc.CallOption) (*emptypb.Empty, error)
	Seek(ctx context.Context, in *SeekRequest, opts ...grpc.CallOption) (*SeekReply, error)
	GetTracks(ctx context.Context, in *GetTracksRequest, opts ...grpc.CallOption) (*GetTracksReply, error)
	Pause(ctx context.Context, in *GuildOnlyRequest, opts ...grpc.CallOption) (*emptypb.Empty, error)
	Resume(ctx context.Context, in *GuildOnlyRequest, opts ...grpc.CallOption) (*emptypb.Empty, error)
	Loop(ctx context.Context, in *GuildOnlyRequest, opts ...grpc.CallOption) (*emptypb.Empty, error)
	ShuffleQueue(ctx context.Context, in *GuildOnlyRequest, opts ...grpc.CallOption) (*emptypb.Empty, error)
	Remove(ctx context.Context, in *RemoveRequest, opts ...grpc.CallOption) (*emptypb.Empty, error)
	SavePlaylist(ctx context.Context, in *SavePlaylistRequest, opts ...grpc.CallOption) (*emptypb.Empty, error)
	LoadPlaylist(ctx context.Context, in *LoadPlaylistRequest, opts ...grpc.CallOption) (*PlayReply, error)
	ListPlaylists(ctx context.Context, in *ListPlaylistsRequest, opts ...grpc.CallOption) (*ListPlaylistsReply, error)
}

type orcaClient struct {
	cc grpc.ClientConnInterface
}

func NewOrcaClient(cc grpc.ClientConnInterface) OrcaClient {
	return &orcaClient{cc}
}

func (c *orcaClient) Join(ctx context.Context, in *JoinRequest, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	out := new(emptypb.Empty)
	err := c.cc.Invoke(ctx, "/orca.Orca/Join", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *orcaClient) Leave(ctx context.Context, in *GuildOnlyRequest, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	out := new(emptypb.Empty)
	err := c.cc.Invoke(ctx, "/orca.Orca/Leave", in, out, opts...)
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

func (c *orcaClient) Skip(ctx context.Context, in *GuildOnlyRequest, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	out := new(emptypb.Empty)
	err := c.cc.Invoke(ctx, "/orca.Orca/Skip", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *orcaClient) Stop(ctx context.Context, in *GuildOnlyRequest, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	out := new(emptypb.Empty)
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

func (c *orcaClient) Pause(ctx context.Context, in *GuildOnlyRequest, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	out := new(emptypb.Empty)
	err := c.cc.Invoke(ctx, "/orca.Orca/Pause", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *orcaClient) Resume(ctx context.Context, in *GuildOnlyRequest, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	out := new(emptypb.Empty)
	err := c.cc.Invoke(ctx, "/orca.Orca/Resume", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *orcaClient) Loop(ctx context.Context, in *GuildOnlyRequest, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	out := new(emptypb.Empty)
	err := c.cc.Invoke(ctx, "/orca.Orca/Loop", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *orcaClient) ShuffleQueue(ctx context.Context, in *GuildOnlyRequest, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	out := new(emptypb.Empty)
	err := c.cc.Invoke(ctx, "/orca.Orca/ShuffleQueue", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *orcaClient) Remove(ctx context.Context, in *RemoveRequest, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	out := new(emptypb.Empty)
	err := c.cc.Invoke(ctx, "/orca.Orca/Remove", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *orcaClient) SavePlaylist(ctx context.Context, in *SavePlaylistRequest, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	out := new(emptypb.Empty)
	err := c.cc.Invoke(ctx, "/orca.Orca/SavePlaylist", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *orcaClient) LoadPlaylist(ctx context.Context, in *LoadPlaylistRequest, opts ...grpc.CallOption) (*PlayReply, error) {
	out := new(PlayReply)
	err := c.cc.Invoke(ctx, "/orca.Orca/LoadPlaylist", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *orcaClient) ListPlaylists(ctx context.Context, in *ListPlaylistsRequest, opts ...grpc.CallOption) (*ListPlaylistsReply, error) {
	out := new(ListPlaylistsReply)
	err := c.cc.Invoke(ctx, "/orca.Orca/ListPlaylists", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// OrcaServer is the server API for Orca service.
// All implementations must embed UnimplementedOrcaServer
// for forward compatibility
type OrcaServer interface {
	Join(context.Context, *JoinRequest) (*emptypb.Empty, error)
	Leave(context.Context, *GuildOnlyRequest) (*emptypb.Empty, error)
	Play(context.Context, *PlayRequest) (*PlayReply, error)
	Skip(context.Context, *GuildOnlyRequest) (*emptypb.Empty, error)
	Stop(context.Context, *GuildOnlyRequest) (*emptypb.Empty, error)
	Seek(context.Context, *SeekRequest) (*SeekReply, error)
	GetTracks(context.Context, *GetTracksRequest) (*GetTracksReply, error)
	Pause(context.Context, *GuildOnlyRequest) (*emptypb.Empty, error)
	Resume(context.Context, *GuildOnlyRequest) (*emptypb.Empty, error)
	Loop(context.Context, *GuildOnlyRequest) (*emptypb.Empty, error)
	ShuffleQueue(context.Context, *GuildOnlyRequest) (*emptypb.Empty, error)
	Remove(context.Context, *RemoveRequest) (*emptypb.Empty, error)
	SavePlaylist(context.Context, *SavePlaylistRequest) (*emptypb.Empty, error)
	LoadPlaylist(context.Context, *LoadPlaylistRequest) (*PlayReply, error)
	ListPlaylists(context.Context, *ListPlaylistsRequest) (*ListPlaylistsReply, error)
	mustEmbedUnimplementedOrcaServer()
}

// UnimplementedOrcaServer must be embedded to have forward compatible implementations.
type UnimplementedOrcaServer struct {
}

func (UnimplementedOrcaServer) Join(context.Context, *JoinRequest) (*emptypb.Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Join not implemented")
}
func (UnimplementedOrcaServer) Leave(context.Context, *GuildOnlyRequest) (*emptypb.Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Leave not implemented")
}
func (UnimplementedOrcaServer) Play(context.Context, *PlayRequest) (*PlayReply, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Play not implemented")
}
func (UnimplementedOrcaServer) Skip(context.Context, *GuildOnlyRequest) (*emptypb.Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Skip not implemented")
}
func (UnimplementedOrcaServer) Stop(context.Context, *GuildOnlyRequest) (*emptypb.Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Stop not implemented")
}
func (UnimplementedOrcaServer) Seek(context.Context, *SeekRequest) (*SeekReply, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Seek not implemented")
}
func (UnimplementedOrcaServer) GetTracks(context.Context, *GetTracksRequest) (*GetTracksReply, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetTracks not implemented")
}
func (UnimplementedOrcaServer) Pause(context.Context, *GuildOnlyRequest) (*emptypb.Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Pause not implemented")
}
func (UnimplementedOrcaServer) Resume(context.Context, *GuildOnlyRequest) (*emptypb.Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Resume not implemented")
}
func (UnimplementedOrcaServer) Loop(context.Context, *GuildOnlyRequest) (*emptypb.Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Loop not implemented")
}
func (UnimplementedOrcaServer) ShuffleQueue(context.Context, *GuildOnlyRequest) (*emptypb.Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ShuffleQueue not implemented")
}
func (UnimplementedOrcaServer) Remove(context.Context, *RemoveRequest) (*emptypb.Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Remove not implemented")
}
func (UnimplementedOrcaServer) SavePlaylist(context.Context, *SavePlaylistRequest) (*emptypb.Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method SavePlaylist not implemented")
}
func (UnimplementedOrcaServer) LoadPlaylist(context.Context, *LoadPlaylistRequest) (*PlayReply, error) {
	return nil, status.Errorf(codes.Unimplemented, "method LoadPlaylist not implemented")
}
func (UnimplementedOrcaServer) ListPlaylists(context.Context, *ListPlaylistsRequest) (*ListPlaylistsReply, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ListPlaylists not implemented")
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

func _Orca_Join_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(JoinRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(OrcaServer).Join(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/orca.Orca/Join",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(OrcaServer).Join(ctx, req.(*JoinRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Orca_Leave_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GuildOnlyRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(OrcaServer).Leave(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/orca.Orca/Leave",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(OrcaServer).Leave(ctx, req.(*GuildOnlyRequest))
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
	in := new(GuildOnlyRequest)
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
		return srv.(OrcaServer).Skip(ctx, req.(*GuildOnlyRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Orca_Stop_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GuildOnlyRequest)
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
		return srv.(OrcaServer).Stop(ctx, req.(*GuildOnlyRequest))
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

func _Orca_Pause_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GuildOnlyRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(OrcaServer).Pause(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/orca.Orca/Pause",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(OrcaServer).Pause(ctx, req.(*GuildOnlyRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Orca_Resume_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GuildOnlyRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(OrcaServer).Resume(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/orca.Orca/Resume",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(OrcaServer).Resume(ctx, req.(*GuildOnlyRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Orca_Loop_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GuildOnlyRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(OrcaServer).Loop(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/orca.Orca/Loop",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(OrcaServer).Loop(ctx, req.(*GuildOnlyRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Orca_ShuffleQueue_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GuildOnlyRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(OrcaServer).ShuffleQueue(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/orca.Orca/ShuffleQueue",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(OrcaServer).ShuffleQueue(ctx, req.(*GuildOnlyRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Orca_Remove_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(RemoveRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(OrcaServer).Remove(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/orca.Orca/Remove",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(OrcaServer).Remove(ctx, req.(*RemoveRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Orca_SavePlaylist_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(SavePlaylistRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(OrcaServer).SavePlaylist(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/orca.Orca/SavePlaylist",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(OrcaServer).SavePlaylist(ctx, req.(*SavePlaylistRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Orca_LoadPlaylist_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(LoadPlaylistRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(OrcaServer).LoadPlaylist(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/orca.Orca/LoadPlaylist",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(OrcaServer).LoadPlaylist(ctx, req.(*LoadPlaylistRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Orca_ListPlaylists_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ListPlaylistsRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(OrcaServer).ListPlaylists(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/orca.Orca/ListPlaylists",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(OrcaServer).ListPlaylists(ctx, req.(*ListPlaylistsRequest))
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
			MethodName: "Join",
			Handler:    _Orca_Join_Handler,
		},
		{
			MethodName: "Leave",
			Handler:    _Orca_Leave_Handler,
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
		{
			MethodName: "Pause",
			Handler:    _Orca_Pause_Handler,
		},
		{
			MethodName: "Resume",
			Handler:    _Orca_Resume_Handler,
		},
		{
			MethodName: "Loop",
			Handler:    _Orca_Loop_Handler,
		},
		{
			MethodName: "ShuffleQueue",
			Handler:    _Orca_ShuffleQueue_Handler,
		},
		{
			MethodName: "Remove",
			Handler:    _Orca_Remove_Handler,
		},
		{
			MethodName: "SavePlaylist",
			Handler:    _Orca_SavePlaylist_Handler,
		},
		{
			MethodName: "LoadPlaylist",
			Handler:    _Orca_LoadPlaylist_Handler,
		},
		{
			MethodName: "ListPlaylists",
			Handler:    _Orca_ListPlaylists_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "proto/orca.proto",
}
