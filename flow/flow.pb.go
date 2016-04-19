// Code generated by protoc-gen-go.
// source: flow/flow.proto
// DO NOT EDIT!

/*
Package flow is a generated protocol buffer package.

It is generated from these files:
	flow/flow.proto

It has these top-level messages:
	FlowEndpointStatistics
	FlowEndpointsStatistics
	FlowStatistics
	Flow
*/
package flow

import proto "github.com/golang/protobuf/proto"
import fmt "fmt"
import math "math"

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
const _ = proto.ProtoPackageIsVersion1

type FlowEndpointLayer int32

const (
	FlowEndpointLayer_LINK      FlowEndpointLayer = 0
	FlowEndpointLayer_NETWORK   FlowEndpointLayer = 1
	FlowEndpointLayer_TRANSPORT FlowEndpointLayer = 2
)

var FlowEndpointLayer_name = map[int32]string{
	0: "LINK",
	1: "NETWORK",
	2: "TRANSPORT",
}
var FlowEndpointLayer_value = map[string]int32{
	"LINK":      0,
	"NETWORK":   1,
	"TRANSPORT": 2,
}

func (x FlowEndpointLayer) String() string {
	return proto.EnumName(FlowEndpointLayer_name, int32(x))
}
func (FlowEndpointLayer) EnumDescriptor() ([]byte, []int) { return fileDescriptor0, []int{0} }

type FlowEndpointType int32

const (
	FlowEndpointType_ETHERNET FlowEndpointType = 0
	FlowEndpointType_IPV4     FlowEndpointType = 1
	FlowEndpointType_TCPPORT  FlowEndpointType = 2
	FlowEndpointType_UDPPORT  FlowEndpointType = 3
	FlowEndpointType_SCTPPORT FlowEndpointType = 4
)

var FlowEndpointType_name = map[int32]string{
	0: "ETHERNET",
	1: "IPV4",
	2: "TCPPORT",
	3: "UDPPORT",
	4: "SCTPPORT",
}
var FlowEndpointType_value = map[string]int32{
	"ETHERNET": 0,
	"IPV4":     1,
	"TCPPORT":  2,
	"UDPPORT":  3,
	"SCTPPORT": 4,
}

func (x FlowEndpointType) String() string {
	return proto.EnumName(FlowEndpointType_name, int32(x))
}
func (FlowEndpointType) EnumDescriptor() ([]byte, []int) { return fileDescriptor0, []int{1} }

type FlowEndpointStatistics struct {
	Value   string `protobuf:"bytes,2,opt,name=Value" json:"Value,omitempty"`
	Packets uint64 `protobuf:"varint,5,opt,name=Packets" json:"Packets,omitempty"`
	Bytes   uint64 `protobuf:"varint,6,opt,name=Bytes" json:"Bytes,omitempty"`
}

func (m *FlowEndpointStatistics) Reset()                    { *m = FlowEndpointStatistics{} }
func (m *FlowEndpointStatistics) String() string            { return proto.CompactTextString(m) }
func (*FlowEndpointStatistics) ProtoMessage()               {}
func (*FlowEndpointStatistics) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{0} }

type FlowEndpointsStatistics struct {
	Type FlowEndpointType        `protobuf:"varint,1,opt,name=Type,enum=flow.FlowEndpointType" json:"Type,omitempty"`
	AB   *FlowEndpointStatistics `protobuf:"bytes,3,opt,name=AB" json:"AB,omitempty"`
	BA   *FlowEndpointStatistics `protobuf:"bytes,4,opt,name=BA" json:"BA,omitempty"`
}

func (m *FlowEndpointsStatistics) Reset()                    { *m = FlowEndpointsStatistics{} }
func (m *FlowEndpointsStatistics) String() string            { return proto.CompactTextString(m) }
func (*FlowEndpointsStatistics) ProtoMessage()               {}
func (*FlowEndpointsStatistics) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{1} }

func (m *FlowEndpointsStatistics) GetAB() *FlowEndpointStatistics {
	if m != nil {
		return m.AB
	}
	return nil
}

func (m *FlowEndpointsStatistics) GetBA() *FlowEndpointStatistics {
	if m != nil {
		return m.BA
	}
	return nil
}

type FlowStatistics struct {
	Start     int64                      `protobuf:"varint,1,opt,name=Start" json:"Start,omitempty"`
	Last      int64                      `protobuf:"varint,2,opt,name=Last" json:"Last,omitempty"`
	Endpoints []*FlowEndpointsStatistics `protobuf:"bytes,3,rep,name=Endpoints" json:"Endpoints,omitempty"`
}

