package main

import (
	"math"
	"sync"
	"time"

	"github.com/256dpi/max-go"

	"streamy"
)

type sender struct {
	signal *max.Inlet
	cmd    *max.Inlet
	state  *max.Outlet
	info   *max.Outlet
	stream *streamy.Stream
	queue  int64
	active bool
	mutex  sync.Mutex
}

func (s *sender) Init(obj *max.Object, args []max.Atom) bool {
	// acquire mutex
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// set active
	s.active = true

	// declare inlets
	s.signal = obj.Inlet(max.Signal, "input", true)
	s.cmd = obj.Inlet(max.Any, "commands", true)

	// declare outlets
	s.state = obj.Outlet(max.Int, "connection state")
	s.info = obj.Outlet(max.Int, "device queue")

	// get broker
	var broker string
	if len(args) > 0 {
		broker, _ = args[0].(string)
	}

	// get name
	var name string
	if len(args) > 1 {
		name, _ = args[1].(string)
	}

	// get base
	var base string
	if len(args) > 2 {
		base, _ = args[2].(string)
	}

	// run setter
	go func() {
		for {
			time.Sleep(33 * time.Millisecond)
			func() {
				// acquire mutex
				s.mutex.Lock()
				defer s.mutex.Unlock()

				// check active
				if !s.active {
					return
				}

				// emit queue
				s.info.Int(s.queue)
			}()
		}
	}()

	// create stream
	s.stream = streamy.NewStream(streamy.Config{
		Broker: broker,
		Name:   name,
		Base:   base,
		Info: func(str string) {
			// handle message
			switch str {
			case "online":
				s.state.Int(1)
			case "offline":
				s.state.Int(0)
			default:
				max.Log(str)
			}
		},
		SampleRate:  44100,
		BitRate:     16,
		DeviceQueue: 16,
	})

	return true
}

func (s *sender) Handle(_ int, msg string, args []max.Atom) {
	// acquire mutex
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// handle message
	switch msg {
	case "connect":
		s.stream.Connect()
	case "reset":
		s.stream.Reset()
	case "disconnect":
		s.stream.Disconnect()
	default:
		max.Error("unknown message %s", msg)
	}
}

// TODO: Buffer some data before start writing?

func (s *sender) Process(in, _ []float64) {
	// acquire mutex
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// convert data
	data := make([]int, len(in))
	for i, sample := range in {
		data[i] = int(sample * math.MaxInt16)
	}

	// write data
	queue, _ := s.stream.Write(data)

	// set queue
	s.queue = int64(queue)
}

func (s *sender) Free() {
	// acquire mutex
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// set active
	s.active = false

	// disconnect
	s.stream.Disconnect()
}

func main() {
	// initialize Max class
	max.Register("streamy", &sender{})
}
