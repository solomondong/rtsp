package rtp

import "fmt"

// some consts
const (
	RTPVERSION    = 2
	hasRtpPadding = 1 << 2
	hasRtpExt     = 1 << 3
)

// Packet defines a rtp packet
// Packet as per https://tools.ietf.org/html/rfc1889#section-5.1
//
//  0                   1                   2                   3
//  0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
// |V=2|P|X|  CC   |M|     PT      |       sequence number         |
// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
// |                           timestamp                           |
// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
// |           synchronization source (SSRC) identifier            |
// +=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+
// |            contributing source (CSRC) identifiers             |
// |                             ....                              |
// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
type Packet struct {
	Version        byte
	Padding        bool
	Ext            bool
	Marker         bool
	PayloadType    byte
	SequenceNumber uint
	Timestamp      uint
	SyncSource     uint

	CSRC []uint

	ExtHeader uint
	ExtData   []byte

	Payload []byte
}

// Strings prints a rtp packet content
func (r Packet) String() string {
	return fmt.Sprintf("RTP Packet: \nVersion:%b---Padding:%v---Ext:%v---Marker:%v--PayloadType:%v\n"+
		"SequenceNumber:%v---Timestamp---%v---SyncSource---%v",
		r.Version, r.Padding, r.Ext, r.Marker, r.PayloadType,
		r.SequenceNumber, r.Timestamp, r.SyncSource)
}

// ParsePacket parses data frame into RTP packet
func ParsePacket(buf []byte) Packet {
	packet := Packet{
		Version:        buf[0] >> 6,
		Padding:        buf[0]>>5&1 != 0,
		Ext:            buf[0]>>4&1 != 0,
		CSRC:           make([]uint, int(buf[0]&0x0f)),
		Marker:         buf[1]>>7 != 0,
		PayloadType:    buf[1] & 0x7f,
		SequenceNumber: toUint(buf[2:4]),
		Timestamp:      toUint(buf[4:8]),
		SyncSource:     toUint(buf[8:12]),
	}
	if packet.Version != RTPVERSION {
		panic("Unsupported version")
	}

	// Next section is the CSRC identifiers, each identifier has 4 bytes
	i := 12
	for j := range packet.CSRC {
		packet.CSRC[j] = toUint(buf[i : i+4])
		i += 4
	}

	// If we have extra header, the following section is it.
	if packet.Ext {
		packet.ExtHeader = toUint(buf[i : i+2])
		length := toUint(buf[i+2 : i+4])
		i += 4
		// if length is > 0, we have ext data, each section is 4 bytes.
		if length > 0 {
			packet.ExtData = buf[i : i+int(length)*4]
			i += int(length) * 4
		}
	}
	// The rest of the packet is the pay load.
	packet.Payload = buf[i:]
	return packet
}
