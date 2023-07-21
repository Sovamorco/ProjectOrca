// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.28.1
// 	protoc        v4.23.4
// source: proto/orca.proto

package orca

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	durationpb "google.golang.org/protobuf/types/known/durationpb"
	emptypb "google.golang.org/protobuf/types/known/emptypb"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type JoinRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	GuildID   string `protobuf:"bytes,1,opt,name=guildID,proto3" json:"guildID,omitempty"`
	ChannelID string `protobuf:"bytes,2,opt,name=channelID,proto3" json:"channelID,omitempty"`
}

func (x *JoinRequest) Reset() {
	*x = JoinRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_proto_orca_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *JoinRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*JoinRequest) ProtoMessage() {}

func (x *JoinRequest) ProtoReflect() protoreflect.Message {
	mi := &file_proto_orca_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use JoinRequest.ProtoReflect.Descriptor instead.
func (*JoinRequest) Descriptor() ([]byte, []int) {
	return file_proto_orca_proto_rawDescGZIP(), []int{0}
}

func (x *JoinRequest) GetGuildID() string {
	if x != nil {
		return x.GuildID
	}
	return ""
}

func (x *JoinRequest) GetChannelID() string {
	if x != nil {
		return x.ChannelID
	}
	return ""
}

type PlayRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	GuildID   string `protobuf:"bytes,1,opt,name=guildID,proto3" json:"guildID,omitempty"`
	ChannelID string `protobuf:"bytes,2,opt,name=channelID,proto3" json:"channelID,omitempty"`
	Url       string `protobuf:"bytes,3,opt,name=url,proto3" json:"url,omitempty"`
	Position  int64  `protobuf:"varint,4,opt,name=position,proto3" json:"position,omitempty"`
}

func (x *PlayRequest) Reset() {
	*x = PlayRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_proto_orca_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *PlayRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*PlayRequest) ProtoMessage() {}

func (x *PlayRequest) ProtoReflect() protoreflect.Message {
	mi := &file_proto_orca_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use PlayRequest.ProtoReflect.Descriptor instead.
func (*PlayRequest) Descriptor() ([]byte, []int) {
	return file_proto_orca_proto_rawDescGZIP(), []int{1}
}

func (x *PlayRequest) GetGuildID() string {
	if x != nil {
		return x.GuildID
	}
	return ""
}

func (x *PlayRequest) GetChannelID() string {
	if x != nil {
		return x.ChannelID
	}
	return ""
}

func (x *PlayRequest) GetUrl() string {
	if x != nil {
		return x.Url
	}
	return ""
}

func (x *PlayRequest) GetPosition() int64 {
	if x != nil {
		return x.Position
	}
	return 0
}

type PlayReply struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Tracks []*TrackData `protobuf:"bytes,1,rep,name=tracks,proto3" json:"tracks,omitempty"`
}

func (x *PlayReply) Reset() {
	*x = PlayReply{}
	if protoimpl.UnsafeEnabled {
		mi := &file_proto_orca_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *PlayReply) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*PlayReply) ProtoMessage() {}

func (x *PlayReply) ProtoReflect() protoreflect.Message {
	mi := &file_proto_orca_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use PlayReply.ProtoReflect.Descriptor instead.
func (*PlayReply) Descriptor() ([]byte, []int) {
	return file_proto_orca_proto_rawDescGZIP(), []int{2}
}

func (x *PlayReply) GetTracks() []*TrackData {
	if x != nil {
		return x.Tracks
	}
	return nil
}

type TrackData struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Title       string               `protobuf:"bytes,1,opt,name=title,proto3" json:"title,omitempty"`
	OriginalURL string               `protobuf:"bytes,2,opt,name=originalURL,proto3" json:"originalURL,omitempty"`
	Live        bool                 `protobuf:"varint,3,opt,name=live,proto3" json:"live,omitempty"`
	Position    *durationpb.Duration `protobuf:"bytes,4,opt,name=position,proto3" json:"position,omitempty"`
	Duration    *durationpb.Duration `protobuf:"bytes,5,opt,name=duration,proto3" json:"duration,omitempty"`
}

