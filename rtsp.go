package rtsp

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/WUMUXIAN/go-common-utils/codec"
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

// Request defines a request body
type Request struct {
	Method        string
	URL           *url.URL
	Proto         string
	ProtoMajor    int
	ProtoMinor    int
	Header        http.Header
	ContentLength int
	Body          io.ReadCloser
}

// String prints a request in a beautiful way.
func (r Request) String() string {
	s := fmt.Sprintf("%s %s %s/%d.%d\r\n", r.Method, r.URL, r.Proto, r.ProtoMajor, r.ProtoMinor)
	for k, v := range r.Header {
		for _, v := range v {
			s += fmt.Sprintf("%s: %s\r\n", k, v)
		}
	}
	s += "\r\n"
	if r.Body != nil {
		str, _ := ioutil.ReadAll(r.Body)
		s += string(str)
	}
	return s
}

// NewRequest creates a new request.
func NewRequest(method, urlStr, cSeq string, body io.ReadCloser) (*Request, error) {
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
	return req, nil
}

// DigestAuthencitation defines different parts of a digest authenciation
type DigestAuthencitation struct {
	UserName string
	Password string
	Realm    string
	Nonce    string
}

// GetDigestResponse calculates a response for digest authenciation
func (d DigestAuthencitation) GetDigestResponse(method, uri string) (response string) {
	hash1 := codec.GetHash(codec.MD5, []byte(d.UserName+":"+d.Realm+":"+d.Password))
	hash2 := codec.GetHash(codec.MD5, []byte(method+":"+uri))
	response = codec.GetHash(codec.MD5, []byte(hash1.Hex()+":"+d.Nonce+":"+hash2.Hex())).Hex()
	return
}

// Session defines a rtsp session.
type Session struct {
	cSeq    int
	conn    net.Conn
	session string
	uri     string
	host    string
	Digest  *DigestAuthencitation
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
		return nil, err
	}
	return
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

// Describe describes the stream
func (s *Session) Describe() (*Response, error) {
	req, err := NewRequest(DESCRIBE, s.uri, s.nextCSeq(), nil)
	if err != nil {
		panic(err)
	}

	req.Header.Add("Accept", "application/sdp")

	if s.conn == nil {
		s.conn, err = net.Dial("tcp", req.URL.Host)
		if err != nil {
			return nil, err
		}
	}

	_, err = io.WriteString(s.conn, req.String())
	if err != nil {
		return nil, err
	}
	return ReadResponse(s.conn)
}

// Options sends a Options command
func (s *Session) Options() (*Response, error) {
	req, err := NewRequest(OPTIONS, s.uri, s.nextCSeq(), nil)
	if err != nil {
		panic(err)
	}

	s.injectAuthencitationInfo(req, OPTIONS)

	if s.conn == nil {
		s.conn, err = net.Dial("tcp", s.host)
		if err != nil {
			return nil, err
		}
	}

	fmt.Println(req.String())

	_, err = io.WriteString(s.conn, req.String())
	if err != nil {
		return nil, err
	}
	return s.handleUnauthorized(ReadResponse(s.conn))
}

// Setup setups how the stream will be transported.
func (s *Session) Setup(transport string) (*Response, error) {
	req, err := NewRequest(SETUP, s.uri, s.nextCSeq(), nil)
	if err != nil {
		panic(err)
	}

	req.Header.Add("Transport", transport)

	if s.conn == nil {
		s.conn, err = net.Dial("tcp", req.URL.Host)
		if err != nil {
			return nil, err
		}
	}

	_, err = io.WriteString(s.conn, req.String())
	if err != nil {
		return nil, err
	}
	resp, err := ReadResponse(s.conn)
	s.session = resp.Header.Get("Session")
	return resp, err
}

func (s *Session) Play(urlStr, sessionId string) (*Response, error) {
	req, err := NewRequest(PLAY, urlStr, s.nextCSeq(), nil)
	if err != nil {
		panic(err)
	}

	req.Header.Add("Session", sessionId)

	if s.conn == nil {
		s.conn, err = net.Dial("tcp", req.URL.Host)
		if err != nil {
			return nil, err
		}
	}

	_, err = io.WriteString(s.conn, req.String())
	if err != nil {
		return nil, err
	}
	return ReadResponse(s.conn)
}

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

// super simple RTSP parser; would be nice if net/http would allow more general parsing
func ReadRequest(r io.Reader) (req *Request, err error) {
	req = new(Request)
	req.Header = make(map[string][]string)

	b := bufio.NewReader(r)
	var s string

	// TODO: allow CR, LF, or CRLF
	if s, err = b.ReadString('\n'); err != nil {
		return
	}

	parts := strings.SplitN(s, " ", 3)
	req.Method = parts[0]
	if req.URL, err = url.Parse(parts[1]); err != nil {
		return
	}

	req.Proto, req.ProtoMajor, req.ProtoMinor, err = ParseRTSPVersion(parts[2])
	if err != nil {
		return
	}

	// read headers
	for {
		if s, err = b.ReadString('\n'); err != nil {
			return
		} else if s = strings.TrimRight(s, "\r\n"); s == "" {
			break
		}

		parts := strings.SplitN(s, ":", 2)
		req.Header.Add(strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]))
	}

	req.ContentLength, _ = strconv.Atoi(req.Header.Get("Content-Length"))
	fmt.Println("Content Length:", req.ContentLength)
	req.Body = closer{b, r}
	return
}

type Response struct {
	Proto      string
	ProtoMajor int
	ProtoMinor int

	StatusCode int
	Status     string

	ContentLength int64

	Header http.Header
	Body   io.ReadCloser
}

func (res Response) String() string {
	s := fmt.Sprintf("%s/%d.%d %d %s\n", res.Proto, res.ProtoMajor, res.ProtoMinor, res.StatusCode, res.Status)
	for k, v := range res.Header {
		for _, v := range v {
			s += fmt.Sprintf("%s: %s\n", k, v)
		}
	}
	return s
}

// ReadResponse reads the server's response
func ReadResponse(r io.Reader) (res *Response, err error) {
	res = new(Response)
	res.Header = make(map[string][]string)

	b := bufio.NewReader(r)
	var s string

	// TODO: allow CR, LF, or CRLF
	if s, err = b.ReadString('\n'); err != nil {
		return
	}

	parts := strings.SplitN(s, " ", 3)
	res.Proto, res.ProtoMajor, res.ProtoMinor, err = ParseRTSPVersion(parts[0])
	if err != nil {
		return
	}

	if res.StatusCode, err = strconv.Atoi(parts[1]); err != nil {
		return
	}

	res.Status = strings.TrimSpace(parts[2])

	// read headers
	for {
		if s, err = b.ReadString('\n'); err != nil {
			return
		} else if s = strings.TrimRight(s, "\r\n"); s == "" {
			break
		}

		parts := strings.SplitN(s, ":", 2)
		res.Header.Add(strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]))
	}

	res.ContentLength, _ = strconv.ParseInt(res.Header.Get("Content-Length"), 10, 64)

	res.Body = closer{b, r}
	return
}
