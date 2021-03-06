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
	BrokerURL   string
	ClientID    string
	BaseTopic   string
	InfoFunc    func(string)
	SampleRate  int
	BitRate     int
	DeviceQueue int
	MaxQueue    int
}

type Stream struct {
	config  Config
	svc     *client.Service
	ready   bool
	writer  *pcmWriter
	encoder *wav.Encoder
	queue   int
	mutex   sync.Mutex
}

func NewStream(config Config) *Stream {
	// create service
	svc := client.NewService(100)
	svc.QueueTimeout = 0

	// subscribe topics
	svc.Subscribe(config.BaseTopic+"/streamy/queue", 0)

	// prepare stream
	stream := &Stream{
		config: config,
		svc:    svc,
	}

	// set state callbacks
	svc.OnlineCallback = func(resumed bool) {
		// acquire mutex
		stream.mutex.Lock()
		defer stream.mutex.Unlock()

		// set state
		stream.ready = true

		// emit info
		if config.InfoFunc != nil {
			config.InfoFunc("online")
		}
	}
	svc.OfflineCallback = func() {
		// acquire mutex
		stream.mutex.Lock()
		defer stream.mutex.Unlock()

		// set state
		stream.ready = false

		// emit info
		if config.InfoFunc != nil {
			config.InfoFunc("offline")
		}
	}
	svc.ErrorCallback = func(err error) {
		// emit info
		if config.InfoFunc != nil {
			config.InfoFunc(fmt.Sprintf("error: %s", err.Error()))
		}
	}

	// set message callback
	svc.MessageCallback = func(msg *packet.Message) error {
		// acquire mutex
		stream.mutex.Lock()
		defer stream.mutex.Unlock()

		// parse and set length
		stream.queue, _ = strconv.Atoi(string(msg.Payload))

		return nil
	}

	return stream
}

func (s *Stream) Connect() {
	// start service
	s.svc.Start(client.NewConfigWithClientID(s.config.BrokerURL, s.config.ClientID))
}

func (s *Stream) Write(samples []int) (int, time.Duration) {
	// acquire mutex
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// check state
	if !s.ready {
		return s.queue, 0
	}

	// check queue
	if s.queue >= s.config.MaxQueue {
		return s.queue, 0
	}

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
		Data:           samples,
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
	s.svc.Publish(s.config.BaseTopic+"/streamy/write", chunk, 0, false)

	// determine timeout
	timeout := time.Second * time.Duration(len(samples)) / time.Duration(s.config.SampleRate)
	if s.queue < 2 {
		timeout = 0
	} else if s.queue < s.config.DeviceQueue/2 {
		timeout /= 2
	}

	// increment queue
	s.queue++

	return s.queue, timeout
}

func (s *Stream) Queue() int {
	// acquire mutex
	s.mutex.Lock()
	defer s.mutex.Unlock()

	return s.queue
}

func (s *Stream) Reset() {
	// acquire mutex
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// clear writer and encoder
	s.writer = nil
	s.encoder = nil

	// send stop
	s.svc.Publish(s.config.BaseTopic+"/streamy/stop", nil, 0, false)
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
