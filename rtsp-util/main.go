package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"strings"

	"github.com/WUMUXIAN/rtsp"
	"github.com/WUMUXIAN/rtsp/rtp"
)

func init() {
	flag.Parse()
}

const sampleRequest = `OPTIONS rtsp://example.com/media.mp4 RTSP/1.0
CSeq: 1
Require: implicit-play
Proxy-Require: gzipped-messages

`

const sampleResponse = `RTSP/1.0 200 OK
CSeq: 1
Public: DESCRIBE, SETUP, TEARDOWN, PLAY, PAUSE

`

func main() {
	if len(flag.Args()) >= 1 {
		rtspURL := flag.Args()[0]

		sess, err := rtsp.NewSession(rtspURL)
		if err != nil {
			log.Fatalln(err)
		}
		res, err := sess.Options()
		if err != nil {
			log.Fatalln(err)
		}
		fmt.Println(res)

		// If we need to authenciate
		if res.StatusCode == rtsp.Unauthorized {
			res, err = sess.Options()
			if err != nil {
				log.Fatalln(err)
			}
			fmt.Println(res)
		}

		res, err = sess.Describe()
		if err != nil {
			log.Fatalln(err)
		}
		fmt.Println(res)

		p, err := rtsp.ParseSdp(&io.LimitedReader{R: res.Body, N: res.ContentLength})
		if err != nil {
			log.Fatalln(err)
		}
		// log.Printf("%+v", p)

		control := p.Medias[0].KVAttributes["control"]
		protocol := p.Medias[0].Procotol + "/TCP"

		res, err = sess.Setup(control, protocol)
		if err != nil {
			log.Fatalln(err)
		}
		// log.Println(res)

		transport := res.Header.Get("Transport")
		fmt.Println(strings.Split(transport, ";"))

		res, err = sess.Play(res.Header.Get("Session"))
		if err != nil {
			log.Fatalln(err)
		}
		log.Println(res)

		rtpSession := rtp.NewTCPInterleavedSession(sess.Conn())
		for {
			select {
			case rtpPacket := <-rtpSession.RtpChan:
				fmt.Println(rtpPacket)
			}
		}

	} else {
		r, err := rtsp.ReadRequest(bytes.NewBufferString(sampleRequest))
		if err != nil {
			fmt.Println(err)
		} else {
			fmt.Println(r)
		}

		res, err := rtsp.ReadResponse(bytes.NewBufferString(sampleResponse))
		if err != nil {
			fmt.Println(err)
		} else {
			fmt.Println(res)
		}
	}
}
