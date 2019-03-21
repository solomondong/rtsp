package client

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/WUMUXIAN/go-common-utils/codec"
)

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

// ReadRequest implements a super simple RTSP parser; would be nice if net/http would allow more general parsing
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
