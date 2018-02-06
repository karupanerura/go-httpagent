package httpagent

import (
	"context"
	"net/http"
	"time"
)

var DefaultAgent = NewAgent(http.DefaultClient)

func NewAgent(client Client) *Agent {
	header := http.Header{}
	return &Agent{
		Client:        client,
		DefaultHeader: header,
		RequestHooks:  NewRequestHooks(),
		ResponseHooks: NewResponseHooks(),
	}
}

type Agent struct {
	Client         Client
	DefaultTimeout time.Duration
	DefaultHeader  http.Header
	RequestHooks   *RequestHooks
	ResponseHooks  *ResponseHooks
}

func nop() {}

func (a *Agent) Do(req *http.Request) (*http.Response, error) {
	// apply default headers
	err := (&RequestHeaderHook{Header: a.DefaultHeader, SkipIfExists: true}).Do(req)
	if err != nil {
		return nil, err
	}

	// do request hooks
	err = a.RequestHooks.Do(req)
	if err != nil {
		return nil, err
	}

	// get client
	client := contextClient(req.Context())
	if client == nil {
		client = a.Client
	}

	// apply timeout
	cancel := nop
	if a.DefaultTimeout > 0 {
		var ctx context.Context
		ctx, cancel = context.WithTimeout(req.Context(), a.DefaultTimeout)
		req = req.WithContext(ctx)
	}

	// do request
	res, err := client.Do(req)
	cancel()
	if err != nil {
		return nil, err
	}

	// do response hooks
	err = a.ResponseHooks.Do(res)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (a *Agent) WithClient(client Client) *Agent {
	return &Agent{
		Client:         client,
		DefaultTimeout: a.DefaultTimeout,
		DefaultHeader:  copyHeader(a.DefaultHeader),
		RequestHooks:   a.RequestHooks.Clone(),
		ResponseHooks:  a.ResponseHooks.Clone(),
	}
}

func copyHeader(src http.Header) (dst http.Header) {
	dst = make(http.Header, len(src))
	for k := range src {
		if len(src) == 0 {
			continue
		}

		dst[k] = make([]string, len(src))
		copy(dst[k], src[k])
	}

	return
}
