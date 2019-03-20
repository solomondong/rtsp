package rtp

import (
	"net"

	"github.com/WUMUXIAN/rtsp/rtcp"
)

// TCPInterleavedSession defines TCPInterleavedSession body
type TCPInterleavedSession struct {
	conn     net.Conn
	RtpChan  <-chan Packet
	rtpChan  chan<- Packet
	RtcpChan <-chan rtcp.Packet
	rtcpChan chan<- rtcp.Packet
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

func (t *TCPInterleavedSession) handleConn(buf []byte) {
	// fmt.Println("Buffer Length-->", len(buf))
	// let's do the parsing here.
	start := string(buf[0])
	if start != "$" {
		// Not a RTP or RTCP packet, skip..
		// fmt.Printf("not a RTP or RTCP package, we can skip:--->\n")
		return
	}
	channel := toUint(buf[1:2])
	// length := toUint(buf[2:4])
	// fmt.Println("channel:", channel)
	// fmt.Println("length:", length)
	if channel == 0 {
		// It's a rtp package, the following content are for rtp.
		t.rtpChan <- ParsePacket(buf[4:])
	} else {
		// fmt.Println("RTCP Channel->", channel)
		// t.getRtcpPackage(buff[2:])
		t.rtcpChan <- rtcp.ParsePacket(buf[4:])
	}
}

// NewTCPInterleavedSession creates a new session over TCP interleaved
func NewTCPInterleavedSession(conn net.Conn) *TCPInterleavedSession {
	rtpChan := make(chan Packet, 10)
	rtcpChan := make(chan rtcp.Packet, 10)
	t := &TCPInterleavedSession{
		conn:     conn,
		RtpChan:  rtpChan,
		rtpChan:  rtpChan,
		RtcpChan: rtcpChan,
		rtcpChan: rtcpChan,
	}
	go t.HandleConn(conn)
	return t
}
