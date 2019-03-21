package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"strings"

	"github.com/WUMUXIAN/rtsp/client"
	"github.com/WUMUXIAN/rtsp/sdp"
	"github.com/nareix/joy4/format/rtsp"
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

func testJoy4(rtspURL string) {
	client, err := rtsp.Dial(rtspURL)
	if err != nil {
		panic(err)
	}
	client.Options()
	client.Describe()
	client.SetupAll()
	client.Play()

	for {
		pkt, err := client.ReadPacket()
		if err != nil {
			panic(err)
		}
		fmt.Println(pkt.IsKeyFrame)
	}

}

func main() {
	if len(flag.Args()) >= 1 {
		rtspURL := flag.Args()[0]
		// testJoy4(rtspURL)

		sess, err := client.NewSession(rtspURL)
		if err != nil {
			log.Fatalln(err)
		}
		err = sess.Options()
		if err != nil {
			log.Fatalln(err)
		}
		res, err := sess.ReadResponse()
		if err != nil {
			log.Fatalln(err)
		}
		fmt.Println(res)

		err = sess.Describe()
		if err != nil {
			log.Fatalln(err)
		}
		res, err = sess.ReadResponse()
		if err != nil {
			log.Fatalln(err)
		}
		fmt.Println(res)

		p, err := sdp.ParseSdp(&io.LimitedReader{R: res.Body, N: res.ContentLength})
		if err != nil {
			log.Fatalln(err)
		}
		log.Printf("%+v", p)

		// After describing, we can create the stream already.
		stream := &client.Stream{Sdp: p.Medias[0]}
		stream.MakeCodecData()

		err = sess.Setup(p.Medias[0].Control, p.Medias[0].Procotol+"/TCP")
		if err != nil {
			log.Fatalln(err)
		}
		res, err = sess.ReadResponse()
		if err != nil {
			log.Fatalln(err)
		}
		fmt.Println(res)

		transport := res.Header.Get("Transport")
		fmt.Println("Transport:", strings.Split(transport, ";"))

		err = sess.Play()
		if err != nil {
			log.Fatalln(err)
		}
		res, err = sess.ReadResponse()
		if err != nil {
			log.Fatalln(err)
		}
		fmt.Println(res)

		// rtpSession := rtp.NewTCPInterleavedSession(sess.Conn())
		// for {
		// 	select {
		// 	case rtpPacket := <-rtpSession.RtpChan:
		// 		// fmt.Println(rtpPacket)
		// 		pkt, ok, err := stream.HandleRtpPacket(rtpPacket)
		// 		if ok {
		// 			fmt.Println("AV Pkt got,", pkt.IsKeyFrame, err)
		// 		}
		// 		fmt.Println("One packet processed.")
		// 		// case rtcpPacket := <-rtpSession.RtcpChan:
		// 		// 	fmt.Println(rtcpPacket)
		// 	}
		// }

	} else {
		r, err := client.ReadRequest(bytes.NewBufferString(sampleRequest))
		if err != nil {
			fmt.Println(err)
		} else {
			fmt.Println(r)
		}

		res, err := client.ReadResponse(bytes.NewBufferString(sampleResponse))
		if err != nil {
			fmt.Println(err)
		} else {
			fmt.Println(res)
		}
	}
}
