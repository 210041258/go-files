// Package packet provides packet capture, decoding, and manipulation.
// By default, it uses a pure Go decoder for common protocols with no CGO
// and no external dependencies. For live capture and advanced features,
// use the "pcap" build tag to enable gopacket integration.
//
// All core types implement fmt.Stringer and are comparable where possible.
package testutils

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"time"

)

// --------------------------------------------------------------------
// Packet and Metadata
// --------------------------------------------------------------------

// Packet represents a raw network packet with capture metadata.
type Packet struct {
	// Data is the raw packet bytes (including link layer).
	Data []byte
	// Timestamp is the capture time (zero if unknown).
	Timestamp time.Time
	// Length is the original capture length (may be larger than len(Data)).
	Length int
	// LinkType is the link layer protocol (e.g., Ethernet, LinuxSLL).
	LinkType LinkType
}

// LinkType identifies the link layer protocol.
type LinkType int

const (
	LinkTypeNull     LinkType = 0
	LinkTypeEthernet LinkType = 1
	LinkTypeLoop     LinkType = 108
	LinkTypeLinuxSLL LinkType = 113
	LinkTypeRaw      LinkType = 101
)

// String returns a human-readable link type.
func (lt LinkType) String() string {
	switch lt {
	case LinkTypeNull:
		return "Null"
	case LinkTypeEthernet:
		return "Ethernet"
	case LinkTypeLoop:
		return "Loopback"
	case LinkTypeLinuxSLL:
		return "Linux cooked"
	case LinkTypeRaw:
		return "Raw IP"
	default:
		return fmt.Sprintf("LinkType(%d)", lt)
	}
}

// --------------------------------------------------------------------
// Ethernet Header
// --------------------------------------------------------------------

// EthernetAddr is a 6-byte MAC address.
type EthernetAddr [6]byte

// String returns the MAC address in colon-hex format.
func (a EthernetAddr) String() string {
	return fmt.Sprintf("%02x:%02x:%02x:%02x:%02x:%02x",
		a[0], a[1], a[2], a[3], a[4], a[5])
}

// IsZero reports whether the address is all zeros.
func (a EthernetAddr) IsZero() bool {
	return a == EthernetAddr{}
}

// Broadcast returns true if this is the broadcast address (ff:ff:ff:ff:ff:ff).
func (a EthernetAddr) Broadcast() bool {
	return a == EthernetBroadcast
}

// Multicast returns true if the address is a multicast MAC.
func (a EthernetAddr) Multicast() bool {
	return a[0]&1 == 1
}

// EthernetBroadcast is the broadcast MAC address.
var EthernetBroadcast = EthernetAddr{0xff, 0xff, 0xff, 0xff, 0xff, 0xff}

// EthernetHeader is a 14-byte Ethernet II frame header.
type EthernetHeader struct {
	DstAddr   EthernetAddr
	SrcAddr   EthernetAddr
	EtherType uint16
}

// DecodeEthernet decodes an Ethernet header from the beginning of data.
// Returns the header, the payload, and an error if decoding fails.
func DecodeEthernet(data []byte) (hdr EthernetHeader, payload []byte, err error) {
	if len(data) < 14 {
		return EthernetHeader{}, nil, ErrPacketTooShort
	}
	copy(hdr.DstAddr[:], data[0:6])
	copy(hdr.SrcAddr[:], data[6:12])
	hdr.EtherType = binary.BigEndian.Uint16(data[12:14])
	return hdr, data[14:], nil
}

// Encode serializes the Ethernet header into a byte slice.
func (h *EthernetHeader) Encode() []byte {
	b := make([]byte, 14)
	copy(b[0:6], h.DstAddr[:])
	copy(b[6:12], h.SrcAddr[:])
	binary.BigEndian.PutUint16(b[12:14], h.EtherType)
	return b
}

// --------------------------------------------------------------------
// IPv4 Header
// --------------------------------------------------------------------

// IPv4Header represents an IPv4 packet header (minimal, no options).
type IPv4Header struct {
	Version     uint8 // always 4
	IHL         uint8 // header length in 32-bit words
	TOS         uint8
	TotalLength uint16
	ID          uint16
	Flags       uint8
	FragmentOff uint16
	TTL         uint8
	Protocol    uint8
	Checksum    uint16
	SrcIP       net.IP
	DstIP       net.IP
}

