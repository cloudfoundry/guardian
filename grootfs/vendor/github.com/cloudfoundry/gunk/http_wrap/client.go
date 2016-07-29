package http_wrap

import os_http "net/http"

/* overloads native http lib with your own custom one */
func NewClientFrom(httpClient *os_http.Client) Client {
	return &_client{httpClient}
}

/* builds new client with golang native http client */
func NewClient() Client {
	return &_client{&os_http.Client{}}
}

type _client struct {
	Delegate *os_http.Client
}

func (client *_client) Do(req *os_http.Request) (resp *os_http.Response, err error) {
	return client.Delegate.Do(req)
}
