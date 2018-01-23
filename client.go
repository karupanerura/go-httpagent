package httpagent

import "net/http"

type Client interface {
	Do(*http.Request) (*http.Response, error)
}

type ClientFunc func(*http.Request) (*http.Response, error)

var _ Client = ClientFunc(func(*http.Request) (*http.Response, error) {
	return nil, nil
})

func (a ClientFunc) Do(req *http.Request) (*http.Response, error) {
	return a(req)
}
