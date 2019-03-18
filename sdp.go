package rtsp

import (
	"bufio"
	"errors"
	"io"
	"strconv"
	"strings"
)

// SessionSectionOriginator defines SessionSectionOriginator body
type SessionSectionOriginator struct {
	UserName    string
	SessionID   string
	Version     string
	NetworkType string
	AddressType string
	Address     string
}

// SessionSectionConnectionInformation defines SessionSectionConnectionInformation body
type SessionSectionConnectionInformation struct {
	NetworkType string
	AddressType string
	Address     string
}

// SessionSectionTime defines SessionSectionTime body
type SessionSectionTime struct {
	StartTime string
	EndTime   string
}

// SessionSectionRepeat defines SessionSectionRepeat body
type SessionSectionRepeat struct {
	StartTime string
	EndTime   string
}

// SessionSection defines SessionSection body
type SessionSection struct {
	Version               int
	Originator            SessionSectionOriginator
	SessionName           string
	SessionInformation    string
	URI                   string
	Emails                []string
	Phones                []string
	ConnectionInformation SessionSectionConnectionInformation
	BandwidthInformation  []string
	TimeZone              string
	EncryptionKey         string
	Time                  []SessionSectionTime
	Repeat                []string
}

// v=0
// o=- 2251938202 2251938202 IN IP4 0.0.0.0
// s=Media Server
// c=IN IP4 0.0.0.0
// t=0 0
// a=control:*
// a=packetization-supported:DH
// a=rtppayload-supported:DH
// a=range:npt=now-
// m=video 0 RTP/AVP 96
// a=control:trackID=0
// a=framerate:25.000000
// a=rtpmap:96 H264/90000
// a=fmtp:96 packetization-mode=1;profile-level-id=64002A;sprop-parameter-sets=Z2QAKqwsaoHgCJ+WbgICAgQA,aO48sAA=
// a=recvonly

// ParseSdp parses the sdp session content.
func ParseSdp(r io.Reader) (SessionSection, error) {
	var packet SessionSection
	s := bufio.NewScanner(r)
	for s.Scan() {
		parts := strings.SplitN(s.Text(), "=", 2)
		if len(parts) == 2 {
			if len(parts[0]) != 1 {
				return packet, errors.New("SDP only allows 1-character variables")
			}

			switch parts[0] {
			// version
			case "v":
				ver, err := strconv.Atoi(parts[1])
				if err != nil {
					return packet, err
				}
				packet.Version = ver
			// owner/creator and session identifier
			case "o":
				// o=<username> <session id> <version> <network type> <address type> <address>
				ogParts := strings.Split(parts[1], " ")
				if len(ogParts) != 6 {
					return packet, errors.New("originator field is wrong")
				}
				packet.Originator = SessionSectionOriginator{ogParts[0], ogParts[1], ogParts[2], ogParts[3], ogParts[4], ogParts[5]}
			// session name
			case "s":
				packet.SessionName = parts[1]
			// session information
			case "i":
				packet.SessionInformation = parts[1]
			// URI of description
			case "u":
				packet.URI = parts[1]
			// email address
			case "e":
				packet.Emails = append(packet.Emails, parts[1])
			// phone number
			case "p":
				packet.Phones = append(packet.Phones, parts[1])
			// connection information - not required if included in all media
			case "c":
				cnParts := strings.Split(parts[1], " ")
				if len(cnParts) != 3 {
					return packet, errors.New("connection info field is wrong")
				}
				packet.ConnectionInformation = SessionSectionConnectionInformation{cnParts[0], cnParts[1], cnParts[2]}
			// bandwidth information
			case "b":
				// TODO: parse this
				packet.BandwidthInformation = append(packet.BandwidthInformation, parts[1])
			case "t":
				// TODO: t might occur multiple times...need to see an example in order to learn how to deal with it.
				tmParts := strings.Split(parts[1], " ")
				if len(tmParts) != 2 {
					return packet, errors.New("time field is wrong")
				}
				packet.Time = append(packet.Time, SessionSectionTime{tmParts[0], tmParts[1]})
			case "r":
				// TODO: need to parse repeats, it may also appear multiple times.
				packet.Repeat = append(packet.Repeat, parts[1])
			// time zone.
			case "z":
				// TODO: need to parse time zone.
				packet.TimeZone = parts[1]
			// encryption key
			case "k":
				packet.EncryptionKey = parts[1]
			}
		}
	}
	return packet, nil
}
