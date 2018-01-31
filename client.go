package httpagent

import (
	"context"
	"net/http"
)

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

type clientContextKeyType struct{}

var clientContextKey = clientContextKeyType{}

func ContextWithClient(ctx context.Context, client Client) context.Context {
	if client == nil {
		panic("nil client")
	}
	return context.WithValue(ctx, clientContextKey, client)
}

func contextClient(ctx context.Context) Client {
	client, ok := ctx.Value(clientContextKey).(Client)
	if !ok {
		return nil
	}
	return client
}
