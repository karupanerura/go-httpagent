package httpagent

import (
	"context"
	"net/http"
)

// Client is an interface that defines the contract for executing HTTP requests.
// It is compatible with http.Client and allows for custom HTTP client implementations.
// This interface enables dependency injection and makes testing easier.
type Client interface {
	Do(*http.Request) (*http.Response, error)
}

// ClientFunc is a function type that implements the Client interface.
// It allows any function with the signature func(*http.Request) (*http.Response, error)
// to be used as a Client, enabling easy creation of mock or custom clients.
//
// Example:
//
//	mockClient := httpagent.ClientFunc(func(req *http.Request) (*http.Response, error) {
//	    return &http.Response{StatusCode: 200}, nil
//	})
type ClientFunc func(*http.Request) (*http.Response, error)

var _ Client = ClientFunc(func(*http.Request) (*http.Response, error) {
	return nil, nil
})

// Do implements the Client interface for ClientFunc.
// It executes the underlying function to perform the HTTP request.
func (a ClientFunc) Do(req *http.Request) (*http.Response, error) {
	return a(req)
}

type clientContextKeyType struct{}

var clientContextKey = clientContextKeyType{}

// ContextWithClient returns a new context with the specified client attached.
// The client can later be retrieved using contextClient (internal function).
//
// This allows for per-request client customization without modifying the Agent.
// If the Agent's Do method is called with a request that has a context containing
// a client, that client will be used instead of the Agent's default client.
//
// Panics if the provided client is nil.
//
// Example:
//
//	customClient := &http.Client{Timeout: 5 * time.Second}
//	ctx := httpagent.ContextWithClient(context.Background(), customClient)
//	req = req.WithContext(ctx)
//	res, err := agent.Do(req)  // Uses customClient instead of agent.Client
func ContextWithClient(ctx context.Context, client Client) context.Context {
	if client == nil {
		panic("nil client")
	}
	return context.WithValue(ctx, clientContextKey, client)
}

// contextClient retrieves a client from the context, if one exists.
// Returns nil if no client is stored in the context.
// This is an internal helper function used by Agent.Do.
func contextClient(ctx context.Context) Client {
	client, ok := ctx.Value(clientContextKey).(Client)
	if !ok {
		return nil
	}
	return client
}
