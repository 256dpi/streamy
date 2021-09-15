package main

import (
	"bufio"
	"bytes"
	_ "embed"
	"encoding/binary"
	"fmt"
	"os"
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

func main() {
	// create service
	svc := client.NewService(100)

	// subscribe topics
	svc.Subscribe("/test/audio", 0)

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
		fmt.Println(msg.String())
		return nil
	}

	// start service
	svc.Start(client.NewConfigWithClientID(broker, "sender"))

	// handle
	var stop, done chan struct{}

	// scan lines
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		switch scanner.Text() {
		case "start":
			if stop != nil {
				fmt.Println("==> already streaming")
				continue
			}
			fmt.Println("==> started stream")
			stop = make(chan struct{})
			done = make(chan struct{})
			go stream(svc, stop, done)
		case "stop":
			if stop == nil {
				svc.Publish("/test/stop", nil, 0, false)
				fmt.Println("==> not streaming")
				continue
			}
			close(stop)
			<-done
			svc.Publish("/test/stop", nil, 0, false)
			stop, done = nil, nil
			fmt.Println("==> stopped stream")
		default:
			fmt.Println("==> unknown command")
		}
	}
}

func stream(svc *client.Service, stop, done chan struct{}) {
	// prepare decoder
	dec, err := mp3.NewDecoder(bytes.NewReader(sound))
	if err != nil {
		panic(err)
	}

	// check sample rate
	if dec.SampleRate() != 44100 {
		panic("invalid sample rate")
	}

	// prepare PCM writer
	var writer pcmWriter

	// prepare encoder
	enc := wav.NewEncoder(&writer, dec.SampleRate(), 16, 1, 1)

	// prepare input buffer 1152 samples (10ms) at 16bit/2CH
	input := make([]byte, 1152*4)

	// prepare integer array
	var array [1152]int

	// prepare output output
	var output audio.IntBuffer
	output.Format = audio.FormatMono44100
	output.SourceBitDepth = 16

	// stream audio
	var counter int
	for {
		// read sample
		n, err := dec.Read(input)
		if err != nil {
			panic(err)
		}

		// get samples
		// samples := n / 4

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
		svc.Publish("/test/write", chunk, 0, false)

		// wait a bit
		if counter % 30 != 0 {
			time.Sleep(25 * time.Millisecond)
		}

		// increment
		counter++

		// check close
		select {
		case <-stop:
			close(done)
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
