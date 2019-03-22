package client

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/WUMUXIAN/go-common-utils/timeutil"
	"github.com/WUMUXIAN/rtsp/rtcp"
	"github.com/WUMUXIAN/rtsp/rtp"
	"github.com/WUMUXIAN/rtsp/sdp"
	"github.com/nareix/joy4/av"
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

// State
const (
	StateDescribed = iota + 1
	StateSetuped
	StateWaitCodecData
	StateReadyForAVPacket
)

// ResponseWriter defines a response writer interface
type ResponseWriter interface {
	http.ResponseWriter
}

// Session defines a rtsp session.
type Session struct {
	cSeq int
	conn net.Conn

	bufConn *bufio.Reader

	session string
	uri     string
	host    string
	Digest  *DigestAuthencitation

	state int

	streams []*Stream

	rtpChan  chan rtp.Packet
	rtcpChan chan rtcp.Packet

	resChan chan Response
	errChan chan error

	timeout int // in seconds.

	timeoutTimer int
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
	if err != nil {
		return
	}
	session.bufConn = bufio.NewReader(session.conn)

	rtpChan := make(chan rtp.Packet, 10)
	rtcpChan := make(chan rtcp.Packet, 10)
	resChan := make(chan Response, 10)

	session.rtpChan = rtpChan
	session.rtcpChan = rtcpChan
	session.resChan = resChan
	session.errChan = make(chan error, 100)

	go session.poll()
	return
}

