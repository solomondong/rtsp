package sdp

import (
	"bufio"
	"encoding/base64"
	"encoding/hex"
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

// ConnectionInformation defines ConnectionInformation body
type ConnectionInformation struct {
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

// SessionSectionMedia defines SessionSectionMedia body
type SessionSectionMedia struct {
	Type                  string
	Port                  int
	Procotol              string
	PayloadType           int
	Title                 string
	ConnectionInformation ConnectionInformation
	BandwidthInformation  []string
	EncryptionKey         string
	BooleanAttributes     map[string]bool
	KVAttributes          map[string]string

	// These attributes are explicit
	Control            string
	Framerate          float64
	Rtpmap             int
	CodecType          string
	TimeScale          int
	Config             []byte
	SpropParameterSets [][]byte
	SizeLength         int
	IndexLength        int
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
	ConnectionInformation ConnectionInformation
	BandwidthInformation  []string
	TimeZone              string
	EncryptionKey         string
	Time                  []SessionSectionTime
	Repeat                []string
	BooleanAttributes     map[string]bool
	KVAttributes          map[string]string
	Medias                []SessionSectionMedia
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
	packet.BooleanAttributes = make(map[string]bool)
	packet.KVAttributes = make(map[string]string)
	s := bufio.NewScanner(r)
	mediaSectionStarted := false
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
				if !mediaSectionStarted {
					packet.SessionInformation = parts[1]
				} else {
					packet.Medias[len(packet.Medias)-1].Title = parts[1]
				}
			// URI of description
			case "u":
				packet.URI = parts[1]
			// email address
			case "e":
				packet.Emails = append(packet.Emails, parts[1])
			// phone numberRtpmap
			case "p":
				packet.Phones = append(packet.Phones, parts[1])
			// connection information - not required if included in all media
			case "c":
				cnParts := strings.Split(parts[1], " ")
				if len(cnParts) != 3 {
					return packet, errors.New("connection info field is wrong")
				}
				if !mediaSectionStarted {
					packet.ConnectionInformation = ConnectionInformation{cnParts[0], cnParts[1], cnParts[2]}
				} else {
					packet.Medias[len(packet.Medias)-1].ConnectionInformation = ConnectionInformation{cnParts[0], cnParts[1], cnParts[2]}
				}
			// bandwidth information
			case "b":
				// TODO: parse this
				if !mediaSectionStarted {
					packet.BandwidthInformation = append(packet.BandwidthInformation, parts[1])
				} else {
					packet.Medias[len(packet.Medias)-1].BandwidthInformation = append(packet.Medias[len(packet.Medias)-1].BandwidthInformation, parts[1])
				}
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
			// encryption keyRtpmap
			case "k":
				if !mediaSectionStarted {
					packet.EncryptionKey = parts[1]
				} else {
					packet.Medias[len(packet.Medias)-1].EncryptionKey = parts[1]
				}
			case "a":
				// the attributes.
				kv := strings.Split(parts[1], ":")
				if len(kv) == 1 {
					if !mediaSectionStarted {
						packet.BooleanAttributes[kv[0]] = true
					} else {
						packet.Medias[len(packet.Medias)-1].BooleanAttributes[kv[0]] = true
					}
				} else {
					if !mediaSectionStarted {
						packet.KVAttributes[kv[0]] = kv[1]
					} else {
						// here we will cater for the special keys.
						switch kv[0] {
						case "control":
							packet.Medias[len(packet.Medias)-1].Control = kv[1]
						case "framerate":
							packet.Medias[len(packet.Medias)-1].Framerate, _ = strconv.ParseFloat(kv[1], 64)
						case "rtpmap":
							rtpmapVals := strings.Split(kv[1], " ")
							packet.Medias[len(packet.Medias)-1].Rtpmap, _ = strconv.Atoi(rtpmapVals[0])
							codecs := strings.Split(rtpmapVals[1], "/")
							packet.Medias[len(packet.Medias)-1].CodecType = codecs[0]
							packet.Medias[len(packet.Medias)-1].TimeScale, _ = strconv.Atoi(codecs[1])
							// 96 packetization-mode=1;profile-level-id=64002A;sprop-parameter-sets=Z2QAKqwsaoHgCJ+WbgICAgQA,aO48sAA=
						case "fmtp":
							fmtpVals := strings.Split(kv[1], ";")
							for _, fmtpVal := range fmtpVals {
								kvals := strings.SplitN(fmtpVal, "=", 2)
								if len(kvals) == 2 {
									switch kvals[0] {
									case "config":
										packet.Medias[len(packet.Medias)-1].Config, _ = hex.DecodeString(kvals[1])
									case "sizelength":
										packet.Medias[len(packet.Medias)-1].SizeLength, _ = strconv.Atoi(kvals[1])
									case "indexlength":
										packet.Medias[len(packet.Medias)-1].IndexLength, _ = strconv.Atoi(kvals[1])
									case "sprop-parameter-sets":
										fields := strings.Split(kvals[1], ",")
										for _, field := range fields {
											val, _ := base64.StdEncoding.DecodeString(field)
											packet.Medias[len(packet.Medias)-1].SpropParameterSets = append(packet.Medias[len(packet.Medias)-1].SpropParameterSets, val)
										}
									}
								}
							}
						default:
							packet.Medias[len(packet.Medias)-1].KVAttributes[kv[0]] = kv[1]
						}

					}
				}
			case "m":
				// the media.
				mediaSectionStarted = true
				maParts := strings.Split(parts[1], " ")
				media := SessionSectionMedia{
					Type:              maParts[0],
					Procotol:          maParts[2],
					BooleanAttributes: make(map[string]bool),
					KVAttributes:      make(map[string]string),
				}
				media.Port, _ = strconv.Atoi(maParts[1])
				media.PayloadType, _ = strconv.Atoi(maParts[3])
				packet.Medias = append(packet.Medias, media)
			}
		}
	}
	return packet, nil
}
