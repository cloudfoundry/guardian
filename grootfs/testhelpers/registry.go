package testhelpers

import (
	"errors"
	"net/http"
	"net/http/httputil"
	"net/url"
	"regexp"
	"sync"
	"time"

	"github.com/onsi/gomega/ghttp"
)

type blobHandler struct {
	httpHandler http.HandlerFunc
	order       int
}

type FakeRegistry struct {
	ActualRegistryURL        *url.URL
	blobHandlers             map[string]blobHandler
	blobRequestsCounter      map[string]int
	blobRegexp               *regexp.Regexp
	manifestRegexp           *regexp.Regexp
	failNextBlobRequests     int
	failNextManifestRequests int
	forceTokenAuthError      bool
	revProxy                 *httputil.ReverseProxy
	server                   *ghttp.Server
	mutex                    *sync.RWMutex
	stopped                  chan struct{}
}

func NewFakeRegistry(actualRegistryURL *url.URL) *FakeRegistry {
	return &FakeRegistry{
		ActualRegistryURL:   actualRegistryURL,
		blobHandlers:        make(map[string]blobHandler),
		blobRequestsCounter: make(map[string]int),
		mutex:               &sync.RWMutex{},
		stopped:             make(chan struct{}, 1),
	}
}

func (r *FakeRegistry) Start() {
	r.revProxy = httputil.NewSingleHostReverseProxy(r.ActualRegistryURL)
	// Dockerhub returns 503 if the host is set to localhost as it happens with
	// the reverse proxy
	oldDirector := r.revProxy.Director
	r.revProxy.Director = func(req *http.Request) {
		oldDirector(req)
		req.Host = r.ActualRegistryURL.Host
	}

	r.server = ghttp.NewTLSServer()

	r.blobRegexp = regexp.MustCompile(`\/v2\/.*\/blobs\/(.*)`)
	r.server.RouteToHandler("GET", r.blobRegexp, r.serveBlob)

	r.manifestRegexp = regexp.MustCompile(`\/v2\/.*\/manifests\/(.*)`)
	r.server.RouteToHandler("GET", r.manifestRegexp, r.serveManifest)

	ourRegexp := regexp.MustCompile(`token.*`)
	r.server.RouteToHandler("GET", ourRegexp, r.serveToken)

	ourRegexp = regexp.MustCompile(`.*`)
	r.server.RouteToHandler("GET", ourRegexp, r.serveHTTP)
}

func (r *FakeRegistry) FailNextBlobRequests(n int) {
	r.failNextBlobRequests = n
}

func (r *FakeRegistry) FailNextManifestRequests(n int) {
	r.failNextManifestRequests = n
}

func (r *FakeRegistry) ForceTokenAuthError() {
	r.mutex.Lock()
	r.forceTokenAuthError = true
	r.mutex.Unlock()
}

func (r *FakeRegistry) serveToken(rw http.ResponseWriter, req *http.Request) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	if r.forceTokenAuthError {
		rw.WriteHeader(http.StatusUnauthorized)
		return
	}

	r.revProxy.ServeHTTP(rw, req)
}

func (r *FakeRegistry) serveHTTP(rw http.ResponseWriter, req *http.Request) {
	match, _ := regexp.MatchString(`^\/v2\/$`, req.RequestURI)
	r.mutex.Lock()
	defer r.mutex.Unlock()
	if r.forceTokenAuthError && match {
		rw.Header().Add("Www-Authenticate", `Bearer realm="https://`+r.Addr()+`/token",service="registry.docker.io"`)
		rw.WriteHeader(http.StatusUnauthorized)
		return
	}

	r.revProxy.ServeHTTP(rw, req)
}

func (r *FakeRegistry) serveManifest(rw http.ResponseWriter, req *http.Request) {
	if r.failNextManifestRequests > 0 {
		r.failNextManifestRequests--
		rw.WriteHeader(http.StatusTeapot)
		rw.Write([]byte("null"))
		return
	}

	r.revProxy.ServeHTTP(rw, req)
}

func (r *FakeRegistry) serveBlob(rw http.ResponseWriter, req *http.Request) {
	if r.failNextBlobRequests > 0 {
		r.failNextBlobRequests--
		rw.WriteHeader(http.StatusTeapot)
		rw.Write([]byte("null"))
		return
	}

	match := r.blobRegexp.FindStringSubmatch(req.URL.Path)
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
	r.stopped <- struct{}{}
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
	for digest := range r.blobRequestsCounter {
		blobDigests = append(blobDigests, digest)
	}

	return blobDigests
}

func (r *FakeRegistry) DelegateToActualRegistry(rw http.ResponseWriter, req *http.Request) {
	r.revProxy.ServeHTTP(rw, req)
}

func (r *FakeRegistry) Delay(timeout time.Duration) error {
	timer := time.NewTimer(timeout)
	select {
	case <-timer.C:
		return nil
	case <-r.stopped:
		return errors.New("Server has been stopped while delaying")
	}
}
