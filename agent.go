// Package httpagent provides an HTTP agent with hooks, timeout, and default header support
// for http.Client and any other similar interface.
//
// The main component is the Agent, which wraps an HTTP client and provides:
//   - Request hooks for modifying requests before sending
//   - Response hooks for processing responses after receiving
//   - Default headers that are applied to all requests
//   - Configurable timeout for all requests
//
// Example usage:
//
//	agent := httpagent.NewAgent(http.DefaultClient)
//	agent.DefaultTimeout = 10 * time.Second
//	agent.DefaultHeader.Set("User-Agent", "my-app/1.0")
//	agent.RequestHooks.Append(&httpagent.RequestDumperHook{Writer: os.Stderr})
//
//	req, _ := http.NewRequest("GET", "https://example.com", nil)
//	res, err := agent.Do(req)
package httpagent

import (
	"context"
	"net/http"
	"time"
)

// DefaultAgent is a pre-configured Agent instance using http.DefaultClient.
// It can be used for simple use cases where customization is not required.
var DefaultAgent = NewAgent(http.DefaultClient)

// NewAgent creates a new Agent with the specified HTTP client.
// The returned Agent is initialized with empty hooks, headers, and no default timeout.
//
// The client parameter must implement the Client interface, which is compatible with
// http.Client and any custom HTTP client implementation.
func NewAgent(client Client) *Agent {
	header := http.Header{}
	return &Agent{
		Client:        client,
		DefaultHeader: header,
		RequestHooks:  NewRequestHooks(),
		ResponseHooks: NewResponseHooks(),
	}
}

// Agent is an HTTP client wrapper that provides additional functionality
// such as request/response hooks, default headers, and request timeouts.
//
// Fields:
//   - Client: The underlying HTTP client used to execute requests
//   - DefaultTimeout: Timeout duration applied to all requests (0 means no timeout)
//   - DefaultHeader: Headers automatically added to all requests (unless already present)
//   - RequestHooks: Hooks executed before sending the request
//   - ResponseHooks: Hooks executed after receiving the response
type Agent struct {
	Client         Client
	DefaultTimeout time.Duration
	DefaultHeader  http.Header
	RequestHooks   *RequestHooks
	ResponseHooks  *ResponseHooks
}

func nop() {}

// Do executes an HTTP request using the configured agent.
//
// The execution follows these steps:
//  1. Apply default headers (skipping headers that already exist in the request)
//  2. Execute all request hooks
//  3. Determine which client to use (context client or agent's default client)
//  4. Apply default timeout if configured
//  5. Execute the HTTP request
//  6. Execute all response hooks
//
// If a client is stored in the request's context using ContextWithClient,
// that client will be used instead of the Agent's default Client.
//
// Returns the HTTP response and any error encountered during execution.
// Errors can occur during hook execution, timeout, or the actual HTTP request.
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

// WithClient creates a new Agent with the specified client, while copying
// all other configuration (timeout, headers, and hooks) from the current Agent.
//
// This is useful for creating agent variants that use different HTTP clients
// but share the same configuration. All hooks, headers, and settings are
// deeply cloned to prevent unintended sharing between agents.
//
// Returns a new Agent instance with the specified client.
func (a *Agent) WithClient(client Client) *Agent {
	return &Agent{
		Client:         client,
		DefaultTimeout: a.DefaultTimeout,
		DefaultHeader:  a.DefaultHeader.Clone(),
		RequestHooks:   a.RequestHooks.Clone(),
		ResponseHooks:  a.ResponseHooks.Clone(),
	}
}
