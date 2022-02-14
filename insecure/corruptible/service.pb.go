// Code generated by protoc-gen-go. DO NOT EDIT.
// source: insecure/corruptible/service.proto

package corruptible

import (
	fmt "fmt"
	proto "github.com/golang/protobuf/proto"
	math "math"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion3 // please upgrade the proto package

type Protocol int32

const (
	Protocol_UNKNOWN   Protocol = 0
	Protocol_UNICAST   Protocol = 1
	Protocol_MULTICAST Protocol = 2
	Protocol_PUBLISH   Protocol = 3
)

var Protocol_name = map[int32]string{
	0: "UNKNOWN",
	1: "UNICAST",
	2: "MULTICAST",
	3: "PUBLISH",
}

var Protocol_value = map[string]int32{
	"UNKNOWN":   0,
	"UNICAST":   1,
	"MULTICAST": 2,
	"PUBLISH":   3,
}

func (x Protocol) String() string {
	return proto.EnumName(Protocol_name, int32(x))
}

func (Protocol) EnumDescriptor() ([]byte, []int) {
	return fileDescriptor_0f4271bd1524ad99, []int{0}
}

// Message is represents the message exchanged between the Zombi and Attacker services.
type Message struct {
	ChannelID            string   `protobuf:"bytes,1,opt,name=ChannelID,proto3" json:"ChannelID,omitempty"`
	OriginID             []byte   `protobuf:"bytes,2,opt,name=OriginID,proto3" json:"OriginID,omitempty"`
	Targets              uint32   `protobuf:"varint,3,opt,name=targets,proto3" json:"targets,omitempty"`
	TargetIDs            [][]byte `protobuf:"bytes,4,rep,name=TargetIDs,proto3" json:"TargetIDs,omitempty"`
	Payload              []byte   `protobuf:"bytes,5,opt,name=Payload,proto3" json:"Payload,omitempty"`
	Type                 string   `protobuf:"bytes,6,opt,name=Type,proto3" json:"Type,omitempty"`
	Protocol             Protocol `protobuf:"varint,7,opt,name=protocol,proto3,enum=corruptible.Protocol" json:"protocol,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *Message) Reset()         { *m = Message{} }
func (m *Message) String() string { return proto.CompactTextString(m) }
func (*Message) ProtoMessage()    {}
func (*Message) Descriptor() ([]byte, []int) {
	return fileDescriptor_0f4271bd1524ad99, []int{0}
}

func (m *Message) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Message.Unmarshal(m, b)
}
func (m *Message) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Message.Marshal(b, m, deterministic)
}
func (m *Message) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Message.Merge(m, src)
}
func (m *Message) XXX_Size() int {
	return xxx_messageInfo_Message.Size(m)
}
func (m *Message) XXX_DiscardUnknown() {
	xxx_messageInfo_Message.DiscardUnknown(m)
}

var xxx_messageInfo_Message proto.InternalMessageInfo

func (m *Message) GetChannelID() string {
	if m != nil {
		return m.ChannelID
	}
	return ""
}

func (m *Message) GetOriginID() []byte {
	if m != nil {
		return m.OriginID
	}
	return nil
}

func (m *Message) GetTargets() uint32 {
	if m != nil {
		return m.Targets
	}
	return 0
}

func (m *Message) GetTargetIDs() [][]byte {
	if m != nil {
		return m.TargetIDs
	}
	return nil
}

func (m *Message) GetPayload() []byte {
	if m != nil {
		return m.Payload
	}
	return nil
}

func (m *Message) GetType() string {
	if m != nil {
		return m.Type
	}
	return ""
}

func (m *Message) GetProtocol() Protocol {
	if m != nil {
		return m.Protocol
	}
	return Protocol_UNKNOWN
}

// AttackerRegisterMessage is the message an attacker uses to register itself to the chosen Zombie,
// and takes its control.
type AttackerRegisterMessage struct {
	Ip                   string   `protobuf:"bytes,1,opt,name=ip,proto3" json:"ip,omitempty"`
	Port                 string   `protobuf:"bytes,2,opt,name=port,proto3" json:"port,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *AttackerRegisterMessage) Reset()         { *m = AttackerRegisterMessage{} }
func (m *AttackerRegisterMessage) String() string { return proto.CompactTextString(m) }
func (*AttackerRegisterMessage) ProtoMessage()    {}
func (*AttackerRegisterMessage) Descriptor() ([]byte, []int) {
	return fileDescriptor_0f4271bd1524ad99, []int{1}
}

func (m *AttackerRegisterMessage) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_AttackerRegisterMessage.Unmarshal(m, b)
}
func (m *AttackerRegisterMessage) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_AttackerRegisterMessage.Marshal(b, m, deterministic)
}
func (m *AttackerRegisterMessage) XXX_Merge(src proto.Message) {
	xxx_messageInfo_AttackerRegisterMessage.Merge(m, src)
}
func (m *AttackerRegisterMessage) XXX_Size() int {
	return xxx_messageInfo_AttackerRegisterMessage.Size(m)
}
func (m *AttackerRegisterMessage) XXX_DiscardUnknown() {
	xxx_messageInfo_AttackerRegisterMessage.DiscardUnknown(m)
}

