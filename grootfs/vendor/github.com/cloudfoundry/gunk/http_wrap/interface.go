/*
Package http_wrap wraps golang http in an interface.

With this you can mock http system calls. Please add to this interface
and implementations with other calls that are to be mocked.

The fake/mock implementation is in an aptly named subdirectory.
*/
package http_wrap

import os_http "net/http"

//go:generate counterfeiter -o httpfakes/fake_http_client.go . Client

/*
Wraps http client side calls.
*/
type Client interface {
	Do(req *os_http.Request) (resp *os_http.Response, err error)
}
