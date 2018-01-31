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
		RequestHooks: NewRequestHooks(
			&RequestHeaderHook{Header: header, SkipIfExists: true},
		),
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
	// do request hooks
	err := a.RequestHooks.Do(req)
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
