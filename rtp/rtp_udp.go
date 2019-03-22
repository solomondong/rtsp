package rtp

import (
	"net"

	"github.com/WUMUXIAN/rtsp/rtcp"
)

// UDPSession defines a udp session
type UDPSession struct {
	Rtp  net.Conn
	Rtcp net.Conn

	RtpChan  <-chan Packet
	RtcpChan <-chan rtcp.Packet

	rtpChan  chan<- Packet
	rtcpChan chan<- rtcp.Packet
}

// NewUDPSession creates a new UDP session
func NewUDPSession(rtpConn, rtcpConn net.Conn) *UDPSession {
	rtpChan := make(chan Packet, 10)
	rtcpChan := make(chan rtcp.Packet, 10)
	s := &UDPSession{
		Rtp:      rtpConn,
		Rtcp:     rtcpConn,
		RtpChan:  rtpChan,
		RtcpChan: rtcpChan,
		rtpChan:  rtpChan,
		rtcpChan: rtcpChan,
	}
	go s.HandleRtpConn(rtpConn)
	go s.HandleRtcpConn(rtcpConn)
	return s
}

func toUint(arr []byte) (ret uint) {
	for i, b := range arr {
		ret |= uint(b) << (8 * uint(len(arr)-i-1))
	}
	return ret
}

// HandleRtpConn handles rtp connection incomming data.
func (s *UDPSession) HandleRtpConn(conn net.Conn) {
	buf := make([]byte, 4096)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			panic(err)
		}

		cpy := make([]byte, n)
		copy(cpy, buf)
		go s.handleRtp(cpy)
	}
}

// HandleRtcpConn handles rtcp connection incomming data.
func (s *UDPSession) HandleRtcpConn(conn net.Conn) {
	buf := make([]byte, 4096)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			panic(err)
		}
		cpy := make([]byte, n)
		copy(cpy, buf)
		go s.handleRtcp(cpy)
	}
}

func (s *UDPSession) handleRtp(buf []byte) {
	s.rtpChan <- ParsePacket(buf, 0)
}

func (s *UDPSession) handleRtcp(buf []byte) {
	// TODO: implement rtcp
	s.rtcpChan <- rtcp.ParsePacket(buf)
}