// DecodeIPv4 decodes an IPv4 header from the beginning of data.
// Returns the header, the payload, and an error if decoding fails.
func DecodeIPv4(data []byte) (hdr IPv4Header, payload []byte, err error) {
	if len(data) < 20 {
		return IPv4Header{}, nil, ErrPacketTooShort
	}
	versionIHL := data[0]
	hdr.Version = versionIHL >> 4
	hdr.IHL = versionIHL & 0x0F
	if hdr.Version != 4 {
		return IPv4Header{}, nil, ErrInvalidVersion
	}
	ihlBytes := int(hdr.IHL) * 4
	if len(data) < ihlBytes {
		return IPv4Header{}, nil, ErrPacketTooShort
	}
	hdr.TOS = data[1]
	hdr.TotalLength = binary.BigEndian.Uint16(data[2:4])
	hdr.ID = binary.BigEndian.Uint16(data[4:6])
	flagsFrag := binary.BigEndian.Uint16(data[6:8])
	hdr.Flags = uint8(flagsFrag >> 13)
	hdr.FragmentOff = flagsFrag & 0x1FFF
	hdr.TTL = data[8]
	hdr.Protocol = data[9]
	hdr.Checksum = binary.BigEndian.Uint16(data[10:12])
	hdr.SrcIP = net.IP(data[12:16])
	hdr.DstIP = net.IP(data[16:20])
	return hdr, data[ihlBytes:], nil
}

// Encode serializes the IPv4 header into a byte slice.
// Recomputes the checksum automatically.
func (h *IPv4Header) Encode() []byte {
	h.Version = 4
	ihl := 5 // no options
	length := 20
	b := make([]byte, length)
	b[0] = (h.Version << 4) | uint8(ihl)
	b[1] = h.TOS
	binary.BigEndian.PutUint16(b[2:4], h.TotalLength)
	binary.BigEndian.PutUint16(b[4:6], h.ID)
	frag := (uint16(h.Flags) << 13) | (h.FragmentOff & 0x1FFF)
	binary.BigEndian.PutUint16(b[6:8], frag)
	b[8] = h.TTL
	b[9] = h.Protocol
	// checksum field zero for calculation
	binary.BigEndian.PutUint16(b[10:12], 0)
	copy(b[12:16], h.SrcIP.To4())
	copy(b[16:20], h.DstIP.To4())
	// compute checksum
	checksum := ipChecksum(b[:20])
	binary.BigEndian.PutUint16(b[10:12], checksum)
	return b
}

// ipChecksum computes the RFC 1071 checksum.
func ipChecksum(data []byte) uint16 {
	var sum uint32
	for i := 0; i < len(data); i += 2 {
		if i+1 >= len(data) {
			sum += uint32(data[i]) << 8
		} else {
			sum += uint32(data[i])<<8 | uint32(data[i+1])
		}
	}
	for (sum >> 16) > 0 {
		sum = (sum & 0xFFFF) + (sum >> 16)
	}
	return ^uint16(sum)
}

// --------------------------------------------------------------------
// IPv6 Header
// --------------------------------------------------------------------

// IPv6Header represents an IPv6 packet header (no extension headers).
type IPv6Header struct {
	Version       uint8 // always 6
	TrafficClass  uint8
	FlowLabel     uint32 // lower 20 bits
	PayloadLength uint16
	NextHeader    uint8
	HopLimit      uint8
	SrcIP         net.IP
	DstIP         net.IP
}

// DecodeIPv6 decodes an IPv6 header from the beginning of data.
func DecodeIPv6(data []byte) (hdr IPv6Header, payload []byte, err error) {
	if len(data) < 40 {
		return IPv6Header{}, nil, ErrPacketTooShort
	}
	vcf := binary.BigEndian.Uint32(data[0:4])
	hdr.Version = uint8(vcf >> 28)
	if hdr.Version != 6 {
		return IPv6Header{}, nil, ErrInvalidVersion
	}
	hdr.TrafficClass = uint8((vcf >> 20) & 0xFF)
	hdr.FlowLabel = vcf & 0xFFFFF
	hdr.PayloadLength = binary.BigEndian.Uint16(data[4:6])
	hdr.NextHeader = data[6]
	hdr.HopLimit = data[7]
	hdr.SrcIP = net.IP(data[8:24])
	hdr.DstIP = net.IP(data[24:40])
	return hdr, data[40:], nil
}

// --------------------------------------------------------------------
// TCP Header
// --------------------------------------------------------------------

