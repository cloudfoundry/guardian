package testhelpers

import (
	"fmt"
	"net"
	"sync"

	"github.com/cloudfoundry/dropsonde/dropsonde_unmarshaller"
	"github.com/cloudfoundry/sonde-go/events"
	"google.golang.org/protobuf/proto"
)

type FakeMetron struct {
	port                  uint16
	connection            net.PacketConn
	dropsondeUnmarshaller *dropsonde_unmarshaller.DropsondeUnmarshaller
	valueMetrics          map[string][]events.ValueMetric
	counterEvents         map[string][]events.CounterEvent
	errors                []events.Error
	stopped               bool
	mtx                   sync.RWMutex
}

func NewFakeMetron(port uint16) *FakeMetron {
	return &FakeMetron{
		port:                  port,
		dropsondeUnmarshaller: dropsonde_unmarshaller.NewDropsondeUnmarshaller(),
		mtx:                   sync.RWMutex{},
		valueMetrics:          make(map[string][]events.ValueMetric),
		counterEvents:         make(map[string][]events.CounterEvent),
		errors:                make([]events.Error, 0),
	}
}

func (m *FakeMetron) Listen() error {
	addr := fmt.Sprintf("localhost:%d", m.port)
	connection, err := net.ListenPacket("udp4", addr)
	if err != nil {
		return err
	}
	m.connection = connection

	return nil
}

func (m *FakeMetron) Run() error {
	readBuffer := make([]byte, 65535) //buffer with size = max theoretical UDP size
	for {
		readCount, _, err := m.connection.ReadFrom(readBuffer)
		if err != nil || m.isStopped() {
			return nil
		}
		if err != nil {
			return err
		}
		readData := make([]byte, readCount) //pass on buffer in size only of read data
		copy(readData, readBuffer[:readCount])

		// unmarshal
		envelope, err := m.dropsondeUnmarshaller.UnmarshallMessage(readData)
		if err != nil {
			return err
		}

		m.mtx.Lock()
		switch *envelope.EventType {
		case events.Envelope_ValueMetric:
			metric := *envelope.GetValueMetric()
			key := metric.GetName()
			m.valueMetrics[key] = append(m.valueMetrics[key], events.ValueMetric{
				Name:  proto.String(metric.GetName()),
				Value: proto.Float64(metric.GetValue()),
				Unit:  proto.String(metric.GetUnit()),
			})

		case events.Envelope_Error:
			err := *envelope.GetError()
			m.errors = append(m.errors, events.Error{
				Source:  proto.String(err.GetSource()),
				Code:    proto.Int32(err.GetCode()),
				Message: proto.String(err.GetMessage()),
			})

		case events.Envelope_CounterEvent:
			counter := *envelope.GetCounterEvent()
			name := counter.GetName()
			m.counterEvents[name] = append(m.counterEvents[name], events.CounterEvent{
				Name:  proto.String(counter.GetName()),
				Delta: proto.Uint64(counter.GetDelta()),
				Total: proto.Uint64(counter.GetTotal()),
			})
		}
		m.mtx.Unlock()
	}
}

func (m *FakeMetron) isStopped() bool {
	m.mtx.RLock()
	defer m.mtx.RUnlock()
	return m.stopped
}

func (m *FakeMetron) Stop() error {
	m.mtx.Lock()
	defer m.mtx.Unlock()
	m.stopped = true

	return m.connection.Close()
}

func (m *FakeMetron) ValueMetricsFor(key string) []events.ValueMetric {
	m.mtx.RLock()
	defer m.mtx.RUnlock()

	metrics, ok := m.valueMetrics[key]
	if !ok {
		return []events.ValueMetric{}
	}

	return metrics
}

func (m *FakeMetron) CounterEvents(name string) []events.CounterEvent {
	m.mtx.RLock()
	defer m.mtx.RUnlock()

	counters, ok := m.counterEvents[name]
	if !ok {
		return []events.CounterEvent{}
	}

	return counters
}

func (m *FakeMetron) Errors() []events.Error {
	m.mtx.RLock()
	defer m.mtx.RUnlock()

	return m.errors
}
