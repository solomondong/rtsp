package rtp

import (
	"bufio"
	"fmt"
	"io"
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
	// Handle connection, what we do is to read the RTP/RTCP packets one by one.
	reader := bufio.NewReader(conn)
	go t.readPackets(reader)
}

func (t *TCPInterleavedSession) readPackets(reader io.Reader) {
	for {
		header := make([]byte, 4)

		if l, err := io.ReadFull(reader, header); err != nil || l != 4 || header[0] != '$' {
			fmt.Println("err, read header not correct", err, l, header)
			return
		}
		length := toUint(header[2:4])
		channel := toUint(header[1:2])

		// fmt.Println("length:", length, "channel:", channel)

		// ready length of data.
		data := make([]byte, length)

		if l, err := io.ReadFull(reader, data); err != nil || l != int(length) {
			fmt.Println("err, read data not correct", err, l, length)
			return
		}

		// this means it's RTP
		if channel%2 == 0 {
			t.rtpChan <- ParsePacket(data)
		} else {
			// this means it's RTCP
			t.rtcpChan <- rtcp.ParsePacket(data)
		}
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
