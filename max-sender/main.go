package main

import (
	"math"

	"github.com/256dpi/max-go"

	"streamy"
)

type sender struct {
	signal *max.Inlet
	cmd    *max.Inlet
	state  *max.Outlet
	queue  *max.Outlet
	stream *streamy.Stream
}

func (s *sender) Init(obj *max.Object, args []max.Atom) bool {
	// declare inlets
	s.signal = obj.Inlet(max.Signal, "input", true)
	s.cmd = obj.Inlet(max.Any, "commands", true)

	// declare outlets
	s.state = obj.Outlet(max.Int, "connection state")
	s.queue = obj.Outlet(max.Int, "device queue")

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
		Queue: func(length int) {
			// set length
			s.queue.Int(int64(length))
		},
		SampleRate:  44100,
		BitRate:     16,
		DeviceQueue: 16,
	})

	return true
}

func (s *sender) Handle(_ int, msg string, args []max.Atom) {
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
	// convert data
	data := make([]int, len(in))
	for i, sample := range in {
		data[i] = int(sample * math.MaxInt16)
	}

	// write data
	s.stream.Write(data)
}

func (s *sender) Free() {
	// disconnect
	s.stream.Disconnect()
}

func main() {
	// initialize Max class
	max.Register("streamy", &sender{})
}
