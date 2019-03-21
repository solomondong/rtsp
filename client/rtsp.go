package client

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/WUMUXIAN/rtsp/rtcp"
	"github.com/WUMUXIAN/rtsp/rtp"
)

// RTSP methods
const (
	// Client to server for presentation and stream objects; recommended
	DESCRIBE = "DESCRIBE"
	// Bidirectional for client and stream objects; optional
	ANNOUNCE = "ANNOUNCE"
	// Bidirectional for client and stream objects; optional
	GETPARAMETER = "GET_PARAMETER"
	// Bidirectional for client and stream objects; required for Client to server, optional for server to client
	OPTIONS = "OPTIONS"
	// Client to server for presentation and stream objects; recommended
	PAUSE = "PAUSE"
	// Client to server for presentation and stream objects; required
	PLAY = "PLAY"
	// Client to server for presentation and stream objects; optional
	RECORD = "RECORD"
	// Server to client for presentation and stream objects; optional
	REDIRECT = "REDIRECT"
	// Client to server for stream objects; required
	SETUP = "SETUP"
	// Bidirectional for presentation and stream objects; optional
	SETPARAMETER = "SET_PARAMETER"
	// Client to server for presentation and stream objects; required
	TEARDOWN = "TEARDOWN"
)

// Response status
const (
	// all requests
	Continue = 100

	// all requests
	OK = 200
	// RECORD
	Created = 201
	// RECORD
	LowOnStorageSpace = 250

	// all requests
	MultipleChoices = 300
	// all requests
	MovedPermanently = 301
	// all requests
	MovedTemporarily = 302
	// all requests
	SeeOther = 303
	// all requests
	UseProxy = 305

	// all requests
	BadRequest = 400
	// all requests
	Unauthorized = 401
	// all requests
	PaymentRequired = 402
	// all requests
	Forbidden = 403
	// all requests
	NotFound = 404
	// all requests
	MethodNotAllowed = 405
	// all requests
	NotAcceptable = 406
	// all requests
	ProxyAuthenticationRequired = 407
	// all requests
	RequestTimeout = 408
	// all requests
	Gone = 410
	// all requests
	LengthRequired = 411
	// DESCRIBE, SETUP
	PreconditionFailed = 412
	// all requests
	RequestEntityTooLarge = 413
	// all requests
	RequestURITooLong = 414
	// all requests
	UnsupportedMediaType = 415
	// SETUP
	Invalidparameter = 451
	// SETUP
	IllegalConferenceIdentifier = 452
	// SETUP
	NotEnoughBandwidth = 453
	// all requests
	SessionNotFound = 454
	// all requests
	MethodNotValidInThisState = 455
	// all requests
	HeaderFieldNotValid = 456
	// PLAY
	InvalidRange = 457
	// SET_PARAMETER
	ParameterIsReadOnly = 458
	// all requests
	AggregateOperationNotAllowed = 459
	// all requests
	OnlyAggregateOperationAllowed = 460
	// all requests
	UnsupportedTransport = 461
	// all requests
	DestinationUnreachable = 462

	// all requests
	InternalServerError = 500
	// all requests
	NotImplemented = 501
	// all requests
	BadGateway = 502
	// all requests
	ServiceUnavailable = 503
	// all requests
	GatewayTimeout = 504
	// all requests
	RTSPVersionNotSupported = 505
	// all requests
	OptionNotsupport = 551
)

// ResponseWriter defines a response writer interface
type ResponseWriter interface {
	http.ResponseWriter
}

// Session defines a rtsp session.
type Session struct {
	cSeq int
	conn net.Conn

	session string
	uri     string
	host    string
	Digest  *DigestAuthencitation

	RtpChan  <-chan rtp.Packet
	rtpChan  chan<- rtp.Packet
	RtcpChan <-chan rtcp.Packet
	rtcpChan chan<- rtcp.Packet
}

// NewSession creates a new rtsp session to a certain stream.
func NewSession(rtspAddr string) (session *Session, err error) {
	url, err := url.Parse(rtspAddr)
	if err != nil {
		return nil, err
	}
	if url.Scheme != "rtsp" {
		return nil, errors.New("invalid rtsp address")
	}
	session = new(Session)
	session.Digest = new(DigestAuthencitation)
	if url.User != nil {
		session.Digest.UserName = url.User.Username()
		session.Digest.Password, _ = url.User.Password()
	}
	session.uri = url.Scheme + "://" + url.Host
	session.host = url.Host

	session.conn, err = net.Dial("tcp", session.host)
	return
}

// NewRequest creates a new request.
func (s *Session) newRequest(method, urlStr, cSeq string, body io.ReadCloser) (*Request, error) {
	u, err := url.Parse(urlStr)
	if err != nil {
		return nil, err
	}

	req := &Request{
		Method:     method,
		URL:        u,
		Proto:      "RTSP",
		ProtoMajor: 1,
		ProtoMinor: 0,
		Header:     map[string][]string{"CSeq": []string{cSeq}},
		Body:       body,
	}

	// always inject the authenciation info when send a request if there're any.
	s.injectAuthencitationInfo(req, method)

	fmt.Println(req)
	return req, nil
}

