package streamy

import (
	"bytes"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/256dpi/gomqtt/client"
	"github.com/256dpi/gomqtt/packet"
	"github.com/go-audio/audio"
	"github.com/go-audio/wav"
)

type Config struct {
	Broker      string
	Name        string
	Base        string
	Info        func(string)
	Queue       func(int)
	SampleRate  int
	BitRate     int
	DeviceQueue int
}

type Stream struct {
	config  Config
	svc     *client.Service
	writer  *pcmWriter
	encoder *wav.Encoder
	queue   int
	mutex   sync.Mutex
}

func NewStream(config Config) *Stream {
	// create service
	svc := client.NewService(100)

	// subscribe topics
	svc.Subscribe(config.Base+"/queue", 0)

	// set state callbacks
	svc.OnlineCallback = func(resumed bool) {
		config.Info("online")
	}
	svc.OfflineCallback = func() {
		config.Info("offline")
	}
	svc.ErrorCallback = func(err error) {
		config.Info(fmt.Sprintf("error: %s", err.Error()))
	}

	// prepare stream
	stream := &Stream{
		config: config,
		svc:    svc,
	}

	// set message callback
	svc.MessageCallback = func(msg *packet.Message) error {
		// parse length
		length, _ := strconv.Atoi(string(msg.Payload))

		// acquire mutex
		stream.mutex.Lock()
		defer stream.mutex.Unlock()

		// set queue
		stream.queue = length

		// run callback
		if stream.config.Queue != nil {
			stream.config.Queue(length)
		}

		return nil
	}

	return stream
}

func (s *Stream) Connect() {
	// start service
	s.svc.Start(client.NewConfigWithClientID(s.config.Broker, s.config.Name))
}

func (s *Stream) Write(data []int) time.Duration {
	// acquire mutex
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// create writer
	if s.writer == nil {
		s.writer = &pcmWriter{}
	}

	// create encoder
	if s.encoder == nil {
		s.encoder = wav.NewEncoder(s.writer, s.config.SampleRate, s.config.BitRate, 1, 1)
	}

	// prepare buffer
	buffer := audio.IntBuffer{
		Format: &audio.Format{
			NumChannels: 1,
			SampleRate:  s.config.SampleRate,
		},
		Data:           data,
		SourceBitDepth: 16,
	}

	// write buffer
	err := s.encoder.Write(&buffer)
	if err != nil {
		panic(err)
	}

	// get PCM chunk and reset writer
	chunk := s.writer.Bytes()
	s.writer.Reset()

	// send chunk
	s.svc.Publish(s.config.Base+"/write", chunk, 0, false)

	// determine timeout
	timeout := time.Second * time.Duration(len(data)) / time.Duration(s.config.SampleRate)
	if s.queue < 2 {
		timeout = 0
	} else if s.queue < s.config.DeviceQueue/2 {
		timeout /= 2
	}

	// increment queue
	s.queue++

	return timeout
}

func (s *Stream) Reset() {
	// acquire mutex
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// clear writer and encoder
	s.writer = nil
	s.encoder = nil

	// send stop
	s.svc.Publish(s.config.Base+"/stop", nil, 0, false)
}

func (s *Stream) Disconnect() {
	// stop service
	s.svc.Stop(true)
}

type pcmWriter struct {
	bytes.Buffer
}

func (s *pcmWriter) Seek(_ int64, _ int) (int64, error) {
	return 0, nil
}
