package diegonats

import (
	"time"

	"github.com/nats-io/nats"
)

type NATSClient interface {
	Connect(urls []string) (chan struct{}, error)
	SetPingInterval(interval time.Duration)
	Close()
	Ping() bool
	Unsubscribe(sub *nats.Subscription) error

	// Via nats-io/nats.Conn
	Publish(subject string, data []byte) error
	PublishRequest(subj, reply string, data []byte) error
	Request(subj string, data []byte, timeout time.Duration) (m *nats.Msg, err error)
	Subscribe(subject string, handler nats.MsgHandler) (*nats.Subscription, error)
	QueueSubscribe(subject, queue string, handler nats.MsgHandler) (*nats.Subscription, error)
}

type natsClient struct {
	*nats.Conn
	pingInterval time.Duration
}

func NewClient() NATSClient {
	return &natsClient{
		pingInterval: nats.DefaultPingInterval,
	}
}

func (nc *natsClient) SetPingInterval(interval time.Duration) {
	nc.pingInterval = interval
}

func (nc *natsClient) Connect(urls []string) (chan struct{}, error) {
	options := nats.DefaultOptions
	options.Servers = urls
	options.ReconnectWait = 500 * time.Millisecond
	options.MaxReconnect = -1
	options.PingInterval = nc.pingInterval

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