// NewRequest creates a new request.
func (s *Session) newRequest(method, urlStr, cSeq string, body []byte) (*Request, error) {
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

	return req, nil
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

func (s *Session) handleUnauthorized(response Response) {
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
	return
}

func (s *Session) sendRequest(req *Request) error {
	if s.conn == nil {
		return errors.New("connection not established")
	}
	fmt.Println(req)
	_, err := io.WriteString(s.conn, req.String())
	return err
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

	_, err = s.readResponse()
	if err != nil {
		return err
	}
	return nil
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

	res, err := s.readResponse()
	if err != nil {
		return err
	}

	// let's parse the SDP response and create the stream
	p, err := sdp.ParseSdp(bytes.NewBuffer(res.Body))
	if err != nil {
		return err
	}
	fmt.Printf("The Parsed Sdp: %+v\n", p)

	// After describing, we can create the stream already.
	for _, media := range p.Medias {
		stream := &Stream{Sdp: media}
		stream.MakeCodecData()
		s.streams = append(s.streams, stream)
	}

	s.state = StateDescribed

	return nil
}

// Setup setups how the stream will be transported.
func (s *Session) Setup() error {
	if s.state != StateDescribed {
		return errors.New("not described yet")
	}
	// setup all streams.
	for _, stream := range s.streams {
		req, err := s.newRequest(SETUP, s.uri+"/"+stream.Sdp.Control, s.nextCSeq(), nil)
		if err != nil {
			return err
		}

		req.Header.Add("Transport", stream.Sdp.Procotol+"/TCP")

		err = s.sendRequest(req)
		if err != nil {
			return err
		}

		_, err = s.readResponse()
		if err != nil {
			return err
		}
	}
	s.state = StateSetuped
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

	_, err = s.readResponse()
	if err != nil {
		return err
	}
	if s.allCodecDataReady() {
		s.state = StateReadyForAVPacket
	} else {
		s.state = StateWaitCodecData
	}

	s.timeoutTimer = int(timeutil.CurrentTimeStamp())

	return nil
}

func (s *Session) allCodecDataReady() bool {
	for _, stream := range s.streams {
		if stream.CodecData == nil {
			return false
		}
	}
	return true
}

func (s *Session) poll() {
	for {
		var b byte
		var err error
		var l int
		if b, err = s.bufConn.ReadByte(); err != nil {
			s.errChan <- err
			continue
		}
		// we check the very first character.
		if b == '$' {
			// prase this RTP/RTCP packet.
			header := make([]byte, 3)

			if l, err = io.ReadFull(s.bufConn, header); err != nil || l != 3 {
				fmt.Println("err, rtp/rtcp header not correct", err, l, header)
				s.errChan <- err
				continue
			}
			length := toUint(header[1:3])
			channel := toUint(header[0:1])

			// fmt.Println("length:", length, "channel:", channel)

			// ready length of data.
			data := make([]byte, length)

			if l, err = io.ReadFull(s.bufConn, data); err != nil || l != int(length) {
				fmt.Println("err, rtp/rtcp data not correct", err, l, length)
				s.errChan <- err
				return
			}

			// this means it's RTP
			if channel%2 == 0 {
				s.rtpChan <- rtp.ParsePacket(data, channel/2)
			} else {
				s.rtcpChan <- rtcp.ParsePacket(data)
				// TODO: remove this if rtcp packet is used later.
				<-s.rtcpChan
			}
		} else if b == 'R' {
			header := make([]byte, 3)
			if l, err = io.ReadFull(s.bufConn, header); err != nil || l != 3 || string(header) != "TSP" {
				// This mean it's not RTSP header start, we don't do anything and let it go.
				// This is not supported to happend usually.
				fmt.Println("Read RTSP header error", err, string(b)+string(header))
			} else {
				// Here we start to parse the RTSP header.
				data := []byte("RTSP")
				lf := false
				lfpos := 0
				pos := 0
				for {
					if b, err = s.bufConn.ReadByte(); err != nil {
						break
					}
					data = append(data, b)
					if b == '\n' {
						if !lf {
							lf = true
							lfpos = pos
						} else {
							if pos-lfpos <= 2 {
								// we reach two consecutive lf, end of payload.
								break
							} else {
								lfpos = pos
							}
						}
					}
					pos++
				}
				if err != nil {
					s.errChan <- err
					continue
				}

				// fmt.Println("Here we should get all the RTSP header:", string(data))

				// Let's parse the response
				var res *Response
				res, err = ReadResponse(bytes.NewBuffer(data))
				if err != nil {
					s.errChan <- err
					continue
				}
				// If the content length is not 0, the followed data is the content, let's read it.
				res.ContentLength, _ = strconv.ParseInt(res.Header.Get("Content-Length"), 10, 64)
				if res.ContentLength > 0 {
					res.Body = make([]byte, res.ContentLength)
					_, err = io.ReadFull(s.bufConn, res.Body)
				}
				s.resChan <- *res
			}
		}
	}
}

func (s *Session) keepAlive() {
	req, err := s.newRequest(GETPARAMETER, s.uri, s.nextCSeq(), nil)
	if err != nil {
		return
	}
	req.Header.Add("Session", s.session)
	err = s.sendRequest(req)
	if err != nil {
		fmt.Println("Send Request error:", err)
		return
	}
	_, err = s.readResponse()
	if err != nil {
		fmt.Println("Read Response error:", err)
	}
	return
}

// ReadAVPacket tried to read an av packet for the stream
func (s *Session) ReadAVPacket() (avPacket *av.Packet, err error) {
	if s.state < StateWaitCodecData {
		return nil, errors.New("stream not played yet")
	}

	currentTime := int(timeutil.CurrentTimeStamp())
	// we will do keep alive before it timeout.
	if currentTime-s.timeoutTimer > s.timeout {
		s.timeoutTimer = currentTime
		// Send keep alive..
		go s.keepAlive()
	}

	// Before we are ready for AV Packet, we need to settle the codec first.
	for {
		if s.state != StateReadyForAVPacket {
			// Let's read a RTP packet out.
			rtpPacket := <-s.rtpChan
			s.streams[rtpPacket.StreamIdx].HandleRtpPacket(rtpPacket)
		} else {
			break
		}

		if s.allCodecDataReady() {
			s.state = StateReadyForAVPacket
			break
		}
	}

	for {
		rtpPacket := <-s.rtpChan
		var pkt av.Packet
		var ok bool
		pkt, ok, err = s.streams[rtpPacket.StreamIdx].HandleRtpPacket(rtpPacket)
		if err != nil {
			return
		}
		if ok {
			avPacket = &pkt
			return
		}
	}
}

// readResponse reads the server's response.
// Don't call this once you start to play.
func (s *Session) readResponse() (res *Response, err error) {
	select {
	case resp := <-s.resChan:
		fmt.Println(resp)
		s.handleUnauthorized(resp)
		// Deal with session ID.
		if session := resp.Header.Get("Session"); session != "" {
			sessionInfo := strings.Split(session, ";")
			s.session = sessionInfo[0]
			if len(sessionInfo) > 1 {
				timeoutInfo := strings.Split(sessionInfo[1], "=")
				s.timeout, _ = strconv.Atoi(timeoutInfo[1])
				fmt.Printf("Time out in %d seconds\n", s.timeout)
			}
		}
		return &resp, nil
	case err = <-s.errChan:
		fmt.Println("response error:", err)
		return nil, err
	}
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

func toUint(arr []byte) (ret uint) {
	for i, b := range arr {
		ret |= uint(b) << (8 * uint(len(arr)-i-1))
	}
	return ret
}
