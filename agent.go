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
			&RequestHeaderHook{Header: header},
		),
		ResponseHooks: NewResponseHooks(),
	}
}

type Agent struct {
	Client        Client
	Timeout       time.Duration
	DefaultHeader http.Header
	RequestHooks  *RequestHooks
	ResponseHooks *ResponseHooks
}

func nop() {}

func (a *Agent) Do(req *http.Request) (*http.Response, error) {
	err := a.RequestHooks.Do(req)
	if err != nil {
		return nil, err
	}

	cancel := nop
	if a.Timeout > 0 {
		var ctx context.Context
		ctx, cancel = context.WithTimeout(req.Context(), a.Timeout)
		req = req.WithContext(ctx)
	}
	res, err := a.Client.Do(req)
	cancel()
	if err != nil {
		return nil, err
	}

	err = a.ResponseHooks.Do(res)
	if err != nil {
		return nil, err
	}

	return res, nil
}