// TCPHeader represents a TCP header (minimal, no options).
type TCPHeader struct {
	SrcPort  uint16
	DstPort  uint16
	SeqNum   uint32
	AckNum   uint32
	DataOff  uint8 // 4 bits, header length in 32-bit words
	Flags    uint8
	Window   uint16
	Checksum uint16
	Urgent   uint16
}

const (
	TCPFlagFIN = 0x01
	TCPFlagSYN = 0x02
	TCPFlagRST = 0x04
	TCPFlagPSH = 0x08
	TCPFlagACK = 0x10
	TCPFlagURG = 0x20
	TCPFlagECE = 0x40
	TCPFlagCWR = 0x80
)

// DecodeTCP decodes a TCP header from the beginning of data.
func DecodeTCP(data []byte) (hdr TCPHeader, payload []byte, err error) {
	if len(data) < 20 {
		return TCPHeader{}, nil, ErrPacketTooShort
	}
	hdr.SrcPort = binary.BigEndian.Uint16(data[0:2])
	hdr.DstPort = binary.BigEndian.Uint16(data[2:4])
	hdr.SeqNum = binary.BigEndian.Uint32(data[4:8])
	hdr.AckNum = binary.BigEndian.Uint32(data[8:12])
	dataOffFlags := binary.BigEndian.Uint16(data[12:14])
	hdr.DataOff = uint8(dataOffFlags >> 12)
	hdr.Flags = uint8(dataOffFlags & 0x0FFF)
	hdr.Window = binary.BigEndian.Uint16(data[14:16])
	hdr.Checksum = binary.BigEndian.Uint16(data[16:18])
	hdr.Urgent = binary.BigEndian.Uint16(data[18:20])
	ihlBytes := int(hdr.DataOff) * 4
	if len(data) < ihlBytes {
		return TCPHeader{}, nil, ErrPacketTooShort
	}
	return hdr, data[ihlBytes:], nil
}

// --------------------------------------------------------------------
// UDP Header
// --------------------------------------------------------------------

// UDPHeader represents a UDP header.
type UDPHeader struct {
	SrcPort  uint16
	DstPort  uint16
	Length   uint16
	Checksum uint16
}

// DecodeUDP decodes a UDP header from the beginning of data.
func DecodeUDP(data []byte) (hdr UDPHeader, payload []byte, err error) {
	if len(data) < 8 {
		return UDPHeader{}, nil, ErrPacketTooShort
	}
	hdr.SrcPort = binary.BigEndian.Uint16(data[0:2])
	hdr.DstPort = binary.BigEndian.Uint16(data[2:4])
	hdr.Length = binary.BigEndian.Uint16(data[4:6])
	hdr.Checksum = binary.BigEndian.Uint16(data[6:8])
	return hdr, data[8:], nil
}

// --------------------------------------------------------------------
// ARP Header
// --------------------------------------------------------------------

// ARPHeader represents an ARP packet.
type ARPHeader struct {
	HTYPE    uint16 // hardware type (1 = Ethernet)
	PTYPE    uint16 // protocol type (0x0800 = IPv4)
	HLEN     uint8  // hardware address length (6 for Ethernet)
	PLEN     uint8  // protocol address length (4 for IPv4)
	OPER     uint16 // operation (1 = request, 2 = reply)
	SenderHA EthernetAddr
	SenderIP net.IP
	TargetHA EthernetAddr
	TargetIP net.IP
}

// DecodeARP decodes an ARP packet.
func DecodeARP(data []byte) (hdr ARPHeader, err error) {
	if len(data) < 28 {
		return ARPHeader{}, ErrPacketTooShort
	}
	hdr.HTYPE = binary.BigEndian.Uint16(data[0:2])
	hdr.PTYPE = binary.BigEndian.Uint16(data[2:4])
	hdr.HLEN = data[4]
	hdr.PLEN = data[5]
	hdr.OPER = binary.BigEndian.Uint16(data[6:8])
	if hdr.HLEN == 6 && hdr.PLEN == 4 {
		copy(hdr.SenderHA[:], data[8:14])
		hdr.SenderIP = net.IP(data[14:18])
		copy(hdr.TargetHA[:], data[18:24])
		hdr.TargetIP = net.IP(data[24:28])
	}
	return hdr, nil
}

// --------------------------------------------------------------------
// Flow and Endpoint (adopted from gopacket's excellent design)
// --------------------------------------------------------------------

// Endpoint represents a communication endpoint (IP, port, MAC).
type Endpoint struct {
	Type EndpointType
	Data []byte // raw bytes, canonical representation
}