func (x *TrackData) Reset() {
	*x = TrackData{}
	if protoimpl.UnsafeEnabled {
		mi := &file_proto_orca_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *TrackData) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*TrackData) ProtoMessage() {}

func (x *TrackData) ProtoReflect() protoreflect.Message {
	mi := &file_proto_orca_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use TrackData.ProtoReflect.Descriptor instead.
func (*TrackData) Descriptor() ([]byte, []int) {
	return file_proto_orca_proto_rawDescGZIP(), []int{3}
}

func (x *TrackData) GetTitle() string {
	if x != nil {
		return x.Title
	}
	return ""
}

func (x *TrackData) GetOriginalURL() string {
	if x != nil {
		return x.OriginalURL
	}
	return ""
}

func (x *TrackData) GetLive() bool {
	if x != nil {
		return x.Live
	}
	return false
}

func (x *TrackData) GetPosition() *durationpb.Duration {
	if x != nil {
		return x.Position
	}
	return nil
}

func (x *TrackData) GetDuration() *durationpb.Duration {
	if x != nil {
		return x.Duration
	}
	return nil
}

type GuildOnlyRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	GuildID string `protobuf:"bytes,1,opt,name=guildID,proto3" json:"guildID,omitempty"`
}

func (x *GuildOnlyRequest) Reset() {
	*x = GuildOnlyRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_proto_orca_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GuildOnlyRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GuildOnlyRequest) ProtoMessage() {}

func (x *GuildOnlyRequest) ProtoReflect() protoreflect.Message {
	mi := &file_proto_orca_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GuildOnlyRequest.ProtoReflect.Descriptor instead.
func (*GuildOnlyRequest) Descriptor() ([]byte, []int) {
	return file_proto_orca_proto_rawDescGZIP(), []int{4}
}

func (x *GuildOnlyRequest) GetGuildID() string {
	if x != nil {
		return x.GuildID
	}
	return ""
}

type SeekRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	GuildID  string               `protobuf:"bytes,1,opt,name=guildID,proto3" json:"guildID,omitempty"`
	Position *durationpb.Duration `protobuf:"bytes,2,opt,name=position,proto3" json:"position,omitempty"`
}

func (x *SeekRequest) Reset() {
	*x = SeekRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_proto_orca_proto_msgTypes[5]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *SeekRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SeekRequest) ProtoMessage() {}

func (x *SeekRequest) ProtoReflect() protoreflect.Message {
	mi := &file_proto_orca_proto_msgTypes[5]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SeekRequest.ProtoReflect.Descriptor instead.
func (*SeekRequest) Descriptor() ([]byte, []int) {
	return file_proto_orca_proto_rawDescGZIP(), []int{5}
}

func (x *SeekRequest) GetGuildID() string {
	if x != nil {
		return x.GuildID
	}
	return ""
}

func (x *SeekRequest) GetPosition() *durationpb.Duration {
	if x != nil {
		return x.Position
	}
	return nil
}

type SeekReply struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *SeekReply) Reset() {
	*x = SeekReply{}
	if protoimpl.UnsafeEnabled {
		mi := &file_proto_orca_proto_msgTypes[6]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *SeekReply) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SeekReply) ProtoMessage() {}

func (x *SeekReply) ProtoReflect() protoreflect.Message {
	mi := &file_proto_orca_proto_msgTypes[6]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SeekReply.ProtoReflect.Descriptor instead.
func (*SeekReply) Descriptor() ([]byte, []int) {
	return file_proto_orca_proto_rawDescGZIP(), []int{6}
}

type GetTracksRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	GuildID string `protobuf:"bytes,1,opt,name=guildID,proto3" json:"guildID,omitempty"`
	Start   int64  `protobuf:"varint,2,opt,name=start,proto3" json:"start,omitempty"`
	End     int64  `protobuf:"varint,3,opt,name=end,proto3" json:"end,omitempty"`
}

