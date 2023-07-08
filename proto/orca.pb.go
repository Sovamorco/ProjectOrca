// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.28.1
// 	protoc        v4.23.4
// source: proto/orca.proto

package orca

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type PlayRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	GuildID   string `protobuf:"bytes,1,opt,name=guildID,proto3" json:"guildID,omitempty"`
	ChannelID string `protobuf:"bytes,2,opt,name=channelID,proto3" json:"channelID,omitempty"`
	Url       string `protobuf:"bytes,3,opt,name=url,proto3" json:"url,omitempty"`
}

func (x *PlayRequest) Reset() {
	*x = PlayRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_proto_orca_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *PlayRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*PlayRequest) ProtoMessage() {}

func (x *PlayRequest) ProtoReflect() protoreflect.Message {
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

// Deprecated: Use PlayRequest.ProtoReflect.Descriptor instead.
func (*PlayRequest) Descriptor() ([]byte, []int) {
	return file_proto_orca_proto_rawDescGZIP(), []int{0}
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

type PlayReply struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Track *TrackData `protobuf:"bytes,1,opt,name=track,proto3" json:"track,omitempty"`
}

func (x *PlayReply) Reset() {
	*x = PlayReply{}
	if protoimpl.UnsafeEnabled {
		mi := &file_proto_orca_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *PlayReply) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*PlayReply) ProtoMessage() {}

func (x *PlayReply) ProtoReflect() protoreflect.Message {
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

// Deprecated: Use PlayReply.ProtoReflect.Descriptor instead.
func (*PlayReply) Descriptor() ([]byte, []int) {
	return file_proto_orca_proto_rawDescGZIP(), []int{1}
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

	Title       string            `protobuf:"bytes,1,opt,name=title,proto3" json:"title,omitempty"`
	OriginalURL string            `protobuf:"bytes,2,opt,name=originalURL,proto3" json:"originalURL,omitempty"`
	Url         string            `protobuf:"bytes,3,opt,name=url,proto3" json:"url,omitempty"`
	HttpHeaders map[string]string `protobuf:"bytes,4,rep,name=httpHeaders,proto3" json:"httpHeaders,omitempty" protobuf_key:"bytes,1,opt,name=key,proto3" protobuf_val:"bytes,2,opt,name=value,proto3"`
}

func (x *TrackData) Reset() {
	*x = TrackData{}
	if protoimpl.UnsafeEnabled {
		mi := &file_proto_orca_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *TrackData) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*TrackData) ProtoMessage() {}

func (x *TrackData) ProtoReflect() protoreflect.Message {
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

// Deprecated: Use TrackData.ProtoReflect.Descriptor instead.
func (*TrackData) Descriptor() ([]byte, []int) {
	return file_proto_orca_proto_rawDescGZIP(), []int{2}
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

func (x *TrackData) GetHttpHeaders() map[string]string {
	if x != nil {
		return x.HttpHeaders
	}
	return nil
}

var File_proto_orca_proto protoreflect.FileDescriptor

var file_proto_orca_proto_rawDesc = []byte{
	0x0a, 0x10, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x6f, 0x72, 0x63, 0x61, 0x2e, 0x70, 0x72, 0x6f,
	0x74, 0x6f, 0x12, 0x04, 0x6f, 0x72, 0x63, 0x61, 0x22, 0x57, 0x0a, 0x0b, 0x50, 0x6c, 0x61, 0x79,
	0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x18, 0x0a, 0x07, 0x67, 0x75, 0x69, 0x6c, 0x64,
	0x49, 0x44, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x07, 0x67, 0x75, 0x69, 0x6c, 0x64, 0x49,
	0x44, 0x12, 0x1c, 0x0a, 0x09, 0x63, 0x68, 0x61, 0x6e, 0x6e, 0x65, 0x6c, 0x49, 0x44, 0x18, 0x02,
	0x20, 0x01, 0x28, 0x09, 0x52, 0x09, 0x63, 0x68, 0x61, 0x6e, 0x6e, 0x65, 0x6c, 0x49, 0x44, 0x12,
	0x10, 0x0a, 0x03, 0x75, 0x72, 0x6c, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x03, 0x75, 0x72,
	0x6c, 0x22, 0x32, 0x0a, 0x09, 0x50, 0x6c, 0x61, 0x79, 0x52, 0x65, 0x70, 0x6c, 0x79, 0x12, 0x25,
	0x0a, 0x05, 0x74, 0x72, 0x61, 0x63, 0x6b, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x0f, 0x2e,
	0x6f, 0x72, 0x63, 0x61, 0x2e, 0x54, 0x72, 0x61, 0x63, 0x6b, 0x44, 0x61, 0x74, 0x61, 0x52, 0x05,
	0x74, 0x72, 0x61, 0x63, 0x6b, 0x22, 0xd9, 0x01, 0x0a, 0x09, 0x54, 0x72, 0x61, 0x63, 0x6b, 0x44,
	0x61, 0x74, 0x61, 0x12, 0x14, 0x0a, 0x05, 0x74, 0x69, 0x74, 0x6c, 0x65, 0x18, 0x01, 0x20, 0x01,
	0x28, 0x09, 0x52, 0x05, 0x74, 0x69, 0x74, 0x6c, 0x65, 0x12, 0x20, 0x0a, 0x0b, 0x6f, 0x72, 0x69,
	0x67, 0x69, 0x6e, 0x61, 0x6c, 0x55, 0x52, 0x4c, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0b,
	0x6f, 0x72, 0x69, 0x67, 0x69, 0x6e, 0x61, 0x6c, 0x55, 0x52, 0x4c, 0x12, 0x10, 0x0a, 0x03, 0x75,
	0x72, 0x6c, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x03, 0x75, 0x72, 0x6c, 0x12, 0x42, 0x0a,
	0x0b, 0x68, 0x74, 0x74, 0x70, 0x48, 0x65, 0x61, 0x64, 0x65, 0x72, 0x73, 0x18, 0x04, 0x20, 0x03,
	0x28, 0x0b, 0x32, 0x20, 0x2e, 0x6f, 0x72, 0x63, 0x61, 0x2e, 0x54, 0x72, 0x61, 0x63, 0x6b, 0x44,
	0x61, 0x74, 0x61, 0x2e, 0x48, 0x74, 0x74, 0x70, 0x48, 0x65, 0x61, 0x64, 0x65, 0x72, 0x73, 0x45,
	0x6e, 0x74, 0x72, 0x79, 0x52, 0x0b, 0x68, 0x74, 0x74, 0x70, 0x48, 0x65, 0x61, 0x64, 0x65, 0x72,
	0x73, 0x1a, 0x3e, 0x0a, 0x10, 0x48, 0x74, 0x74, 0x70, 0x48, 0x65, 0x61, 0x64, 0x65, 0x72, 0x73,
	0x45, 0x6e, 0x74, 0x72, 0x79, 0x12, 0x10, 0x0a, 0x03, 0x6b, 0x65, 0x79, 0x18, 0x01, 0x20, 0x01,
	0x28, 0x09, 0x52, 0x03, 0x6b, 0x65, 0x79, 0x12, 0x14, 0x0a, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65,
	0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x3a, 0x02, 0x38,
	0x01, 0x32, 0x34, 0x0a, 0x04, 0x4f, 0x72, 0x63, 0x61, 0x12, 0x2c, 0x0a, 0x04, 0x50, 0x6c, 0x61,
	0x79, 0x12, 0x11, 0x2e, 0x6f, 0x72, 0x63, 0x61, 0x2e, 0x50, 0x6c, 0x61, 0x79, 0x52, 0x65, 0x71,
	0x75, 0x65, 0x73, 0x74, 0x1a, 0x0f, 0x2e, 0x6f, 0x72, 0x63, 0x61, 0x2e, 0x50, 0x6c, 0x61, 0x79,
	0x52, 0x65, 0x70, 0x6c, 0x79, 0x22, 0x00, 0x42, 0x18, 0x5a, 0x16, 0x50, 0x72, 0x6f, 0x6a, 0x65,
	0x63, 0x74, 0x4f, 0x72, 0x63, 0x61, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x6f, 0x72, 0x63,
	0x61, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
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

var file_proto_orca_proto_msgTypes = make([]protoimpl.MessageInfo, 4)
var file_proto_orca_proto_goTypes = []interface{}{
	(*PlayRequest)(nil), // 0: orca.PlayRequest
	(*PlayReply)(nil),   // 1: orca.PlayReply
	(*TrackData)(nil),   // 2: orca.TrackData
	nil,                 // 3: orca.TrackData.HttpHeadersEntry
}
var file_proto_orca_proto_depIdxs = []int32{
	2, // 0: orca.PlayReply.track:type_name -> orca.TrackData
	3, // 1: orca.TrackData.httpHeaders:type_name -> orca.TrackData.HttpHeadersEntry
	0, // 2: orca.Orca.Play:input_type -> orca.PlayRequest
	1, // 3: orca.Orca.Play:output_type -> orca.PlayReply
	3, // [3:4] is the sub-list for method output_type
	2, // [2:3] is the sub-list for method input_type
	2, // [2:2] is the sub-list for extension type_name
	2, // [2:2] is the sub-list for extension extendee
	0, // [0:2] is the sub-list for field type_name
}

func init() { file_proto_orca_proto_init() }
func file_proto_orca_proto_init() {
	if File_proto_orca_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_proto_orca_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
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
		file_proto_orca_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
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
		file_proto_orca_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
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
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_proto_orca_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   4,
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
