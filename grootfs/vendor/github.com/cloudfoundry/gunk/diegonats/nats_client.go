package diegonats

import (
	"time"

	"github.com/apcera/nats"
)

type NATSClient interface {
	Connect(urls []string) (chan struct{}, error)
	Close()
	Ping() bool
	Unsubscribe(sub *nats.Subscription) error

	// Via apcera/nats.Conn
	Publish(subject string, data []byte) error
	PublishRequest(subj, reply string, data []byte) error
	Request(subj string, data []byte, timeout time.Duration) (m *nats.Msg, err error)
	Subscribe(subject string, handler nats.MsgHandler) (*nats.Subscription, error)
	QueueSubscribe(subject, queue string, handler nats.MsgHandler) (*nats.Subscription, error)
}

type natsClient struct {
	*nats.Conn
}

func NewClient() NATSClient {
	return &natsClient{}
}

func (nc *natsClient) Connect(urls []string) (chan struct{}, error) {
	options := nats.DefaultOptions
	options.Servers = urls
	options.ReconnectWait = 500 * time.Millisecond
	options.MaxReconnect = -1

	closedChan := make(chan struct{})
	options.ClosedCB = func(*nats.Conn) {
		close(closedChan)
	}

	natsConnection, err := options.Connect()
	if err != nil {
		return nil, err
	}

	nc.Conn = natsConnection
	return closedChan, nil
}

func (nc *natsClient) Close() {
	if nc.Conn != nil {
		nc.Conn.Close()
	}
}

func (c *natsClient) Ping() bool {
	err := c.FlushTimeout(500 * time.Millisecond)
	return err == nil
}

func (nc *natsClient) Unsubscribe(sub *nats.Subscription) error {
	return sub.Unsubscribe()
}
