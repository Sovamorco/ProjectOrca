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
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type RegisterRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Token string `protobuf:"bytes,1,opt,name=token,proto3" json:"token,omitempty"`
}

func (x *RegisterRequest) Reset() {
	*x = RegisterRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_proto_orca_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *RegisterRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*RegisterRequest) ProtoMessage() {}

func (x *RegisterRequest) ProtoReflect() protoreflect.Message {
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

// Deprecated: Use RegisterRequest.ProtoReflect.Descriptor instead.
func (*RegisterRequest) Descriptor() ([]byte, []int) {
	return file_proto_orca_proto_rawDescGZIP(), []int{0}
}

func (x *RegisterRequest) GetToken() string {
	if x != nil {
		return x.Token
	}
	return ""
}

type RegisterReply struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Token string `protobuf:"bytes,1,opt,name=token,proto3" json:"token,omitempty"`
}

func (x *RegisterReply) Reset() {
	*x = RegisterReply{}
	if protoimpl.UnsafeEnabled {
		mi := &file_proto_orca_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *RegisterReply) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*RegisterReply) ProtoMessage() {}

func (x *RegisterReply) ProtoReflect() protoreflect.Message {
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

// Deprecated: Use RegisterReply.ProtoReflect.Descriptor instead.
func (*RegisterReply) Descriptor() ([]byte, []int) {
	return file_proto_orca_proto_rawDescGZIP(), []int{1}
}

func (x *RegisterReply) GetToken() string {
	if x != nil {
		return x.Token
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
		mi := &file_proto_orca_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *PlayRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*PlayRequest) ProtoMessage() {}

func (x *PlayRequest) ProtoReflect() protoreflect.Message {
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

// Deprecated: Use PlayRequest.ProtoReflect.Descriptor instead.
func (*PlayRequest) Descriptor() ([]byte, []int) {
	return file_proto_orca_proto_rawDescGZIP(), []int{2}
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

	Track *TrackData `protobuf:"bytes,1,opt,name=track,proto3" json:"track,omitempty"`
}

func (x *PlayReply) Reset() {
	*x = PlayReply{}
	if protoimpl.UnsafeEnabled {
		mi := &file_proto_orca_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *PlayReply) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*PlayReply) ProtoMessage() {}

func (x *PlayReply) ProtoReflect() protoreflect.Message {
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

// Deprecated: Use PlayReply.ProtoReflect.Descriptor instead.
func (*PlayReply) Descriptor() ([]byte, []int) {
	return file_proto_orca_proto_rawDescGZIP(), []int{3}
}

func (x *PlayReply) GetTrack() *TrackData {
	if x != nil {
		return x.Track
	}
	return nil
}

type TrackData struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Title       string               `protobuf:"bytes,1,opt,name=title,proto3" json:"title,omitempty"`
	OriginalURL string               `protobuf:"bytes,2,opt,name=originalURL,proto3" json:"originalURL,omitempty"`
	Url         string               `protobuf:"bytes,3,opt,name=url,proto3" json:"url,omitempty"`
	Live        bool                 `protobuf:"varint,4,opt,name=live,proto3" json:"live,omitempty"`
	Position    *durationpb.Duration `protobuf:"bytes,5,opt,name=position,proto3" json:"position,omitempty"`
	Duration    *durationpb.Duration `protobuf:"bytes,6,opt,name=duration,proto3" json:"duration,omitempty"`
}

func (x *TrackData) Reset() {
	*x = TrackData{}
	if protoimpl.UnsafeEnabled {
		mi := &file_proto_orca_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *TrackData) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*TrackData) ProtoMessage() {}

func (x *TrackData) ProtoReflect() protoreflect.Message {
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

// Deprecated: Use TrackData.ProtoReflect.Descriptor instead.
func (*TrackData) Descriptor() ([]byte, []int) {
	return file_proto_orca_proto_rawDescGZIP(), []int{4}
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

func (x *TrackData) GetUrl() string {
	if x != nil {
		return x.Url
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
		mi := &file_proto_orca_proto_msgTypes[5]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GuildOnlyRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GuildOnlyRequest) ProtoMessage() {}

func (x *GuildOnlyRequest) ProtoReflect() protoreflect.Message {
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

// Deprecated: Use GuildOnlyRequest.ProtoReflect.Descriptor instead.
func (*GuildOnlyRequest) Descriptor() ([]byte, []int) {
	return file_proto_orca_proto_rawDescGZIP(), []int{5}
}

func (x *GuildOnlyRequest) GetGuildID() string {
	if x != nil {
		return x.GuildID
	}
	return ""
}

type GuildOnlyReply struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *GuildOnlyReply) Reset() {
	*x = GuildOnlyReply{}
	if protoimpl.UnsafeEnabled {
		mi := &file_proto_orca_proto_msgTypes[6]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GuildOnlyReply) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GuildOnlyReply) ProtoMessage() {}

func (x *GuildOnlyReply) ProtoReflect() protoreflect.Message {
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

// Deprecated: Use GuildOnlyReply.ProtoReflect.Descriptor instead.
func (*GuildOnlyReply) Descriptor() ([]byte, []int) {
	return file_proto_orca_proto_rawDescGZIP(), []int{6}
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
		mi := &file_proto_orca_proto_msgTypes[7]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *SeekRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SeekRequest) ProtoMessage() {}

func (x *SeekRequest) ProtoReflect() protoreflect.Message {
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

// Deprecated: Use SeekRequest.ProtoReflect.Descriptor instead.
func (*SeekRequest) Descriptor() ([]byte, []int) {
	return file_proto_orca_proto_rawDescGZIP(), []int{7}
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
		mi := &file_proto_orca_proto_msgTypes[8]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *SeekReply) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SeekReply) ProtoMessage() {}

func (x *SeekReply) ProtoReflect() protoreflect.Message {
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

// Deprecated: Use SeekReply.ProtoReflect.Descriptor instead.
func (*SeekReply) Descriptor() ([]byte, []int) {
	return file_proto_orca_proto_rawDescGZIP(), []int{8}
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
		mi := &file_proto_orca_proto_msgTypes[9]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GetTracksRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetTracksRequest) ProtoMessage() {}

func (x *GetTracksRequest) ProtoReflect() protoreflect.Message {
	mi := &file_proto_orca_proto_msgTypes[9]
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
	return file_proto_orca_proto_rawDescGZIP(), []int{9}
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
		mi := &file_proto_orca_proto_msgTypes[10]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GetTracksReply) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetTracksReply) ProtoMessage() {}

func (x *GetTracksReply) ProtoReflect() protoreflect.Message {
	mi := &file_proto_orca_proto_msgTypes[10]
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
	return file_proto_orca_proto_rawDescGZIP(), []int{10}
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
	0x6f, 0x6e, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0x27, 0x0a, 0x0f, 0x52, 0x65, 0x67, 0x69,
	0x73, 0x74, 0x65, 0x72, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x14, 0x0a, 0x05, 0x74,
	0x6f, 0x6b, 0x65, 0x6e, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x05, 0x74, 0x6f, 0x6b, 0x65,
	0x6e, 0x22, 0x25, 0x0a, 0x0d, 0x52, 0x65, 0x67, 0x69, 0x73, 0x74, 0x65, 0x72, 0x52, 0x65, 0x70,
	0x6c, 0x79, 0x12, 0x14, 0x0a, 0x05, 0x74, 0x6f, 0x6b, 0x65, 0x6e, 0x18, 0x01, 0x20, 0x01, 0x28,
	0x09, 0x52, 0x05, 0x74, 0x6f, 0x6b, 0x65, 0x6e, 0x22, 0x73, 0x0a, 0x0b, 0x50, 0x6c, 0x61, 0x79,
	0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x18, 0x0a, 0x07, 0x67, 0x75, 0x69, 0x6c, 0x64,
	0x49, 0x44, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x07, 0x67, 0x75, 0x69, 0x6c, 0x64, 0x49,
	0x44, 0x12, 0x1c, 0x0a, 0x09, 0x63, 0x68, 0x61, 0x6e, 0x6e, 0x65, 0x6c, 0x49, 0x44, 0x18, 0x02,
	0x20, 0x01, 0x28, 0x09, 0x52, 0x09, 0x63, 0x68, 0x61, 0x6e, 0x6e, 0x65, 0x6c, 0x49, 0x44, 0x12,
	0x10, 0x0a, 0x03, 0x75, 0x72, 0x6c, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x03, 0x75, 0x72,
	0x6c, 0x12, 0x1a, 0x0a, 0x08, 0x70, 0x6f, 0x73, 0x69, 0x74, 0x69, 0x6f, 0x6e, 0x18, 0x04, 0x20,
	0x01, 0x28, 0x03, 0x52, 0x08, 0x70, 0x6f, 0x73, 0x69, 0x74, 0x69, 0x6f, 0x6e, 0x22, 0x32, 0x0a,
	0x09, 0x50, 0x6c, 0x61, 0x79, 0x52, 0x65, 0x70, 0x6c, 0x79, 0x12, 0x25, 0x0a, 0x05, 0x74, 0x72,
	0x61, 0x63, 0x6b, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x0f, 0x2e, 0x6f, 0x72, 0x63, 0x61,
	0x2e, 0x54, 0x72, 0x61, 0x63, 0x6b, 0x44, 0x61, 0x74, 0x61, 0x52, 0x05, 0x74, 0x72, 0x61, 0x63,
	0x6b, 0x22, 0xd7, 0x01, 0x0a, 0x09, 0x54, 0x72, 0x61, 0x63, 0x6b, 0x44, 0x61, 0x74, 0x61, 0x12,
	0x14, 0x0a, 0x05, 0x74, 0x69, 0x74, 0x6c, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x05,
	0x74, 0x69, 0x74, 0x6c, 0x65, 0x12, 0x20, 0x0a, 0x0b, 0x6f, 0x72, 0x69, 0x67, 0x69, 0x6e, 0x61,
	0x6c, 0x55, 0x52, 0x4c, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0b, 0x6f, 0x72, 0x69, 0x67,
	0x69, 0x6e, 0x61, 0x6c, 0x55, 0x52, 0x4c, 0x12, 0x10, 0x0a, 0x03, 0x75, 0x72, 0x6c, 0x18, 0x03,
	0x20, 0x01, 0x28, 0x09, 0x52, 0x03, 0x75, 0x72, 0x6c, 0x12, 0x12, 0x0a, 0x04, 0x6c, 0x69, 0x76,
	0x65, 0x18, 0x04, 0x20, 0x01, 0x28, 0x08, 0x52, 0x04, 0x6c, 0x69, 0x76, 0x65, 0x12, 0x35, 0x0a,
	0x08, 0x70, 0x6f, 0x73, 0x69, 0x74, 0x69, 0x6f, 0x6e, 0x18, 0x05, 0x20, 0x01, 0x28, 0x0b, 0x32,
	0x19, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75,
	0x66, 0x2e, 0x44, 0x75, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x52, 0x08, 0x70, 0x6f, 0x73, 0x69,
	0x74, 0x69, 0x6f, 0x6e, 0x12, 0x35, 0x0a, 0x08, 0x64, 0x75, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e,
	0x18, 0x06, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x19, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x44, 0x75, 0x72, 0x61, 0x74, 0x69, 0x6f,
	0x6e, 0x52, 0x08, 0x64, 0x75, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x22, 0x2c, 0x0a, 0x10, 0x47,
	0x75, 0x69, 0x6c, 0x64, 0x4f, 0x6e, 0x6c, 0x79, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12,
	0x18, 0x0a, 0x07, 0x67, 0x75, 0x69, 0x6c, 0x64, 0x49, 0x44, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09,
	0x52, 0x07, 0x67, 0x75, 0x69, 0x6c, 0x64, 0x49, 0x44, 0x22, 0x10, 0x0a, 0x0e, 0x47, 0x75, 0x69,
	0x6c, 0x64, 0x4f, 0x6e, 0x6c, 0x79, 0x52, 0x65, 0x70, 0x6c, 0x79, 0x22, 0x5e, 0x0a, 0x0b, 0x53,
	0x65, 0x65, 0x6b, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x18, 0x0a, 0x07, 0x67, 0x75,
	0x69, 0x6c, 0x64, 0x49, 0x44, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x07, 0x67, 0x75, 0x69,
	0x6c, 0x64, 0x49, 0x44, 0x12, 0x35, 0x0a, 0x08, 0x70, 0x6f, 0x73, 0x69, 0x74, 0x69, 0x6f, 0x6e,
	0x18, 0x02, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x19, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x44, 0x75, 0x72, 0x61, 0x74, 0x69, 0x6f,
	0x6e, 0x52, 0x08, 0x70, 0x6f, 0x73, 0x69, 0x74, 0x69, 0x6f, 0x6e, 0x22, 0x0b, 0x0a, 0x09, 0x53,
	0x65, 0x65, 0x6b, 0x52, 0x65, 0x70, 0x6c, 0x79, 0x22, 0x54, 0x0a, 0x10, 0x47, 0x65, 0x74, 0x54,
	0x72, 0x61, 0x63, 0x6b, 0x73, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x18, 0x0a, 0x07,
	0x67, 0x75, 0x69, 0x6c, 0x64, 0x49, 0x44, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x07, 0x67,
	0x75, 0x69, 0x6c, 0x64, 0x49, 0x44, 0x12, 0x14, 0x0a, 0x05, 0x73, 0x74, 0x61, 0x72, 0x74, 0x18,
	0x02, 0x20, 0x01, 0x28, 0x03, 0x52, 0x05, 0x73, 0x74, 0x61, 0x72, 0x74, 0x12, 0x10, 0x0a, 0x03,
	0x65, 0x6e, 0x64, 0x18, 0x03, 0x20, 0x01, 0x28, 0x03, 0x52, 0x03, 0x65, 0x6e, 0x64, 0x22, 0x75,
	0x0a, 0x0e, 0x47, 0x65, 0x74, 0x54, 0x72, 0x61, 0x63, 0x6b, 0x73, 0x52, 0x65, 0x70, 0x6c, 0x79,
	0x12, 0x27, 0x0a, 0x06, 0x74, 0x72, 0x61, 0x63, 0x6b, 0x73, 0x18, 0x01, 0x20, 0x03, 0x28, 0x0b,
	0x32, 0x0f, 0x2e, 0x6f, 0x72, 0x63, 0x61, 0x2e, 0x54, 0x72, 0x61, 0x63, 0x6b, 0x44, 0x61, 0x74,
	0x61, 0x52, 0x06, 0x74, 0x72, 0x61, 0x63, 0x6b, 0x73, 0x12, 0x20, 0x0a, 0x0b, 0x74, 0x6f, 0x74,
	0x61, 0x6c, 0x54, 0x72, 0x61, 0x63, 0x6b, 0x73, 0x18, 0x02, 0x20, 0x01, 0x28, 0x03, 0x52, 0x0b,
	0x74, 0x6f, 0x74, 0x61, 0x6c, 0x54, 0x72, 0x61, 0x63, 0x6b, 0x73, 0x12, 0x18, 0x0a, 0x07, 0x6c,
	0x6f, 0x6f, 0x70, 0x69, 0x6e, 0x67, 0x18, 0x03, 0x20, 0x01, 0x28, 0x08, 0x52, 0x07, 0x6c, 0x6f,
	0x6f, 0x70, 0x69, 0x6e, 0x67, 0x32, 0xf4, 0x03, 0x0a, 0x04, 0x4f, 0x72, 0x63, 0x61, 0x12, 0x38,
	0x0a, 0x08, 0x52, 0x65, 0x67, 0x69, 0x73, 0x74, 0x65, 0x72, 0x12, 0x15, 0x2e, 0x6f, 0x72, 0x63,
	0x61, 0x2e, 0x52, 0x65, 0x67, 0x69, 0x73, 0x74, 0x65, 0x72, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73,
	0x74, 0x1a, 0x13, 0x2e, 0x6f, 0x72, 0x63, 0x61, 0x2e, 0x52, 0x65, 0x67, 0x69, 0x73, 0x74, 0x65,
	0x72, 0x52, 0x65, 0x70, 0x6c, 0x79, 0x22, 0x00, 0x12, 0x2c, 0x0a, 0x04, 0x50, 0x6c, 0x61, 0x79,
	0x12, 0x11, 0x2e, 0x6f, 0x72, 0x63, 0x61, 0x2e, 0x50, 0x6c, 0x61, 0x79, 0x52, 0x65, 0x71, 0x75,
	0x65, 0x73, 0x74, 0x1a, 0x0f, 0x2e, 0x6f, 0x72, 0x63, 0x61, 0x2e, 0x50, 0x6c, 0x61, 0x79, 0x52,
	0x65, 0x70, 0x6c, 0x79, 0x22, 0x00, 0x12, 0x36, 0x0a, 0x04, 0x53, 0x6b, 0x69, 0x70, 0x12, 0x16,
	0x2e, 0x6f, 0x72, 0x63, 0x61, 0x2e, 0x47, 0x75, 0x69, 0x6c, 0x64, 0x4f, 0x6e, 0x6c, 0x79, 0x52,
	0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x14, 0x2e, 0x6f, 0x72, 0x63, 0x61, 0x2e, 0x47, 0x75,
	0x69, 0x6c, 0x64, 0x4f, 0x6e, 0x6c, 0x79, 0x52, 0x65, 0x70, 0x6c, 0x79, 0x22, 0x00, 0x12, 0x36,
	0x0a, 0x04, 0x53, 0x74, 0x6f, 0x70, 0x12, 0x16, 0x2e, 0x6f, 0x72, 0x63, 0x61, 0x2e, 0x47, 0x75,
	0x69, 0x6c, 0x64, 0x4f, 0x6e, 0x6c, 0x79, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x14,
	0x2e, 0x6f, 0x72, 0x63, 0x61, 0x2e, 0x47, 0x75, 0x69, 0x6c, 0x64, 0x4f, 0x6e, 0x6c, 0x79, 0x52,
	0x65, 0x70, 0x6c, 0x79, 0x22, 0x00, 0x12, 0x2c, 0x0a, 0x04, 0x53, 0x65, 0x65, 0x6b, 0x12, 0x11,
	0x2e, 0x6f, 0x72, 0x63, 0x61, 0x2e, 0x53, 0x65, 0x65, 0x6b, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73,
	0x74, 0x1a, 0x0f, 0x2e, 0x6f, 0x72, 0x63, 0x61, 0x2e, 0x53, 0x65, 0x65, 0x6b, 0x52, 0x65, 0x70,
	0x6c, 0x79, 0x22, 0x00, 0x12, 0x3b, 0x0a, 0x09, 0x47, 0x65, 0x74, 0x54, 0x72, 0x61, 0x63, 0x6b,
	0x73, 0x12, 0x16, 0x2e, 0x6f, 0x72, 0x63, 0x61, 0x2e, 0x47, 0x65, 0x74, 0x54, 0x72, 0x61, 0x63,
	0x6b, 0x73, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x14, 0x2e, 0x6f, 0x72, 0x63, 0x61,
	0x2e, 0x47, 0x65, 0x74, 0x54, 0x72, 0x61, 0x63, 0x6b, 0x73, 0x52, 0x65, 0x70, 0x6c, 0x79, 0x22,
	0x00, 0x12, 0x37, 0x0a, 0x05, 0x50, 0x61, 0x75, 0x73, 0x65, 0x12, 0x16, 0x2e, 0x6f, 0x72, 0x63,
	0x61, 0x2e, 0x47, 0x75, 0x69, 0x6c, 0x64, 0x4f, 0x6e, 0x6c, 0x79, 0x52, 0x65, 0x71, 0x75, 0x65,
	0x73, 0x74, 0x1a, 0x14, 0x2e, 0x6f, 0x72, 0x63, 0x61, 0x2e, 0x47, 0x75, 0x69, 0x6c, 0x64, 0x4f,
	0x6e, 0x6c, 0x79, 0x52, 0x65, 0x70, 0x6c, 0x79, 0x22, 0x00, 0x12, 0x38, 0x0a, 0x06, 0x52, 0x65,
	0x73, 0x75, 0x6d, 0x65, 0x12, 0x16, 0x2e, 0x6f, 0x72, 0x63, 0x61, 0x2e, 0x47, 0x75, 0x69, 0x6c,
	0x64, 0x4f, 0x6e, 0x6c, 0x79, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x14, 0x2e, 0x6f,
	0x72, 0x63, 0x61, 0x2e, 0x47, 0x75, 0x69, 0x6c, 0x64, 0x4f, 0x6e, 0x6c, 0x79, 0x52, 0x65, 0x70,
	0x6c, 0x79, 0x22, 0x00, 0x12, 0x36, 0x0a, 0x04, 0x4c, 0x6f, 0x6f, 0x70, 0x12, 0x16, 0x2e, 0x6f,
	0x72, 0x63, 0x61, 0x2e, 0x47, 0x75, 0x69, 0x6c, 0x64, 0x4f, 0x6e, 0x6c, 0x79, 0x52, 0x65, 0x71,
	0x75, 0x65, 0x73, 0x74, 0x1a, 0x14, 0x2e, 0x6f, 0x72, 0x63, 0x61, 0x2e, 0x47, 0x75, 0x69, 0x6c,
	0x64, 0x4f, 0x6e, 0x6c, 0x79, 0x52, 0x65, 0x70, 0x6c, 0x79, 0x22, 0x00, 0x42, 0x18, 0x5a, 0x16,
	0x50, 0x72, 0x6f, 0x6a, 0x65, 0x63, 0x74, 0x4f, 0x72, 0x63, 0x61, 0x2f, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x2f, 0x6f, 0x72, 0x63, 0x61, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
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

var file_proto_orca_proto_msgTypes = make([]protoimpl.MessageInfo, 11)
var file_proto_orca_proto_goTypes = []interface{}{
	(*RegisterRequest)(nil),     // 0: orca.RegisterRequest
	(*RegisterReply)(nil),       // 1: orca.RegisterReply
	(*PlayRequest)(nil),         // 2: orca.PlayRequest
	(*PlayReply)(nil),           // 3: orca.PlayReply
	(*TrackData)(nil),           // 4: orca.TrackData
	(*GuildOnlyRequest)(nil),    // 5: orca.GuildOnlyRequest
	(*GuildOnlyReply)(nil),      // 6: orca.GuildOnlyReply
	(*SeekRequest)(nil),         // 7: orca.SeekRequest
	(*SeekReply)(nil),           // 8: orca.SeekReply
	(*GetTracksRequest)(nil),    // 9: orca.GetTracksRequest
	(*GetTracksReply)(nil),      // 10: orca.GetTracksReply
	(*durationpb.Duration)(nil), // 11: google.protobuf.Duration
}
var file_proto_orca_proto_depIdxs = []int32{
	4,  // 0: orca.PlayReply.track:type_name -> orca.TrackData
	11, // 1: orca.TrackData.position:type_name -> google.protobuf.Duration
	11, // 2: orca.TrackData.duration:type_name -> google.protobuf.Duration
	11, // 3: orca.SeekRequest.position:type_name -> google.protobuf.Duration
	4,  // 4: orca.GetTracksReply.tracks:type_name -> orca.TrackData
	0,  // 5: orca.Orca.Register:input_type -> orca.RegisterRequest
	2,  // 6: orca.Orca.Play:input_type -> orca.PlayRequest
	5,  // 7: orca.Orca.Skip:input_type -> orca.GuildOnlyRequest
	5,  // 8: orca.Orca.Stop:input_type -> orca.GuildOnlyRequest
	7,  // 9: orca.Orca.Seek:input_type -> orca.SeekRequest
	9,  // 10: orca.Orca.GetTracks:input_type -> orca.GetTracksRequest
	5,  // 11: orca.Orca.Pause:input_type -> orca.GuildOnlyRequest
	5,  // 12: orca.Orca.Resume:input_type -> orca.GuildOnlyRequest
	5,  // 13: orca.Orca.Loop:input_type -> orca.GuildOnlyRequest
	1,  // 14: orca.Orca.Register:output_type -> orca.RegisterReply
	3,  // 15: orca.Orca.Play:output_type -> orca.PlayReply
	6,  // 16: orca.Orca.Skip:output_type -> orca.GuildOnlyReply
	6,  // 17: orca.Orca.Stop:output_type -> orca.GuildOnlyReply
	8,  // 18: orca.Orca.Seek:output_type -> orca.SeekReply
	10, // 19: orca.Orca.GetTracks:output_type -> orca.GetTracksReply
	6,  // 20: orca.Orca.Pause:output_type -> orca.GuildOnlyReply
	6,  // 21: orca.Orca.Resume:output_type -> orca.GuildOnlyReply
	6,  // 22: orca.Orca.Loop:output_type -> orca.GuildOnlyReply
	14, // [14:23] is the sub-list for method output_type
	5,  // [5:14] is the sub-list for method input_type
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
			switch v := v.(*RegisterRequest); i {
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
			switch v := v.(*RegisterReply); i {
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
		file_proto_orca_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
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
		file_proto_orca_proto_msgTypes[4].Exporter = func(v interface{}, i int) interface{} {
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
		file_proto_orca_proto_msgTypes[5].Exporter = func(v interface{}, i int) interface{} {
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
		file_proto_orca_proto_msgTypes[6].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GuildOnlyReply); i {
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
		file_proto_orca_proto_msgTypes[8].Exporter = func(v interface{}, i int) interface{} {
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
		file_proto_orca_proto_msgTypes[9].Exporter = func(v interface{}, i int) interface{} {
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
		file_proto_orca_proto_msgTypes[10].Exporter = func(v interface{}, i int) interface{} {
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
			NumMessages:   11,
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