func (x *GetTracksRequest) Reset() {
	*x = GetTracksRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_proto_orca_proto_msgTypes[7]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GetTracksRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetTracksRequest) ProtoMessage() {}

func (x *GetTracksRequest) ProtoReflect() protoreflect.Message {
	mi := &file_proto_orca_proto_msgTypes[7]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetTracksRequest.ProtoReflect.Descriptor instead.
func (*GetTracksRequest) Descriptor() ([]byte, []int) {
	return file_proto_orca_proto_rawDescGZIP(), []int{7}
}

func (x *GetTracksRequest) GetGuildID() string {
	if x != nil {
		return x.GuildID
	}
	return ""
}

func (x *GetTracksRequest) GetStart() int64 {
	if x != nil {
		return x.Start
	}
	return 0
}

func (x *GetTracksRequest) GetEnd() int64 {
	if x != nil {
		return x.End
	}
	return 0
}

type GetTracksReply struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Tracks      []*TrackData `protobuf:"bytes,1,rep,name=tracks,proto3" json:"tracks,omitempty"`
	TotalTracks int64        `protobuf:"varint,2,opt,name=totalTracks,proto3" json:"totalTracks,omitempty"`
	Looping     bool         `protobuf:"varint,3,opt,name=looping,proto3" json:"looping,omitempty"`
}

func (x *GetTracksReply) Reset() {
	*x = GetTracksReply{}
	if protoimpl.UnsafeEnabled {
		mi := &file_proto_orca_proto_msgTypes[8]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GetTracksReply) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetTracksReply) ProtoMessage() {}

