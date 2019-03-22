package client

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
)

// Response defines a RTSP response
type Response struct {
	Proto      string
	ProtoMajor int
	ProtoMinor int

	StatusCode int
	Status     string

	ContentLength int64

	Header http.Header
	Body   []byte
}

// String format the response print.
func (res Response) String() string {
	s := fmt.Sprintf("%s/%d.%d %d %s\n", res.Proto, res.ProtoMajor, res.ProtoMinor, res.StatusCode, res.Status)
	for k, v := range res.Header {
		for _, v := range v {
			s += fmt.Sprintf("%s: %s\n", k, v)
		}
	}
	return s
}

// ReadResponse reads a RTSP response from io.reader.
func ReadResponse(r io.Reader) (res *Response, err error) {
	res = new(Response)
	res.Header = make(map[string][]string)

	b := bufio.NewReader(r)
	var str string

	// TODO: allow CR, LF, or CRLF
	if str, err = b.ReadString('\n'); err != nil {
		return
	}

	parts := strings.SplitN(str, " ", 3)
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
		if str, err = b.ReadString('\n'); err != nil {
			return
		} else if str = strings.TrimRight(str, "\r\n"); str == "" {
			break
		}

		parts := strings.SplitN(str, ":", 2)
		res.Header.Add(strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]))
	}

	return
}