var xxx_messageInfo_AttackerRegisterMessage proto.InternalMessageInfo

func (m *AttackerRegisterMessage) GetIp() string {
	if m != nil {
		return m.Ip
	}
	return ""
}

func (m *AttackerRegisterMessage) GetPort() string {
	if m != nil {
		return m.Port
	}
	return ""
}

// ActionResponse is used as the gRPC result to denote whether an action was successful on the
// remote Zombie or not.
type ActionResponse struct {
	Complete             bool     `protobuf:"varint,1,opt,name=complete,proto3" json:"complete,omitempty"`
	Error                string   `protobuf:"bytes,2,opt,name=error,proto3" json:"error,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *ActionResponse) Reset()         { *m = ActionResponse{} }
func (m *ActionResponse) String() string { return proto.CompactTextString(m) }
func (*ActionResponse) ProtoMessage()    {}
func (*ActionResponse) Descriptor() ([]byte, []int) {
	return fileDescriptor_0f4271bd1524ad99, []int{2}
}

func (m *ActionResponse) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_ActionResponse.Unmarshal(m, b)
}
func (m *ActionResponse) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_ActionResponse.Marshal(b, m, deterministic)
}
func (m *ActionResponse) XXX_Merge(src proto.Message) {
	xxx_messageInfo_ActionResponse.Merge(m, src)
}
func (m *ActionResponse) XXX_Size() int {
	return xxx_messageInfo_ActionResponse.Size(m)
}
func (m *ActionResponse) XXX_DiscardUnknown() {
	xxx_messageInfo_ActionResponse.DiscardUnknown(m)
}

var xxx_messageInfo_ActionResponse proto.InternalMessageInfo

func (m *ActionResponse) GetComplete() bool {
	if m != nil {
		return m.Complete
	}
	return false
}

func (m *ActionResponse) GetError() string {
	if m != nil {
		return m.Error
	}
	return ""
}

func init() {
	proto.RegisterEnum("corruptible.Protocol", Protocol_name, Protocol_value)
	proto.RegisterType((*Message)(nil), "corruptible.Message")
	proto.RegisterType((*AttackerRegisterMessage)(nil), "corruptible.AttackerRegisterMessage")
	proto.RegisterType((*ActionResponse)(nil), "corruptible.ActionResponse")
}

func init() {
	proto.RegisterFile("insecure/corruptible/service.proto", fileDescriptor_0f4271bd1524ad99)
}

var fileDescriptor_0f4271bd1524ad99 = []byte{
	// 409 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x9c, 0x92, 0xc1, 0x6e, 0xd3, 0x40,
	0x10, 0x86, 0xbb, 0x49, 0x1a, 0x3b, 0xd3, 0x36, 0x8a, 0x56, 0x01, 0x56, 0x85, 0x83, 0x65, 0x71,
	0xb0, 0x38, 0xa4, 0x22, 0x1c, 0x11, 0x88, 0xa4, 0x39, 0x60, 0x68, 0x93, 0x68, 0x93, 0x08, 0xa9,
	0x37, 0xdb, 0x1d, 0x99, 0x15, 0xae, 0x77, 0xb5, 0xbb, 0x45, 0xea, 0x8b, 0xf0, 0x74, 0x3c, 0x0c,
	0xf2, 0x26, 0x4e, 0x63, 0x09, 0x24, 0xd4, 0xdb, 0xfc, 0x3b, 0x33, 0xff, 0xcc, 0x67, 0x0f, 0x84,
	0xa2, 0x34, 0x98, 0xdd, 0x6b, 0xbc, 0xc8, 0xa4, 0xd6, 0xf7, 0xca, 0x8a, 0xb4, 0xc0, 0x0b, 0x83,
	0xfa, 0xa7, 0xc8, 0x70, 0xa4, 0xb4, 0xb4, 0x92, 0x9e, 0x1c, 0xa4, 0xc2, 0xdf, 0x04, 0xbc, 0x6b,
	0x34, 0x26, 0xc9, 0x91, 0xbe, 0x82, 0xde, 0xe5, 0xf7, 0xa4, 0x2c, 0xb1, 0x88, 0x67, 0x8c, 0x04,
	0x24, 0xea, 0xf1, 0xc7, 0x07, 0x7a, 0x0e, 0xfe, 0x42, 0x8b, 0x5c, 0x94, 0xf1, 0x8c, 0xb5, 0x02,
	0x12, 0x9d, 0xf2, 0xbd, 0xa6, 0x0c, 0x3c, 0x9b, 0xe8, 0x1c, 0xad, 0x61, 0xed, 0x80, 0x44, 0x67,
	0xbc, 0x96, 0x95, 0xe7, 0xda, 0x85, 0xf1, 0xcc, 0xb0, 0x4e, 0xd0, 0x8e, 0x4e, 0xf9, 0xe3, 0x43,
	0xd5, 0xb7, 0x4c, 0x1e, 0x0a, 0x99, 0xdc, 0xb2, 0x63, 0x67, 0x59, 0x4b, 0x4a, 0xa1, 0xb3, 0x7e,
	0x50, 0xc8, 0xba, 0x6e, 0x0d, 0x17, 0xd3, 0xb7, 0xe0, 0x3b, 0x82, 0x4c, 0x16, 0xcc, 0x0b, 0x48,
	0xd4, 0x1f, 0x3f, 0x1b, 0x1d, 0xb0, 0x8c, 0x96, 0xbb, 0x24, 0xdf, 0x97, 0x85, 0x1f, 0xe0, 0xc5,
	0xc4, 0xda, 0x24, 0xfb, 0x81, 0x9a, 0x63, 0x2e, 0x8c, 0x45, 0x5d, 0xd3, 0xf6, 0xa1, 0x25, 0xd4,
	0x0e, 0xb3, 0x25, 0x54, 0x35, 0x51, 0x49, 0x6d, 0x1d, 0x5b, 0x8f, 0xbb, 0x38, 0x9c, 0x42, 0x7f,
	0x92, 0x59, 0x21, 0x4b, 0x8e, 0x46, 0xc9, 0xd2, 0x60, 0xf5, 0x15, 0x32, 0x79, 0xa7, 0x0a, 0xb4,
	0xe8, 0x7a, 0x7d, 0xbe, 0xd7, 0x74, 0x08, 0xc7, 0xa8, 0xb5, 0xd4, 0x3b, 0x8b, 0xad, 0x78, 0xf3,
	0x09, 0xfc, 0x7a, 0x31, 0x7a, 0x02, 0xde, 0x66, 0xfe, 0x75, 0xbe, 0xf8, 0x36, 0x1f, 0x1c, 0x6d,
	0x45, 0x7c, 0x39, 0x59, 0xad, 0x07, 0x84, 0x9e, 0x41, 0xef, 0x7a, 0x73, 0xb5, 0xde, 0xca, 0x56,
	0x95, 0x5b, 0x6e, 0xa6, 0x57, 0xf1, 0xea, 0xf3, 0xa0, 0x3d, 0xfe, 0x45, 0xa0, 0x7b, 0x23, 0xef,
	0x52, 0x81, 0x74, 0x01, 0x7e, 0xcd, 0x41, 0x5f, 0x37, 0xe0, 0xff, 0x81, 0x79, 0xfe, 0xb2, 0x59,
	0xd5, 0xa0, 0x09, 0x8f, 0xe8, 0x7b, 0xe8, 0xac, 0xb0, 0xbc, 0xa5, 0xc3, 0x46, 0xd9, 0xff, 0x35,
	0x8f, 0xbf, 0x80, 0x5f, 0x8f, 0xa5, 0x1f, 0xc1, 0x5b, 0xa4, 0xd5, 0xa1, 0xe1, 0x93, 0xbc, 0xa6,
	0xcf, 0x6f, 0x86, 0x7f, 0xbb, 0xdd, 0xb4, 0xeb, 0xfe, 0xe5, 0xbb, 0x3f, 0x01, 0x00, 0x00, 0xff,
	0xff, 0xfe, 0x92, 0x3a, 0x9b, 0xda, 0x02, 0x00, 0x00,
}