// SetInterleaved sets the session as interleaved
func (s *Session) SetInterleaved() {
	rtpChan := make(chan rtp.Packet, 10)
	rtcpChan := make(chan rtcp.Packet, 10)
	s.rtpChan = rtpChan
	s.RtpChan = rtpChan
	s.rtcpChan = rtcpChan
	s.RtcpChan = rtcpChan
}

// Host returns the host this session connects to.
func (s *Session) Host() string {
	return s.host
}

func (s *Session) nextCSeq() string {
	s.cSeq++
	return strconv.Itoa(s.cSeq)
}

func (s *Session) injectAuthencitationInfo(request *Request, method string) {
	if s.Digest.Nonce != "" {
		digest := fmt.Sprintf("Digest username=\"%s\", realm=\"%s\", nonce=\"%s\", uri=\"%s\", response=\"%s\"",
			s.Digest.UserName, s.Digest.Realm, s.Digest.Nonce, s.uri, s.Digest.GetDigestResponse(method, s.uri))
		request.Header["Authorization"] = []string{
			digest,
		}
	}
}

func (s *Session) handleUnauthorized(response *Response, err error) (*Response, error) {
	if err != nil {
		return response, err
	}
	if response.StatusCode == Unauthorized {
		// In this case, the response header will be containing digest info only.
		for key, digest := range response.Header {
			if key != "Cseq" {
				dg := strings.Replace(digest[0], "Digest", "", -1)
				dgFields := strings.Split(dg, ",")
				for _, dgField := range dgFields {
					dgField = strings.TrimSpace(dgField)
					dgFieldDetails := strings.Split(dgField, "=")
					if dgFieldDetails[0] == "realm" {
						s.Digest.Realm = strings.Trim(dgFieldDetails[1], "\"")
					} else {
						s.Digest.Nonce = strings.Trim(dgFieldDetails[1], "\"")
					}
				}
			}
		}
	}
	return response, err
}

func (s *Session) sendRequest(req *Request) error {
	if s.conn == nil {
		return errors.New("connection not established")
	}

	_, err := io.WriteString(s.conn, req.String())
	return err
}

// Describe describes the stream
func (s *Session) Describe() error {
	req, err := s.newRequest(DESCRIBE, s.uri, s.nextCSeq(), nil)
	if err != nil {
		return err
	}

	req.Header.Add("Accept", "application/sdp")

	err = s.sendRequest(req)

	if err != nil {
		return err
	}
	return nil
}

// Options sends a Options command
func (s *Session) Options() error {
	req, err := s.newRequest(OPTIONS, s.uri, s.nextCSeq(), nil)
	if err != nil {
		return err
	}

	err = s.sendRequest(req)
	if err != nil {
		return err
	}
	return nil
}

// Setup setups how the stream will be transported.
func (s *Session) Setup(trackID, transport string) error {
	req, err := s.newRequest(SETUP, s.uri+"/"+trackID, s.nextCSeq(), nil)
	if err != nil {
		return err
	}

	req.Header.Add("Transport", transport)

	err = s.sendRequest(req)
	if err != nil {
		return err
	}
	return nil
}

// Play plays a video stream given the sessionID
func (s *Session) Play() error {
	req, err := s.newRequest(PLAY, s.uri, s.nextCSeq(), nil)
	if err != nil {
		return err
	}
	req.Header.Add("Session", s.session)

	err = s.sendRequest(req)
	if err != nil {
		return err
	}
	return nil
}

// ReadResponse reads the server's response
func (s *Session) ReadResponse() (res *Response, err error) {
	res, err = ReadResponse(s.conn)
	// Deal with 401 before return.
	res, err = s.handleUnauthorized(res, err)

	// Deal with session ID.
	if session := res.Header.Get("Session"); session != "" {
		s.session = session
	}
	return
}

//
// func (s *Session) ReadPacket() (av.Packet, error) {
//
// }

type closer struct {
	*bufio.Reader
	r io.Reader
}

func (c closer) Close() error {
	if c.Reader == nil {
		return nil
	}
	defer func() {
		c.Reader = nil
		c.r = nil
	}()
	if r, ok := c.r.(io.ReadCloser); ok {
		return r.Close()
	}
	return nil
}

// ParseRTSPVersion partse the version of RTSP protocol.
func ParseRTSPVersion(s string) (proto string, major int, minor int, err error) {
	parts := strings.SplitN(s, "/", 2)
	proto = parts[0]
	parts = strings.SplitN(parts[1], ".", 2)
	if major, err = strconv.Atoi(parts[0]); err != nil {
		return
	}
	if minor, err = strconv.Atoi(parts[0]); err != nil {
		return
	}
	return
}
