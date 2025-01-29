package nerd

import "github.com/containerd/containerd/v2/client"

type NerdStopper struct {
	client *containerd.Client
}

func NewNerdStopper(client *client.Client) *NerdStopper {
	return &NerdStopper{client: client}
}

func (s NerdStopper) Stop() error {
	return s.client.Close()
}