func (m *FlowStatistics) Reset()                    { *m = FlowStatistics{} }
func (m *FlowStatistics) String() string            { return proto.CompactTextString(m) }
func (*FlowStatistics) ProtoMessage()               {}
func (*FlowStatistics) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{2} }

func (m *FlowStatistics) GetEndpoints() []*FlowEndpointsStatistics {
	if m != nil {
		return m.Endpoints
	}
	return nil
}

type Flow struct {
	UUID       string `protobuf:"bytes,1,opt,name=UUID" json:"UUID,omitempty"`
	LayersPath string `protobuf:"bytes,2,opt,name=LayersPath" json:"LayersPath,omitempty"`
	// Data Flow info
	Statistics *FlowStatistics `protobuf:"bytes,3,opt,name=Statistics" json:"Statistics,omitempty"`
	FlowUUID   string          `protobuf:"bytes,5,opt,name=FlowUUID" json:"FlowUUID,omitempty"`
	// Topology info
	ProbeGraphPath string `protobuf:"bytes,11,opt,name=ProbeGraphPath" json:"ProbeGraphPath,omitempty"`
	IfSrcGraphPath string `protobuf:"bytes,14,opt,name=IfSrcGraphPath" json:"IfSrcGraphPath,omitempty"`
	IfDstGraphPath string `protobuf:"bytes,19,opt,name=IfDstGraphPath" json:"IfDstGraphPath,omitempty"`
}

func (m *Flow) Reset()                    { *m = Flow{} }
func (m *Flow) String() string            { return proto.CompactTextString(m) }
func (*Flow) ProtoMessage()               {}
func (*Flow) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{3} }

func (m *Flow) GetStatistics() *FlowStatistics {
	if m != nil {
		return m.Statistics
	}
	return nil
}

func init() {
	proto.RegisterType((*FlowEndpointStatistics)(nil), "flow.FlowEndpointStatistics")
	proto.RegisterType((*FlowEndpointsStatistics)(nil), "flow.FlowEndpointsStatistics")
	proto.RegisterType((*FlowStatistics)(nil), "flow.FlowStatistics")
	proto.RegisterType((*Flow)(nil), "flow.Flow")
	proto.RegisterEnum("flow.FlowEndpointLayer", FlowEndpointLayer_name, FlowEndpointLayer_value)
	proto.RegisterEnum("flow.FlowEndpointType", FlowEndpointType_name, FlowEndpointType_value)
}

