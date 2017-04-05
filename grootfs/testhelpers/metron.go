package testhelpers

import (
	"fmt"
	"net"
	"sync"

	"github.com/cloudfoundry/dropsonde/dropsonde_unmarshaller"
	"github.com/cloudfoundry/sonde-go/events"
)

type FakeMetron struct {
	port                  uint16
	connection            net.PacketConn
	dropsondeUnmarshaller *dropsonde_unmarshaller.DropsondeUnmarshaller
	valueMetrics          map[string][]events.ValueMetric
	errors                []events.Error
	stopped               bool
	mtx                   sync.RWMutex
}

func NewFakeMetron(port uint16) *FakeMetron {
	return &FakeMetron{
		port: port,
		dropsondeUnmarshaller: dropsonde_unmarshaller.NewDropsondeUnmarshaller(nil),
		mtx:          sync.RWMutex{},
		valueMetrics: make(map[string][]events.ValueMetric),
		errors:       make([]events.Error, 0),
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
		if err != nil && m.isStopped() {
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

		switch *envelope.EventType {
		case events.Envelope_ValueMetric:
			m.mtx.Lock()
			metric := *envelope.ValueMetric
			key := *metric.Name
			m.valueMetrics[key] = append(m.valueMetrics[key], metric)
			m.mtx.Unlock()

		case events.Envelope_Error:
			m.mtx.Lock()
			err := *envelope.Error
			m.errors = append(m.errors, err)
			m.mtx.Unlock()
		}
	}
}

func (m *FakeMetron) isStopped() bool {
	m.mtx.RLock()
	defer m.mtx.RUnlock()
	return m.stopped
}

func (m *FakeMetron) Stop() error {
	m.mtx.RLock()
	defer m.mtx.RUnlock()
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

func (m *FakeMetron) Errors() []events.Error {
	m.mtx.RLock()
	defer m.mtx.RUnlock()

	return m.errors
}
