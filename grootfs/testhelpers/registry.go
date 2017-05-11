package testhelpers

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"regexp"
	"sync"

	"github.com/onsi/gomega/ghttp"
)

type blobHandler struct {
	httpHandler http.HandlerFunc
	order       int
}

type FakeRegistry struct {
	ActualRegistryURL   *url.URL
	blobHandlers        map[string]blobHandler
	blobRequestsCounter map[string]int
	revProxy            *httputil.ReverseProxy
	server              *ghttp.Server
	mutex               *sync.RWMutex
}

func NewFakeRegistry(actualRegistryURL *url.URL) *FakeRegistry {
	return &FakeRegistry{
		ActualRegistryURL:   actualRegistryURL,
		blobHandlers:        make(map[string]blobHandler),
		blobRequestsCounter: make(map[string]int),
		mutex:               &sync.RWMutex{},
	}
}

func (r *FakeRegistry) Start() error {
	r.revProxy = httputil.NewSingleHostReverseProxy(r.ActualRegistryURL)
	// Dockerhub returns 503 if the host is set to localhost as it happens with
	// the reverse proxy
	oldDirector := r.revProxy.Director
	r.revProxy.Director = func(req *http.Request) {
		oldDirector(req)
		req.Host = r.ActualRegistryURL.Host
	}

	r.server = ghttp.NewTLSServer()

	blobRegexp, err := regexp.Compile(`\/v2\/.*\/blobs\/sha256:[a-f0-9]*`)
	if err != nil {
		return err
	}
	r.server.RouteToHandler("GET", blobRegexp, r.serveBlob)

	ourRegexp, err := regexp.Compile(`.*`)
	if err != nil {
		return err
	}
	r.server.RouteToHandler("GET", ourRegexp, r.revProxy.ServeHTTP)

	return nil
}

func (r *FakeRegistry) serveBlob(rw http.ResponseWriter, req *http.Request) {
	re, err := regexp.Compile(`\/v2\/.*\/blobs\/sha256:([a-f0-9]*)`)
	if err != nil {
		r.revProxy.ServeHTTP(rw, req)
		return
	}

	match := re.FindStringSubmatch(req.URL.Path)
	if match == nil {
		r.revProxy.ServeHTTP(rw, req)
		return
	}

	digest := match[1]

	r.mutex.Lock()
	r.blobRequestsCounter[digest]++
	r.mutex.Unlock()

	r.mutex.RLock()
	defer r.mutex.RUnlock()
	handler, ok := r.blobHandlers[digest]

	if !ok {
		r.revProxy.ServeHTTP(rw, req)
		return
	}

	if handler.order == 0 || r.blobRequestsCounter[digest] == handler.order {
		handler.httpHandler(rw, req)
	} else {
		r.revProxy.ServeHTTP(rw, req)
	}
}

func (r *FakeRegistry) Stop() {
	r.server.Close()
}

func (r *FakeRegistry) Addr() string {
	return r.server.Addr()
}

func (r *FakeRegistry) WhenGettingBlob(digest string, order int, httpHandler http.HandlerFunc) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.blobHandlers[digest] = blobHandler{
		httpHandler: httpHandler,
		order:       order,
	}
}

func (r *FakeRegistry) RequestedBlobs() []string {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	blobDigests := []string{}
	for digest, _ := range r.blobRequestsCounter {
		blobDigests = append(blobDigests, digest)
	}

	return blobDigests
}
