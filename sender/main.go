package main

import (
	"bufio"
	"bytes"
	_ "embed"
	"encoding/binary"
	"fmt"
	"os"
	"time"

	"github.com/hajimehoshi/go-mp3"

	"streamy"
)

//go:embed sound.mp3
var sound []byte

const brokerURL = "mqtt://localhost:2883"
const sampleRate = 44100
const bitRate = 16
const deviceQueue = 16

func main() {
	// create writer
	stream := streamy.NewStream(streamy.Config{
		BrokerURL: brokerURL,
		ClientID:  "sender",
		BaseTopic: "/test",
		InfoFunc: func(str string) {
			fmt.Println("==>", str)
		},
		SampleRate:  sampleRate,
		BitRate:     bitRate,
		DeviceQueue: deviceQueue,
		MaxQueue:    32,
	})

	// connect
	stream.Connect()

	// prepare handle
	var handle *writer

	// scan lines
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		switch scanner.Text() {
		case "start":
			if handle != nil {
				continue
			}
			stream.Reset()
			handle = newWriter(stream)
			go handle.run()
			fmt.Println("==> started")
		case "stop":
			if handle != nil {
				handle.close()
				handle = nil
				fmt.Println("==> stopped")
			}
			stream.Reset()
		default:
			fmt.Println("==> unknown")
		}
	}
}

type writer struct {
	stream *streamy.Stream
	stop   chan struct{}
	done   chan struct{}
	queue  int
}

func newWriter(stream *streamy.Stream) *writer {
	return &writer{
		stream: stream,
		stop:   make(chan struct{}),
		done:   make(chan struct{}),
	}
}

func (s *writer) close() {
	close(s.stop)
	<-s.done
}

func (s *writer) run() {
	// prepare decoder
	dec, err := mp3.NewDecoder(bytes.NewReader(sound))
	if err != nil {
		panic(err)
	}

	// check sample rate
	if dec.SampleRate() != sampleRate {
		panic("invalid sample rate")
	}

	// prepare input buffer 1152 samples (~26ms) @ 16bit/2CH
	input := make([]byte, 1152*4)

	// writer audio
	for {
		// read sample
		n, err := dec.Read(input)
		if err != nil {
			panic(err)
		}

		// convert sample
		samples := make([]int, n/4)
		for i := 0; i < n; i += 4 {
			samples[i/4] = int(int16(binary.LittleEndian.Uint16(input[i:])))
		}

		// write chunk
		_, timeout := s.stream.Write(samples)

		// await timeout
		time.Sleep(timeout)

		// check close
		select {
		case <-s.stop:
			close(s.done)
			return
		default:
		}
	}
}
