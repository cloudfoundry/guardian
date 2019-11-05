package nerd

import "github.com/containerd/containerd"

type NerdStopper struct {
	client *containerd.Client
}

func NewNerdStopper(client *containerd.Client) *NerdStopper {
	return &NerdStopper{client: client}
}

func (s NerdStopper) Stop() error {
	return s.client.Close()
}