var fileDescriptor0 = []byte{
	// 448 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x09, 0x6e, 0x88, 0x02, 0xff, 0x8c, 0x93, 0x41, 0x6f, 0x94, 0x40,
	0x14, 0xc7, 0x5d, 0x18, 0xba, 0xcb, 0x5b, 0x45, 0x1c, 0x9b, 0x4a, 0x8c, 0x1a, 0xc3, 0xc1, 0x98,
	0x8d, 0xa9, 0x49, 0xed, 0xc5, 0x78, 0x82, 0x2e, 0x2a, 0x69, 0xb3, 0x25, 0x03, 0x5b, 0x6f, 0x26,
	0xc3, 0xca, 0xa6, 0x44, 0x52, 0x08, 0x33, 0xb5, 0xd9, 0xbb, 0x5f, 0xc4, 0x6f, 0xea, 0x9b, 0x61,
	0x5b, 0x58, 0x7b, 0xf1, 0x32, 0xcc, 0xff, 0xcd, 0xef, 0xbd, 0xff, 0xbc, 0x07, 0xc0, 0xe3, 0x75,
	0x55, 0xdf, 0xbc, 0x57, 0xcb, 0x61, 0xd3, 0xd6, 0xb2, 0xa6, 0x44, 0xed, 0xfd, 0xef, 0x70, 0xf0,
	0x19, 0x9f, 0xd1, 0xd5, 0x8f, 0xa6, 0x2e, 0xaf, 0x64, 0x2a, 0xb9, 0x2c, 0x85, 0x2c, 0x57, 0x82,
	0xee, 0x83, 0x75, 0xc1, 0xab, 0xeb, 0xc2, 0x33, 0x5e, 0x8f, 0xde, 0xda, 0xcc, 0xfa, 0xa5, 0x04,
	0xf5, 0x60, 0x9c, 0xf0, 0xd5, 0xcf, 0x42, 0x0a, 0xcf, 0xc2, 0x38, 0x61, 0xe3, 0xa6, 0x93, 0x8a,
	0x0f, 0x37, 0xb2, 0x10, 0xde, 0x9e, 0x8e, 0x5b, 0xb9, 0x12, 0xfe, 0x9f, 0x11, 0x3c, 0x1b, 0x1a,
	0x88, 0x81, 0xc3, 0x0c, 0x48, 0xb6, 0x69, 0x0a, 0x6f, 0x84, 0x09, 0xce, 0xd1, 0xc1, 0xa1, 0xbe,
	0xdc, 0x10, 0x56, 0xa7, 0x8c, 0x48, 0x5c, 0xe9, 0x3b, 0x30, 0x82, 0xd0, 0x33, 0x91, 0x9c, 0x1e,
	0xbd, 0xb8, 0x4f, 0xf6, 0x55, 0x99, 0xc1, 0x43, 0x45, 0x87, 0x81, 0x47, 0xfe, 0x87, 0xce, 0x03,
	0xff, 0x06, 0x1c, 0x75, 0xba, 0xdb, 0x3b, 0xaa, 0x56, 0xea, 0xab, 0x99, 0xcc, 0x12, 0x4a, 0x50,
	0x0a, 0xe4, 0x8c, 0x0b, 0xa9, 0x07, 0x62, 0x32, 0x52, 0xe1, 0x9e, 0x7e, 0x02, 0xfb, 0xae, 0x35,
	0xbc, 0x9e, 0x89, 0x86, 0x2f, 0xef, 0x1b, 0x0e, 0xba, 0x66, 0x76, 0x71, 0x1b, 0xf4, 0x7f, 0x1b,
	0x40, 0x14, 0xa6, 0x2a, 0x2f, 0x97, 0xf1, 0x5c, 0xdb, 0xd9, 0x8c, 0x5c, 0xe3, 0x9e, 0xbe, 0x02,
	0x38, 0xe3, 0x9b, 0xa2, 0x15, 0x09, 0x97, 0x97, 0xdb, 0x97, 0x00, 0xd5, 0x5d, 0x84, 0x1e, 0x03,
	0xf4, 0x55, 0xb7, 0x93, 0xd9, 0xef, 0xad, 0x07, 0x8e, 0x20, 0xfa, 0xce, 0x9e, 0xc3, 0x44, 0x9d,
	0x6a, 0x37, 0x4b, 0xd7, 0x9c, 0xac, 0xb7, 0x9a, 0xbe, 0x01, 0x27, 0x69, 0xeb, 0xbc, 0xf8, 0xd2,
	0xf2, 0xe6, 0x52, 0xbb, 0x4e, 0x35, 0xe1, 0x34, 0x3b, 0x51, 0xc5, 0xc5, 0xeb, 0xb4, 0x5d, 0xf5,
	0x9c, 0xd3, 0x71, 0xe5, 0x4e, 0xb4, 0xe3, 0xe6, 0x42, 0xf6, 0xdc, 0xd3, 0x5b, 0x6e, 0x18, 0x9d,
	0x7d, 0x84, 0x27, 0xc3, 0x61, 0xe9, 0xae, 0xe9, 0x04, 0x87, 0x1d, 0x2f, 0x4e, 0xdd, 0x07, 0x74,
	0x0a, 0xe3, 0x45, 0x94, 0x7d, 0x3b, 0x67, 0xa7, 0xee, 0x88, 0x3e, 0x02, 0x3b, 0x63, 0xc1, 0x22,
	0x4d, 0xce, 0x59, 0xe6, 0x1a, 0x33, 0x06, 0xee, 0xbf, 0x1f, 0x0c, 0x7d, 0x08, 0x93, 0x28, 0xfb,
	0x1a, 0x31, 0x4c, 0xc2, 0x6c, 0xac, 0x13, 0x27, 0x17, 0xc7, 0x98, 0x8a, 0x75, 0xb2, 0x93, 0xa4,
	0x4b, 0x54, 0x62, 0x39, 0xef, 0x84, 0xa9, 0x32, 0xd2, 0x93, 0xac, 0x53, 0x24, 0xdf, 0xd3, 0xff,
	0xc7, 0x87, 0xbf, 0x01, 0x00, 0x00, 0xff, 0xff, 0x01, 0x1e, 0xfe, 0x56, 0x32, 0x03, 0x00, 0x00,
}
