package test_server

import (
	"encoding/base64"
	"fmt"
	. "github.com/onsi/gomega"
	"io/ioutil"
	"net/http"
)

func CombineHandlers(handlers ...http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		for _, handler := range handlers {
			handler(w, req)
		}
	}
}

func VerifyRequest(method string, path string, rawQuery ...string) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		Expect(req.Method).To(Equal(method), "Method mismatch")
		Expect(req.URL.Path).To(Equal(path), "Path mismatch")
		if len(rawQuery) > 0 {
			Expect(req.URL.RawQuery).To(Equal(rawQuery[0]), "RawQuery mismatch")
		}
	}
}

func VerifyContentType(contentType string) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		Expect(req.Header.Get("Content-Type")).To(Equal(contentType))
	}
}

func VerifyBasicAuth(username string, password string) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		auth := req.Header.Get("Authorization")
		decoded, err := base64.StdEncoding.DecodeString(auth[6:])
		Expect(err).NotTo(HaveOccurred())

		Expect(string(decoded)).To(Equal(fmt.Sprintf("%s:%s", username, password)), "Authorization mismatch")
	}
}

func VerifyHeader(header http.Header) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		for key, values := range header {
			key = http.CanonicalHeaderKey(key)
			Expect(req.Header[key]).To(Equal(values), "Header mismatch for key: %s", key)
		}
	}
}

func VerifyJSON(expectedJSON string) http.HandlerFunc {
	return CombineHandlers(
		VerifyContentType("application/json"),
		func(w http.ResponseWriter, req *http.Request) {
			body, err := ioutil.ReadAll(req.Body)
			req.Body.Close()
			Expect(err).NotTo(HaveOccurred())
			Expect(body).To(MatchJSON(expectedJSON), "JSON Mismatch")
		},
	)
}

func Respond(statusCode int, body string) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(statusCode)
		w.Write([]byte(body))
	}
}

func RespondPtr(statusCode *int, body *string) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(*statusCode)
		w.Write([]byte(*body))
	}
}