// EndpointType identifies the type of endpoint.
type EndpointType uint8

const (
	EndpointInvalid EndpointType = iota
	EndpointMAC
	EndpointIPv4
	EndpointIPv6
	EndpointTCPPort
	EndpointUDPPort
	EndpointPort // generic port
)

// NewEndpoint creates an endpoint from raw bytes.
func NewEndpoint(typ EndpointType, data []byte) Endpoint {
	c := make([]byte, len(data))
	copy(c, data)
	return Endpoint{Type: typ, Data: c}
}

// NewMACEndpoint creates an Ethernet MAC endpoint.
func NewMACEndpoint(addr EthernetAddr) Endpoint {
	return NewEndpoint(EndpointMAC, addr[:])
}

// NewIPv4Endpoint creates an IPv4 address endpoint.
func NewIPv4Endpoint(ip net.IP) Endpoint {
	return NewEndpoint(EndpointIPv4, ip.To4())
}

// NewIPv6Endpoint creates an IPv6 address endpoint.
func NewIPv6Endpoint(ip net.IP) Endpoint {
	return NewEndpoint(EndpointIPv6, ip.To16())
}

// NewTCPPortEndpoint creates a TCP port endpoint.
func NewTCPPortEndpoint(port uint16) Endpoint {
	b := make([]byte, 2)
	binary.BigEndian.PutUint16(b, port)
	return NewEndpoint(EndpointTCPPort, b)
}

// String returns a human-readable endpoint.
func (e Endpoint) String() string {
	switch e.Type {
	case EndpointMAC:
		var mac EthernetAddr
		if len(e.Data) == 6 {
			copy(mac[:], e.Data)
			return mac.String()
		}
	case EndpointIPv4:
		return net.IP(e.Data).String()
	case EndpointIPv6:
		return net.IP(e.Data).String()
	case EndpointTCPPort, EndpointUDPPort, EndpointPort:
		if len(e.Data) == 2 {
			return fmt.Sprintf("%d", binary.BigEndian.Uint16(e.Data))
		}
	}
	return fmt.Sprintf("unknown(%x)", e.Data)
}

// Flow represents a bidirectional flow between two endpoints.
type Flow struct {
	Src Endpoint
	Dst Endpoint
}

// NewFlow creates a flow from source and destination endpoints.
func NewFlow(src, dst Endpoint) Flow {
	return Flow{Src: src, Dst: dst}
}

// Reverse returns a flow with source and destination swapped.
func (f Flow) Reverse() Flow {
	return Flow{Src: f.Dst, Dst: f.Src}
}

// FastHash returns a symmetric hash (same for A->B and B->A).
func (f Flow) FastHash() uint64 {
	// XOR of both endpoint hashes
	h1 := hashEndpoint(f.Src)
	h2 := hashEndpoint(f.Dst)
	return h1 ^ h2
}

// hashEndpoint computes a 64-bit hash of endpoint bytes.
func hashEndpoint(e Endpoint) uint64 {
	var h uint64
	for _, b := range e.Data {
		h = h*131 + uint64(b)
	}
	return h
}

// --------------------------------------------------------------------
// Errors
// --------------------------------------------------------------------

var (
	ErrPacketTooShort  = errors.New("packet: data too short")
	ErrInvalidVersion  = errors.New("packet: invalid IP version")
	ErrUnsupportedLink = errors.New("packet: unsupported link type")
	ErrDecoding        = errors.New("packet: decoding failed")
)

// --------------------------------------------------------------------
// Integration with value.Option
// --------------------------------------------------------------------

// DecodeEthernetOpt returns an Option containing the header if successful.
func DecodeEthernetOpt(data []byte) value.Option[EthernetHeader] {
	h, _, err := DecodeEthernet(data)
	if err != nil {
		return value.None[EthernetHeader]()
	}
	return value.Some(h)
}

// DecodeIPv4Opt returns an Option containing the IPv4 header.
func DecodeIPv4Opt(data []byte) value.Option[IPv4Header] {
	h, _, err := DecodeIPv4(data)
	if err != nil {
		return value.None[IPv4Header]()
	}
	return value.Some(h)
}

// DecodeTCPOpt returns an Option containing the TCP header.
func DecodeTCPOpt(data []byte) value.Option[TCPHeader] {
	h, _, err := DecodeTCP(data)
	if err != nil {
		return value.None[TCPHeader]()
	}
	return value.Some(h)
}
