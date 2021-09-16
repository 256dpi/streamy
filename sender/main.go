package main

import (
	"bufio"
	"bytes"
	_ "embed"
	"encoding/binary"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/256dpi/gomqtt/client"
	"github.com/256dpi/gomqtt/packet"
	"github.com/go-audio/audio"
	"github.com/go-audio/wav"
	"github.com/hajimehoshi/go-mp3"
)

//go:embed sound.mp3
var sound []byte

const broker = "mqtt://localhost:1883"

const queueLength = 16
const sampleRate = 44100

func main() {
	// prepare handle
	var handle *stream

	// create service
	svc := client.NewService(100)

	// subscribe topics
	svc.Subscribe("/test/queue", 0)

	// set state callbacks
	svc.OnlineCallback = func(resumed bool) {
		fmt.Println("==> online")
	}
	svc.OfflineCallback = func() {
		fmt.Println("==> offline")
	}
	svc.ErrorCallback = func(err error) {
		fmt.Println("==> error", err.Error())
	}

	// set message callback
	svc.MessageCallback = func(msg *packet.Message) error {
		length, _ := strconv.Atoi(string(msg.Payload))
		if handle != nil {
			handle.queue = length
		}
		return nil
	}

	// start service
	svc.Start(client.NewConfigWithClientID(broker, "sender"))

	// scan lines
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		switch scanner.Text() {
		case "start":
			if handle != nil {
				continue
			}
			fmt.Println("==> started")
			handle = newStream(svc)
			go handle.run()
		case "stop":
			if handle != nil {
				handle.close()
				handle = nil
				fmt.Println("==> stopped")
			}
			svc.Publish("/test/stop", nil, 0, false)
		default:
			fmt.Println("==> unknown")
		}
	}
}

type stream struct {
	svc   *client.Service
	stop  chan struct{}
	done  chan struct{}
	queue int
}

func newStream(svc *client.Service) *stream {
	return &stream{
		svc:  svc,
		stop: make(chan struct{}),
		done: make(chan struct{}),
	}
}

func (s *stream) close() {
	close(s.stop)
	<-s.done
}

func (s *stream) run() {
	// prepare decoder
	dec, err := mp3.NewDecoder(bytes.NewReader(sound))
	if err != nil {
		panic(err)
	}

	// check sample rate
	if dec.SampleRate() != sampleRate {
		panic("invalid sample rate")
	}

	// prepare PCM writer
	var writer pcmWriter

	// prepare encoder
	enc := wav.NewEncoder(&writer, dec.SampleRate(), 16, 1, 1)

	// prepare input buffer 1152 samples (~26ms) @ 16bit/2CH
	input := make([]byte, 1152*4)

	// prepare integer array
	var array [1152]int

	// prepare output output
	var output audio.IntBuffer
	output.Format = audio.FormatMono44100
	output.SourceBitDepth = 16

	// stream audio
	for {
		// read sample
		n, err := dec.Read(input)
		if err != nil {
			panic(err)
		}

		// get samples
		samples := n / 4

		// fill array
		var num int
		for i := 0; i < n; i += 4 {
			num++
			array[i/4] = int(int16(binary.LittleEndian.Uint16(input[i:])))
		}

		// set array
		output.Data = array[:num]

		// write output
		err = enc.Write(&output)
		if err != nil {
			panic(err)
		}

		// get PCM chunk
		chunk := writer.Bytes()

		// reset writer
		writer.Reset()

		// send chunk
		s.svc.Publish("/test/write", chunk, 0, false)

		// determine timeout
		timeout := time.Second * time.Duration(samples) / sampleRate
		if s.queue < 2 {
			timeout = 0
		} else if s.queue < queueLength/2 {
			timeout /= 2
		}

		// increment queue
		s.queue++

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

type pcmWriter struct {
	bytes.Buffer
}

func (s *pcmWriter) Seek(_ int64, _ int) (int64, error) {
	return 0, nil
}
