package main

import (
	"bytes"
	"flag"
	"fmt"
	"log"

	"github.com/WUMUXIAN/rtsp/client"
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

		err = sess.Describe()
		if err != nil {
			log.Fatalln(err)
		}

		err = sess.Setup()
		if err != nil {
			log.Fatalln(err)
		}

		err = sess.Play()
		if err != nil {
			log.Fatalln(err)
		}

		for {
			pkt, err := sess.ReadAVPacket()
			if err != nil {
				panic(err)
			}
			fmt.Println("Key frame:", pkt.IsKeyFrame)
		}
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
