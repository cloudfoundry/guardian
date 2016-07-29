package test_server

import (
	"fmt"
	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega/format"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"sync"
)

func new() *Server {
	return &Server{
		AllowUnhandledRequests:     false,
		UnhandledRequestStatusCode: http.StatusInternalServerError,
		writeLock:                  &sync.Mutex{},
	}
}
func New() *Server {
	s := new()
	s.HTTPTestServer = httptest.NewServer(s)
	return s
}

func NewTLS() *Server {
	s := new()
	s.HTTPTestServer = httptest.NewTLSServer(s)
	return s
}

type Server struct {
	HTTPTestServer *httptest.Server

	receivedRequests []*http.Request
	requestHandlers  []http.HandlerFunc

	AllowUnhandledRequests     bool
	UnhandledRequestStatusCode int
	writeLock                  *sync.Mutex
	calls                      int
}

func (s *Server) URL() string {
	return s.HTTPTestServer.URL
}

func (s *Server) Close() {
	server := s.HTTPTestServer
	s.HTTPTestServer = nil
	server.Close()
}

func (s *Server) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	s.writeLock.Lock()
	defer s.writeLock.Unlock()

	if s.calls < len(s.requestHandlers) {
		s.requestHandlers[s.calls](w, req)
	} else {
		if s.AllowUnhandledRequests {
			ioutil.ReadAll(req.Body)
			req.Body.Close()
			w.WriteHeader(s.UnhandledRequestStatusCode)
		} else {
			ginkgo.Fail(fmt.Sprintf("Received unhandled request:\n%s", format.Object(req, 1)))
		}
	}
	s.receivedRequests = append(s.receivedRequests, req)
	s.calls++
}

func (s *Server) ReceivedRequestsCount() int {
	s.writeLock.Lock()
	defer s.writeLock.Unlock()

	return len(s.receivedRequests)
}

func (s *Server) Append(handlers ...http.HandlerFunc) {
	s.writeLock.Lock()
	defer s.writeLock.Unlock()

	s.requestHandlers = append(s.requestHandlers, handlers...)
}

func (s *Server) Set(index int, handler http.HandlerFunc) {
	s.writeLock.Lock()
	defer s.writeLock.Unlock()

	s.requestHandlers[index] = handler
}

func (s *Server) Get(index int) http.HandlerFunc {
	s.writeLock.Lock()
	defer s.writeLock.Unlock()

	return s.requestHandlers[index]
}

func (s *Server) Wrap(index int, handler http.HandlerFunc) {
	existingHandler := s.Get(index)
	s.Set(index, CombineHandlers(existingHandler, handler))
}