func (x *GetTracksReply) ProtoReflect() protoreflect.Message {
	mi := &file_proto_orca_proto_msgTypes[8]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetTracksReply.ProtoReflect.Descriptor instead.
func (*GetTracksReply) Descriptor() ([]byte, []int) {
	return file_proto_orca_proto_rawDescGZIP(), []int{8}
}

func (x *GetTracksReply) GetTracks() []*TrackData {
	if x != nil {
		return x.Tracks
	}
	return nil
}

func (x *GetTracksReply) GetTotalTracks() int64 {
	if x != nil {
		return x.TotalTracks
	}
	return 0
}

func (x *GetTracksReply) GetLooping() bool {
	if x != nil {
		return x.Looping
	}
	return false
}

var File_proto_orca_proto protoreflect.FileDescriptor

var file_proto_orca_proto_rawDesc = []byte{
	0x0a, 0x10, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x6f, 0x72, 0x63, 0x61, 0x2e, 0x70, 0x72, 0x6f,
	0x74, 0x6f, 0x12, 0x04, 0x6f, 0x72, 0x63, 0x61, 0x1a, 0x1e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65,
	0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2f, 0x64, 0x75, 0x72, 0x61, 0x74, 0x69,
	0x6f, 0x6e, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x1b, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65,
	0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2f, 0x65, 0x6d, 0x70, 0x74, 0x79, 0x2e,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0x45, 0x0a, 0x0b, 0x4a, 0x6f, 0x69, 0x6e, 0x52, 0x65, 0x71,
	0x75, 0x65, 0x73, 0x74, 0x12, 0x18, 0x0a, 0x07, 0x67, 0x75, 0x69, 0x6c, 0x64, 0x49, 0x44, 0x18,
	0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x07, 0x67, 0x75, 0x69, 0x6c, 0x64, 0x49, 0x44, 0x12, 0x1c,
	0x0a, 0x09, 0x63, 0x68, 0x61, 0x6e, 0x6e, 0x65, 0x6c, 0x49, 0x44, 0x18, 0x02, 0x20, 0x01, 0x28,
	0x09, 0x52, 0x09, 0x63, 0x68, 0x61, 0x6e, 0x6e, 0x65, 0x6c, 0x49, 0x44, 0x22, 0x73, 0x0a, 0x0b,
	0x50, 0x6c, 0x61, 0x79, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x18, 0x0a, 0x07, 0x67,
	0x75, 0x69, 0x6c, 0x64, 0x49, 0x44, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x07, 0x67, 0x75,
	0x69, 0x6c, 0x64, 0x49, 0x44, 0x12, 0x1c, 0x0a, 0x09, 0x63, 0x68, 0x61, 0x6e, 0x6e, 0x65, 0x6c,
	0x49, 0x44, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x09, 0x63, 0x68, 0x61, 0x6e, 0x6e, 0x65,
	0x6c, 0x49, 0x44, 0x12, 0x10, 0x0a, 0x03, 0x75, 0x72, 0x6c, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09,
	0x52, 0x03, 0x75, 0x72, 0x6c, 0x12, 0x1a, 0x0a, 0x08, 0x70, 0x6f, 0x73, 0x69, 0x74, 0x69, 0x6f,
	0x6e, 0x18, 0x04, 0x20, 0x01, 0x28, 0x03, 0x52, 0x08, 0x70, 0x6f, 0x73, 0x69, 0x74, 0x69, 0x6f,
	0x6e, 0x22, 0x34, 0x0a, 0x09, 0x50, 0x6c, 0x61, 0x79, 0x52, 0x65, 0x70, 0x6c, 0x79, 0x12, 0x27,
	0x0a, 0x06, 0x74, 0x72, 0x61, 0x63, 0x6b, 0x73, 0x18, 0x01, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x0f,
	0x2e, 0x6f, 0x72, 0x63, 0x61, 0x2e, 0x54, 0x72, 0x61, 0x63, 0x6b, 0x44, 0x61, 0x74, 0x61, 0x52,
	0x06, 0x74, 0x72, 0x61, 0x63, 0x6b, 0x73, 0x22, 0xc5, 0x01, 0x0a, 0x09, 0x54, 0x72, 0x61, 0x63,
	0x6b, 0x44, 0x61, 0x74, 0x61, 0x12, 0x14, 0x0a, 0x05, 0x74, 0x69, 0x74, 0x6c, 0x65, 0x18, 0x01,
	0x20, 0x01, 0x28, 0x09, 0x52, 0x05, 0x74, 0x69, 0x74, 0x6c, 0x65, 0x12, 0x20, 0x0a, 0x0b, 0x6f,
	0x72, 0x69, 0x67, 0x69, 0x6e, 0x61, 0x6c, 0x55, 0x52, 0x4c, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09,
	0x52, 0x0b, 0x6f, 0x72, 0x69, 0x67, 0x69, 0x6e, 0x61, 0x6c, 0x55, 0x52, 0x4c, 0x12, 0x12, 0x0a,
	0x04, 0x6c, 0x69, 0x76, 0x65, 0x18, 0x03, 0x20, 0x01, 0x28, 0x08, 0x52, 0x04, 0x6c, 0x69, 0x76,
	0x65, 0x12, 0x35, 0x0a, 0x08, 0x70, 0x6f, 0x73, 0x69, 0x74, 0x69, 0x6f, 0x6e, 0x18, 0x04, 0x20,
	0x01, 0x28, 0x0b, 0x32, 0x19, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f,
	0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x44, 0x75, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x52, 0x08,
	0x70, 0x6f, 0x73, 0x69, 0x74, 0x69, 0x6f, 0x6e, 0x12, 0x35, 0x0a, 0x08, 0x64, 0x75, 0x72, 0x61,
	0x74, 0x69, 0x6f, 0x6e, 0x18, 0x05, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x19, 0x2e, 0x67, 0x6f, 0x6f,
	0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x44, 0x75, 0x72,
	0x61, 0x74, 0x69, 0x6f, 0x6e, 0x52, 0x08, 0x64, 0x75, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x22,
	0x2c, 0x0a, 0x10, 0x47, 0x75, 0x69, 0x6c, 0x64, 0x4f, 0x6e, 0x6c, 0x79, 0x52, 0x65, 0x71, 0x75,
	0x65, 0x73, 0x74, 0x12, 0x18, 0x0a, 0x07, 0x67, 0x75, 0x69, 0x6c, 0x64, 0x49, 0x44, 0x18, 0x01,
	0x20, 0x01, 0x28, 0x09, 0x52, 0x07, 0x67, 0x75, 0x69, 0x6c, 0x64, 0x49, 0x44, 0x22, 0x5e, 0x0a,
	0x0b, 0x53, 0x65, 0x65, 0x6b, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x18, 0x0a, 0x07,
	0x67, 0x75, 0x69, 0x6c, 0x64, 0x49, 0x44, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x07, 0x67,
	0x75, 0x69, 0x6c, 0x64, 0x49, 0x44, 0x12, 0x35, 0x0a, 0x08, 0x70, 0x6f, 0x73, 0x69, 0x74, 0x69,
	0x6f, 0x6e, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x19, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c,
	0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x44, 0x75, 0x72, 0x61, 0x74,
	0x69, 0x6f, 0x6e, 0x52, 0x08, 0x70, 0x6f, 0x73, 0x69, 0x74, 0x69, 0x6f, 0x6e, 0x22, 0x0b, 0x0a,
	0x09, 0x53, 0x65, 0x65, 0x6b, 0x52, 0x65, 0x70, 0x6c, 0x79, 0x22, 0x54, 0x0a, 0x10, 0x47, 0x65,
	0x74, 0x54, 0x72, 0x61, 0x63, 0x6b, 0x73, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x18,
	0x0a, 0x07, 0x67, 0x75, 0x69, 0x6c, 0x64, 0x49, 0x44, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52,
	0x07, 0x67, 0x75, 0x69, 0x6c, 0x64, 0x49, 0x44, 0x12, 0x14, 0x0a, 0x05, 0x73, 0x74, 0x61, 0x72,
	0x74, 0x18, 0x02, 0x20, 0x01, 0x28, 0x03, 0x52, 0x05, 0x73, 0x74, 0x61, 0x72, 0x74, 0x12, 0x10,
	0x0a, 0x03, 0x65, 0x6e, 0x64, 0x18, 0x03, 0x20, 0x01, 0x28, 0x03, 0x52, 0x03, 0x65, 0x6e, 0x64,
	0x22, 0x75, 0x0a, 0x0e, 0x47, 0x65, 0x74, 0x54, 0x72, 0x61, 0x63, 0x6b, 0x73, 0x52, 0x65, 0x70,
	0x6c, 0x79, 0x12, 0x27, 0x0a, 0x06, 0x74, 0x72, 0x61, 0x63, 0x6b, 0x73, 0x18, 0x01, 0x20, 0x03,
	0x28, 0x0b, 0x32, 0x0f, 0x2e, 0x6f, 0x72, 0x63, 0x61, 0x2e, 0x54, 0x72, 0x61, 0x63, 0x6b, 0x44,
	0x61, 0x74, 0x61, 0x52, 0x06, 0x74, 0x72, 0x61, 0x63, 0x6b, 0x73, 0x12, 0x20, 0x0a, 0x0b, 0x74,
	0x6f, 0x74, 0x61, 0x6c, 0x54, 0x72, 0x61, 0x63, 0x6b, 0x73, 0x18, 0x02, 0x20, 0x01, 0x28, 0x03,
	0x52, 0x0b, 0x74, 0x6f, 0x74, 0x61, 0x6c, 0x54, 0x72, 0x61, 0x63, 0x6b, 0x73, 0x12, 0x18, 0x0a,
	0x07, 0x6c, 0x6f, 0x6f, 0x70, 0x69, 0x6e, 0x67, 0x18, 0x03, 0x20, 0x01, 0x28, 0x08, 0x52, 0x07,
	0x6c, 0x6f, 0x6f, 0x70, 0x69, 0x6e, 0x67, 0x32, 0xf6, 0x04, 0x0a, 0x04, 0x4f, 0x72, 0x63, 0x61,
	0x12, 0x33, 0x0a, 0x04, 0x4a, 0x6f, 0x69, 0x6e, 0x12, 0x11, 0x2e, 0x6f, 0x72, 0x63, 0x61, 0x2e,
	0x4a, 0x6f, 0x69, 0x6e, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x16, 0x2e, 0x67, 0x6f,
	0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x45, 0x6d,
	0x70, 0x74, 0x79, 0x22, 0x00, 0x12, 0x39, 0x0a, 0x05, 0x4c, 0x65, 0x61, 0x76, 0x65, 0x12, 0x16,
	0x2e, 0x6f, 0x72, 0x63, 0x61, 0x2e, 0x47, 0x75, 0x69, 0x6c, 0x64, 0x4f, 0x6e, 0x6c, 0x79, 0x52,
	0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x16, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x45, 0x6d, 0x70, 0x74, 0x79, 0x22, 0x00,
	0x12, 0x2c, 0x0a, 0x04, 0x50, 0x6c, 0x61, 0x79, 0x12, 0x11, 0x2e, 0x6f, 0x72, 0x63, 0x61, 0x2e,
	0x50, 0x6c, 0x61, 0x79, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x0f, 0x2e, 0x6f, 0x72,
	0x63, 0x61, 0x2e, 0x50, 0x6c, 0x61, 0x79, 0x52, 0x65, 0x70, 0x6c, 0x79, 0x22, 0x00, 0x12, 0x38,
	0x0a, 0x04, 0x53, 0x6b, 0x69, 0x70, 0x12, 0x16, 0x2e, 0x6f, 0x72, 0x63, 0x61, 0x2e, 0x47, 0x75,
	0x69, 0x6c, 0x64, 0x4f, 0x6e, 0x6c, 0x79, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x16,
	0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66,
	0x2e, 0x45, 0x6d, 0x70, 0x74, 0x79, 0x22, 0x00, 0x12, 0x38, 0x0a, 0x04, 0x53, 0x74, 0x6f, 0x70,
	0x12, 0x16, 0x2e, 0x6f, 0x72, 0x63, 0x61, 0x2e, 0x47, 0x75, 0x69, 0x6c, 0x64, 0x4f, 0x6e, 0x6c,
	0x79, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x16, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c,
	0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x45, 0x6d, 0x70, 0x74, 0x79,
	0x22, 0x00, 0x12, 0x2c, 0x0a, 0x04, 0x53, 0x65, 0x65, 0x6b, 0x12, 0x11, 0x2e, 0x6f, 0x72, 0x63,
	0x61, 0x2e, 0x53, 0x65, 0x65, 0x6b, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x0f, 0x2e,
	0x6f, 0x72, 0x63, 0x61, 0x2e, 0x53, 0x65, 0x65, 0x6b, 0x52, 0x65, 0x70, 0x6c, 0x79, 0x22, 0x00,
	0x12, 0x3b, 0x0a, 0x09, 0x47, 0x65, 0x74, 0x54, 0x72, 0x61, 0x63, 0x6b, 0x73, 0x12, 0x16, 0x2e,
	0x6f, 0x72, 0x63, 0x61, 0x2e, 0x47, 0x65, 0x74, 0x54, 0x72, 0x61, 0x63, 0x6b, 0x73, 0x52, 0x65,
	0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x14, 0x2e, 0x6f, 0x72, 0x63, 0x61, 0x2e, 0x47, 0x65, 0x74,
	0x54, 0x72, 0x61, 0x63, 0x6b, 0x73, 0x52, 0x65, 0x70, 0x6c, 0x79, 0x22, 0x00, 0x12, 0x39, 0x0a,
	0x05, 0x50, 0x61, 0x75, 0x73, 0x65, 0x12, 0x16, 0x2e, 0x6f, 0x72, 0x63, 0x61, 0x2e, 0x47, 0x75,
	0x69, 0x6c, 0x64, 0x4f, 0x6e, 0x6c, 0x79, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x16,
	0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66,
	0x2e, 0x45, 0x6d, 0x70, 0x74, 0x79, 0x22, 0x00, 0x12, 0x3a, 0x0a, 0x06, 0x52, 0x65, 0x73, 0x75,
	0x6d, 0x65, 0x12, 0x16, 0x2e, 0x6f, 0x72, 0x63, 0x61, 0x2e, 0x47, 0x75, 0x69, 0x6c, 0x64, 0x4f,
	0x6e, 0x6c, 0x79, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x16, 0x2e, 0x67, 0x6f, 0x6f,
	0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x45, 0x6d, 0x70,
	0x74, 0x79, 0x22, 0x00, 0x12, 0x38, 0x0a, 0x04, 0x4c, 0x6f, 0x6f, 0x70, 0x12, 0x16, 0x2e, 0x6f,
	0x72, 0x63, 0x61, 0x2e, 0x47, 0x75, 0x69, 0x6c, 0x64, 0x4f, 0x6e, 0x6c, 0x79, 0x52, 0x65, 0x71,
	0x75, 0x65, 0x73, 0x74, 0x1a, 0x16, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x45, 0x6d, 0x70, 0x74, 0x79, 0x22, 0x00, 0x12, 0x40,
	0x0a, 0x0c, 0x53, 0x68, 0x75, 0x66, 0x66, 0x6c, 0x65, 0x51, 0x75, 0x65, 0x75, 0x65, 0x12, 0x16,
	0x2e, 0x6f, 0x72, 0x63, 0x61, 0x2e, 0x47, 0x75, 0x69, 0x6c, 0x64, 0x4f, 0x6e, 0x6c, 0x79, 0x52,
	0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x16, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x45, 0x6d, 0x70, 0x74, 0x79, 0x22, 0x00,
	0x42, 0x18, 0x5a, 0x16, 0x50, 0x72, 0x6f, 0x6a, 0x65, 0x63, 0x74, 0x4f, 0x72, 0x63, 0x61, 0x2f,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x6f, 0x72, 0x63, 0x61, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x33,
}

var (
	file_proto_orca_proto_rawDescOnce sync.Once
	file_proto_orca_proto_rawDescData = file_proto_orca_proto_rawDesc
)

func file_proto_orca_proto_rawDescGZIP() []byte {
	file_proto_orca_proto_rawDescOnce.Do(func() {
		file_proto_orca_proto_rawDescData = protoimpl.X.CompressGZIP(file_proto_orca_proto_rawDescData)
	})
	return file_proto_orca_proto_rawDescData
}

var file_proto_orca_proto_msgTypes = make([]protoimpl.MessageInfo, 9)
var file_proto_orca_proto_goTypes = []interface{}{
	(*JoinRequest)(nil),         // 0: orca.JoinRequest
	(*PlayRequest)(nil),         // 1: orca.PlayRequest
	(*PlayReply)(nil),           // 2: orca.PlayReply
	(*TrackData)(nil),           // 3: orca.TrackData
	(*GuildOnlyRequest)(nil),    // 4: orca.GuildOnlyRequest
	(*SeekRequest)(nil),         // 5: orca.SeekRequest
	(*SeekReply)(nil),           // 6: orca.SeekReply
	(*GetTracksRequest)(nil),    // 7: orca.GetTracksRequest
	(*GetTracksReply)(nil),      // 8: orca.GetTracksReply
	(*durationpb.Duration)(nil), // 9: google.protobuf.Duration
	(*emptypb.Empty)(nil),       // 10: google.protobuf.Empty
}
var file_proto_orca_proto_depIdxs = []int32{
	3,  // 0: orca.PlayReply.tracks:type_name -> orca.TrackData
	9,  // 1: orca.TrackData.position:type_name -> google.protobuf.Duration
	9,  // 2: orca.TrackData.duration:type_name -> google.protobuf.Duration
	9,  // 3: orca.SeekRequest.position:type_name -> google.protobuf.Duration
	3,  // 4: orca.GetTracksReply.tracks:type_name -> orca.TrackData
	0,  // 5: orca.Orca.Join:input_type -> orca.JoinRequest
	4,  // 6: orca.Orca.Leave:input_type -> orca.GuildOnlyRequest
	1,  // 7: orca.Orca.Play:input_type -> orca.PlayRequest
	4,  // 8: orca.Orca.Skip:input_type -> orca.GuildOnlyRequest
	4,  // 9: orca.Orca.Stop:input_type -> orca.GuildOnlyRequest
	5,  // 10: orca.Orca.Seek:input_type -> orca.SeekRequest
	7,  // 11: orca.Orca.GetTracks:input_type -> orca.GetTracksRequest
	4,  // 12: orca.Orca.Pause:input_type -> orca.GuildOnlyRequest
	4,  // 13: orca.Orca.Resume:input_type -> orca.GuildOnlyRequest
	4,  // 14: orca.Orca.Loop:input_type -> orca.GuildOnlyRequest
	4,  // 15: orca.Orca.ShuffleQueue:input_type -> orca.GuildOnlyRequest
	10, // 16: orca.Orca.Join:output_type -> google.protobuf.Empty
	10, // 17: orca.Orca.Leave:output_type -> google.protobuf.Empty
	2,  // 18: orca.Orca.Play:output_type -> orca.PlayReply
	10, // 19: orca.Orca.Skip:output_type -> google.protobuf.Empty
	10, // 20: orca.Orca.Stop:output_type -> google.protobuf.Empty
	6,  // 21: orca.Orca.Seek:output_type -> orca.SeekReply
	8,  // 22: orca.Orca.GetTracks:output_type -> orca.GetTracksReply
	10, // 23: orca.Orca.Pause:output_type -> google.protobuf.Empty
	10, // 24: orca.Orca.Resume:output_type -> google.protobuf.Empty
	10, // 25: orca.Orca.Loop:output_type -> google.protobuf.Empty
	10, // 26: orca.Orca.ShuffleQueue:output_type -> google.protobuf.Empty
	16, // [16:27] is the sub-list for method output_type
	5,  // [5:16] is the sub-list for method input_type
	5,  // [5:5] is the sub-list for extension type_name
	5,  // [5:5] is the sub-list for extension extendee
	0,  // [0:5] is the sub-list for field type_name
}

func init() { file_proto_orca_proto_init() }
func file_proto_orca_proto_init() {
	if File_proto_orca_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_proto_orca_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*JoinRequest); i {
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
		file_proto_orca_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*PlayRequest); i {
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
		file_proto_orca_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*PlayReply); i {
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
		file_proto_orca_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*TrackData); i {
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
		file_proto_orca_proto_msgTypes[4].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GuildOnlyRequest); i {
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
		file_proto_orca_proto_msgTypes[5].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*SeekRequest); i {
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
		file_proto_orca_proto_msgTypes[6].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*SeekReply); i {
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
		file_proto_orca_proto_msgTypes[7].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GetTracksRequest); i {
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
		file_proto_orca_proto_msgTypes[8].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GetTracksReply); i {
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
			RawDescriptor: file_proto_orca_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   9,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_proto_orca_proto_goTypes,
		DependencyIndexes: file_proto_orca_proto_depIdxs,
		MessageInfos:      file_proto_orca_proto_msgTypes,
	}.Build()
	File_proto_orca_proto = out.File
	file_proto_orca_proto_rawDesc = nil
	file_proto_orca_proto_goTypes = nil
	file_proto_orca_proto_depIdxs = nil
}
