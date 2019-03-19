package rtp

import (
	"fmt"
	"net"
)

// TCPInterleavedSession defines TCPInterleavedSession body
type TCPInterleavedSession struct {
	conn    net.Conn
	RtpChan <-chan RtpPacket
	rtpChan chan<- RtpPacket
}

// HandleConn handles the interleaved TCP connection
func (t *TCPInterleavedSession) HandleConn(conn net.Conn) {
	buf := make([]byte, 4096)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			panic(err)
		}
		cpy := make([]byte, n)
		copy(cpy, buf)
		go t.handleConn(cpy)
	}
}

func (t *TCPInterleavedSession) getRtpPacket(buf []byte) {
	packet := RtpPacket{
		Version:        buf[0] >> 6,
		Padding:        buf[0]>>5&1 != 0,
		Ext:            buf[0]>>4&1 != 0,
		CSRC:           make([]uint, buf[0]&0x0f),
		Marker:         buf[1]>>7 != 0,
		PayloadType:    buf[1] & 0x7f,
		SequenceNumber: toUint(buf[2:4]),
		Timestamp:      toUint(buf[4:8]),
		SyncSource:     toUint(buf[8:12]),
	}
	if packet.Version != RTPVERSION {
		panic("Unsupported version")
	}

	i := 12

	for j := range packet.CSRC {
		packet.CSRC[j] = toUint(buf[i : i+4])
		i += 4
	}

	if packet.Ext {
		packet.ExtHeader = toUint(buf[i : i+2])
		length := toUint(buf[i+2 : i+4])
		i += 4
		if length > 0 {
			packet.ExtData = buf[i : i+int(length)*4]
			i += int(length) * 4
		}
	}

	packet.Payload = buf[i:]

	t.rtpChan <- packet
}

func (t *TCPInterleavedSession) handleConn(buf []byte) {
	// fmt.Println("Buffer Length-->", len(buf))
	// let's do the parsing here.
	start := string(buf[0])
	if start != "$" {
		// fmt.Printf("not a RTP or RTCP package, we can skip:--->\n")
		return
	}

	channel := toUint(buf[1:2])
	length := toUint(buf[2:4])
	fmt.Println("channel:", channel)
	fmt.Println("length:", length)
	if channel == 0 {
		// It's a rtp package.
		// fmt.Printf("length of byte data: %d: %b\n", len(buf[4:]), buf[4:])
		// fmt.Printf("length of byte data: %d: %b\n", len(buf[4:4+length]), buf[4:4+length])
		// fmt.Printf("the rest is the real RTP data: %d: %b\n", len(buf[4+length:]), buf[4+length:])
		t.getRtpPacket(buf[4:])
	} else {
		fmt.Println("RTCP Channel->", channel)
		// t.getRtcpPackage(buff[2:])
	}
}

func NewTCPInterleavedSession(conn net.Conn) *TCPInterleavedSession {
	rtpChan := make(chan RtpPacket, 10)
	t := &TCPInterleavedSession{
		conn:    conn,
		RtpChan: rtpChan,
		rtpChan: rtpChan,
	}
	go t.HandleConn(conn)
	return t
}
