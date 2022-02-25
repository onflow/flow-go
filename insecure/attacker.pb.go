// Code generated by protoc-gen-go. DO NOT EDIT.
// source: insecure/attacker.proto

package insecure

import (
	context "context"
	fmt "fmt"
	proto "github.com/golang/protobuf/proto"
	empty "github.com/golang/protobuf/ptypes/empty"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
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
	return fileDescriptor_de1530a0da5eef1a, []int{0}
}

// Message is represents the message exchanged between the CorruptibleConduitFactory and Attacker services.
type Message struct {
	ChannelID            string   `protobuf:"bytes,1,opt,name=ChannelID,proto3" json:"ChannelID,omitempty"`
	OriginID             []byte   `protobuf:"bytes,2,opt,name=OriginID,proto3" json:"OriginID,omitempty"`
	Targets              uint32   `protobuf:"varint,3,opt,name=Targets,proto3" json:"Targets,omitempty"`
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
	return fileDescriptor_de1530a0da5eef1a, []int{0}
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

// AttackerRegisterMessage is the message an attacker uses to register itself to the chosen CorruptibleConduitFactory,
// and takes its control.
type AttackerRegisterMessage struct {
	Address              string   `protobuf:"bytes,1,opt,name=address,proto3" json:"address,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *AttackerRegisterMessage) Reset()         { *m = AttackerRegisterMessage{} }
func (m *AttackerRegisterMessage) String() string { return proto.CompactTextString(m) }
func (*AttackerRegisterMessage) ProtoMessage()    {}
func (*AttackerRegisterMessage) Descriptor() ([]byte, []int) {
	return fileDescriptor_de1530a0da5eef1a, []int{1}
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

func (m *AttackerRegisterMessage) GetAddress() string {
	if m != nil {
		return m.Address
	}
	return ""
}

func init() {
	proto.RegisterEnum("corruptible.Protocol", Protocol_name, Protocol_value)
	proto.RegisterType((*Message)(nil), "corruptible.Message")
	proto.RegisterType((*AttackerRegisterMessage)(nil), "corruptible.AttackerRegisterMessage")
}

func init() { proto.RegisterFile("insecure/attacker.proto", fileDescriptor_de1530a0da5eef1a) }

var fileDescriptor_de1530a0da5eef1a = []byte{
	// 398 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x9c, 0x52, 0x4d, 0x6f, 0xd3, 0x40,
	0x10, 0xed, 0x36, 0xa5, 0x76, 0xa6, 0x2d, 0x8a, 0x56, 0xd0, 0x2e, 0x81, 0x83, 0x15, 0x71, 0xb0,
	0x38, 0x38, 0x22, 0x3d, 0x72, 0xa1, 0x75, 0xf8, 0x30, 0xb4, 0x89, 0xe5, 0x3a, 0x42, 0xe2, 0xb6,
	0xb6, 0x07, 0x63, 0x61, 0xbc, 0xd6, 0xee, 0x1a, 0xc9, 0xbf, 0x8d, 0xbf, 0xc2, 0x8f, 0x41, 0xb6,
	0xb3, 0x09, 0x3d, 0x20, 0x24, 0x6e, 0xf3, 0xe6, 0xcd, 0x3c, 0xbd, 0xb7, 0xb3, 0x70, 0x51, 0x54,
	0x0a, 0xd3, 0x46, 0xe2, 0x9c, 0x6b, 0xcd, 0xd3, 0x6f, 0x28, 0xbd, 0x5a, 0x0a, 0x2d, 0xe8, 0x49,
	0x2a, 0xa4, 0x6c, 0x6a, 0x5d, 0x24, 0x25, 0x4e, 0x9f, 0xe6, 0x42, 0xe4, 0x25, 0xce, 0x7b, 0x2a,
	0x69, 0xbe, 0xcc, 0xf1, 0x7b, 0xad, 0xdb, 0x61, 0x72, 0xf6, 0x8b, 0x80, 0x75, 0x8b, 0x4a, 0xf1,
	0x1c, 0xe9, 0x33, 0x18, 0xfb, 0x5f, 0x79, 0x55, 0x61, 0x19, 0x2c, 0x19, 0x71, 0x88, 0x3b, 0x8e,
	0xf6, 0x0d, 0x3a, 0x05, 0x7b, 0x2d, 0x8b, 0xbc, 0xa8, 0x82, 0x25, 0x3b, 0x74, 0x88, 0x7b, 0x1a,
	0xed, 0x30, 0x65, 0x60, 0xc5, 0x5c, 0xe6, 0xa8, 0x15, 0x1b, 0x39, 0xc4, 0x3d, 0x8b, 0x0c, 0xec,
	0x34, 0x87, 0x32, 0x58, 0x2a, 0x76, 0xe4, 0x8c, 0xdc, 0xd3, 0x68, 0xdf, 0xe8, 0xf6, 0x42, 0xde,
	0x96, 0x82, 0x67, 0xec, 0x41, 0x2f, 0x69, 0x20, 0xa5, 0x70, 0x14, 0xb7, 0x35, 0xb2, 0xe3, 0xde,
	0x46, 0x5f, 0xd3, 0x97, 0x60, 0xf7, 0xa6, 0x53, 0x51, 0x32, 0xcb, 0x21, 0xee, 0xc3, 0xc5, 0x63,
	0xef, 0x8f, 0xa0, 0x5e, 0xb8, 0x25, 0xa3, 0xdd, 0xd8, 0xec, 0x12, 0x2e, 0xae, 0xb6, 0x4f, 0x13,
	0x61, 0x5e, 0x28, 0x8d, 0xd2, 0xa4, 0x65, 0x60, 0xf1, 0x2c, 0x93, 0xa8, 0xd4, 0x36, 0xab, 0x81,
	0x2f, 0x5e, 0x83, 0x6d, 0xa4, 0xe8, 0x09, 0x58, 0x9b, 0xd5, 0xc7, 0xd5, 0xfa, 0xd3, 0x6a, 0x72,
	0x30, 0x80, 0xc0, 0xbf, 0xba, 0x8b, 0x27, 0x84, 0x9e, 0xc1, 0xf8, 0x76, 0x73, 0x13, 0x0f, 0xf0,
	0xb0, 0xe3, 0xc2, 0xcd, 0xf5, 0x4d, 0x70, 0xf7, 0x7e, 0x32, 0x5a, 0xfc, 0x24, 0xf0, 0xc4, 0xdf,
	0x3b, 0xf3, 0x45, 0x95, 0x35, 0x85, 0x7e, 0xcb, 0x53, 0x2d, 0x64, 0x4b, 0x23, 0x98, 0x18, 0x33,
	0xc6, 0x1c, 0x7d, 0x7e, 0x2f, 0xc9, 0x5f, 0x3c, 0x4f, 0xcf, 0xbd, 0xe1, 0x96, 0x9e, 0xb9, 0xa5,
	0xf7, 0xa6, 0xbb, 0xe5, 0xec, 0x80, 0x7e, 0x80, 0xf3, 0x50, 0x8a, 0x14, 0x95, 0x32, 0xbb, 0x26,
	0xe7, 0xa3, 0x7b, 0xca, 0xff, 0x54, 0x72, 0xc9, 0xe2, 0x1d, 0xd8, 0x3b, 0x5f, 0xaf, 0xc0, 0x5a,
	0x27, 0x0a, 0xe5, 0x8f, 0xff, 0x10, 0xba, 0x86, 0xcf, 0xb6, 0xf9, 0xa1, 0xc9, 0x71, 0xcf, 0x5f,
	0xfe, 0x0e, 0x00, 0x00, 0xff, 0xff, 0xae, 0x06, 0xa2, 0x72, 0xb4, 0x02, 0x00, 0x00,
}

// Reference imports to suppress errors if they are not otherwise used.
var _ context.Context
var _ grpc.ClientConn

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
const _ = grpc.SupportPackageIsVersion4

// CorruptibleConduitFactoryClient is the client API for CorruptibleConduitFactory service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://godoc.org/google.golang.org/grpc#ClientConn.NewStream.
type CorruptibleConduitFactoryClient interface {
	RegisterAttacker(ctx context.Context, in *AttackerRegisterMessage, opts ...grpc.CallOption) (*empty.Empty, error)
	ProcessAttackerMessage(ctx context.Context, opts ...grpc.CallOption) (CorruptibleConduitFactory_ProcessAttackerMessageClient, error)
}

type corruptibleConduitFactoryClient struct {
	cc *grpc.ClientConn
}

func NewCorruptibleConduitFactoryClient(cc *grpc.ClientConn) CorruptibleConduitFactoryClient {
	return &corruptibleConduitFactoryClient{cc}
}

func (c *corruptibleConduitFactoryClient) RegisterAttacker(ctx context.Context, in *AttackerRegisterMessage, opts ...grpc.CallOption) (*empty.Empty, error) {
	out := new(empty.Empty)
	err := c.cc.Invoke(ctx, "/corruptible.CorruptibleConduitFactory/RegisterAttacker", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *corruptibleConduitFactoryClient) ProcessAttackerMessage(ctx context.Context, opts ...grpc.CallOption) (CorruptibleConduitFactory_ProcessAttackerMessageClient, error) {
	stream, err := c.cc.NewStream(ctx, &_CorruptibleConduitFactory_serviceDesc.Streams[0], "/corruptible.CorruptibleConduitFactory/ProcessAttackerMessage", opts...)
	if err != nil {
		return nil, err
	}
	x := &corruptibleConduitFactoryProcessAttackerMessageClient{stream}
	return x, nil
}

type CorruptibleConduitFactory_ProcessAttackerMessageClient interface {
	Send(*Message) error
	CloseAndRecv() (*empty.Empty, error)
	grpc.ClientStream
}

type corruptibleConduitFactoryProcessAttackerMessageClient struct {
	grpc.ClientStream
}

func (x *corruptibleConduitFactoryProcessAttackerMessageClient) Send(m *Message) error {
	return x.ClientStream.SendMsg(m)
}

func (x *corruptibleConduitFactoryProcessAttackerMessageClient) CloseAndRecv() (*empty.Empty, error) {
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	m := new(empty.Empty)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// CorruptibleConduitFactoryServer is the server API for CorruptibleConduitFactory service.
type CorruptibleConduitFactoryServer interface {
	RegisterAttacker(context.Context, *AttackerRegisterMessage) (*empty.Empty, error)
	ProcessAttackerMessage(CorruptibleConduitFactory_ProcessAttackerMessageServer) error
}

// UnimplementedCorruptibleConduitFactoryServer can be embedded to have forward compatible implementations.
type UnimplementedCorruptibleConduitFactoryServer struct {
}

func (*UnimplementedCorruptibleConduitFactoryServer) RegisterAttacker(ctx context.Context, req *AttackerRegisterMessage) (*empty.Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method RegisterAttacker not implemented")
}
func (*UnimplementedCorruptibleConduitFactoryServer) ProcessAttackerMessage(srv CorruptibleConduitFactory_ProcessAttackerMessageServer) error {
	return status.Errorf(codes.Unimplemented, "method ProcessAttackerMessage not implemented")
}

func RegisterCorruptibleConduitFactoryServer(s *grpc.Server, srv CorruptibleConduitFactoryServer) {
	s.RegisterService(&_CorruptibleConduitFactory_serviceDesc, srv)
}

func _CorruptibleConduitFactory_RegisterAttacker_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(AttackerRegisterMessage)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(CorruptibleConduitFactoryServer).RegisterAttacker(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/corruptible.CorruptibleConduitFactory/RegisterAttacker",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(CorruptibleConduitFactoryServer).RegisterAttacker(ctx, req.(*AttackerRegisterMessage))
	}
	return interceptor(ctx, in, info, handler)
}

func _CorruptibleConduitFactory_ProcessAttackerMessage_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(CorruptibleConduitFactoryServer).ProcessAttackerMessage(&corruptibleConduitFactoryProcessAttackerMessageServer{stream})
}

type CorruptibleConduitFactory_ProcessAttackerMessageServer interface {
	SendAndClose(*empty.Empty) error
	Recv() (*Message, error)
	grpc.ServerStream
}

type corruptibleConduitFactoryProcessAttackerMessageServer struct {
	grpc.ServerStream
}

func (x *corruptibleConduitFactoryProcessAttackerMessageServer) SendAndClose(m *empty.Empty) error {
	return x.ServerStream.SendMsg(m)
}

func (x *corruptibleConduitFactoryProcessAttackerMessageServer) Recv() (*Message, error) {
	m := new(Message)
	if err := x.ServerStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

var _CorruptibleConduitFactory_serviceDesc = grpc.ServiceDesc{
	ServiceName: "corruptible.CorruptibleConduitFactory",
	HandlerType: (*CorruptibleConduitFactoryServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "RegisterAttacker",
			Handler:    _CorruptibleConduitFactory_RegisterAttacker_Handler,
		},
	},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "ProcessAttackerMessage",
			Handler:       _CorruptibleConduitFactory_ProcessAttackerMessage_Handler,
			ClientStreams: true,
		},
	},
	Metadata: "insecure/attacker.proto",
}

// AttackerClient is the client API for Attacker service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://godoc.org/google.golang.org/grpc#ClientConn.NewStream.
type AttackerClient interface {
	Observe(ctx context.Context, opts ...grpc.CallOption) (Attacker_ObserveClient, error)
}

type attackerClient struct {
	cc *grpc.ClientConn
}

func NewAttackerClient(cc *grpc.ClientConn) AttackerClient {
	return &attackerClient{cc}
}

func (c *attackerClient) Observe(ctx context.Context, opts ...grpc.CallOption) (Attacker_ObserveClient, error) {
	stream, err := c.cc.NewStream(ctx, &_Attacker_serviceDesc.Streams[0], "/corruptible.Attacker/Observe", opts...)
	if err != nil {
		return nil, err
	}
	x := &attackerObserveClient{stream}
	return x, nil
}

type Attacker_ObserveClient interface {
	Send(*Message) error
	CloseAndRecv() (*empty.Empty, error)
	grpc.ClientStream
}

type attackerObserveClient struct {
	grpc.ClientStream
}

func (x *attackerObserveClient) Send(m *Message) error {
	return x.ClientStream.SendMsg(m)
}

func (x *attackerObserveClient) CloseAndRecv() (*empty.Empty, error) {
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	m := new(empty.Empty)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// AttackerServer is the server API for Attacker service.
type AttackerServer interface {
	Observe(Attacker_ObserveServer) error
}

// UnimplementedAttackerServer can be embedded to have forward compatible implementations.
type UnimplementedAttackerServer struct {
}

func (*UnimplementedAttackerServer) Observe(srv Attacker_ObserveServer) error {
	return status.Errorf(codes.Unimplemented, "method Observe not implemented")
}

func RegisterAttackerServer(s *grpc.Server, srv AttackerServer) {
	s.RegisterService(&_Attacker_serviceDesc, srv)
}

func _Attacker_Observe_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(AttackerServer).Observe(&attackerObserveServer{stream})
}

type Attacker_ObserveServer interface {
	SendAndClose(*empty.Empty) error
	Recv() (*Message, error)
	grpc.ServerStream
}

type attackerObserveServer struct {
	grpc.ServerStream
}

func (x *attackerObserveServer) SendAndClose(m *empty.Empty) error {
	return x.ServerStream.SendMsg(m)
}

func (x *attackerObserveServer) Recv() (*Message, error) {
	m := new(Message)
	if err := x.ServerStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

var _Attacker_serviceDesc = grpc.ServiceDesc{
	ServiceName: "corruptible.Attacker",
	HandlerType: (*AttackerServer)(nil),
	Methods:     []grpc.MethodDesc{},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "Observe",
			Handler:       _Attacker_Observe_Handler,
			ClientStreams: true,
		},
	},
	Metadata: "insecure/attacker.proto",
}
