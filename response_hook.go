package httpagent

import (
	"io"
	"net/http"
	"net/http/httputil"
)

// ResponseHook is an interface for hooks that can process or inspect HTTP responses
// after they are received. Hooks can be used for logging, response validation,
// error handling, metrics collection, or any other post-response processing.
//
// The Do method is called with the response and should return an error if the
// response processing fails. If an error is returned, it will be returned to the caller.
type ResponseHook interface {
	Do(*http.Response) error
}

// ResponseHookFunc is a function type that implements the ResponseHook interface.
// It allows any function with the signature func(*http.Response) error to be used
// as a ResponseHook, enabling easy creation of custom hooks without defining new types.
//
// Example:
//
//	logHook := httpagent.ResponseHookFunc(func(res *http.Response) error {
//	    log.Printf("Response status: %d", res.StatusCode)
//	    return nil
//	})
type ResponseHookFunc func(*http.Response) error

// Do implements the ResponseHook interface for ResponseHookFunc.
// It executes the underlying function to process the response.
func (h ResponseHookFunc) Do(req *http.Response) error {
	return h(req)
}

// NopResponseHook is a no-operation ResponseHook that does nothing.
// It can be used as a placeholder or to conditionally disable hooks.
// When appending to ResponseHooks, NopResponseHook is automatically skipped for optimization.
var NopResponseHook = nopResponseHook{}

type nopResponseHook struct{}

func (h nopResponseHook) Do(_ *http.Response) error {
	return nil
}

// ResponseHooks is a collection of ResponseHook instances that are executed sequentially.
// It provides methods to add, execute, and manage multiple response hooks.
// Hooks are executed in the order they were added.
type ResponseHooks struct {
	hooks []ResponseHook
}

// NewResponseHooks creates a new ResponseHooks instance with the provided hooks.
// The hooks are added in the order they appear in the arguments.
//
// Example:
//
//	hooks := httpagent.NewResponseHooks(hook1, hook2, hook3)
func NewResponseHooks(hooks ...ResponseHook) (h *ResponseHooks) {
	h = &ResponseHooks{}
	for _, hook := range hooks {
		h.Append(hook)
	}
	return
}

// Append adds a ResponseHook to the end of the hooks collection.
// 
// Panics if the provided hook is nil.
//
// The method performs two optimizations:
//  1. NopResponseHook instances are skipped and not added to the collection
//  2. If the hook is itself a ResponseHooks instance, its hooks are flattened
//     and added individually to avoid nested hook collections
func (h *ResponseHooks) Append(hook ResponseHook) {
	if hook == nil {
		panic("nil hook")
	}

	// Optimize: skip to add nop
	if hook == NopResponseHook {
		return
	}

	// Optimize: flatten
	if hooks, ok := hook.(*ResponseHooks); ok {
		h.hooks = append(h.hooks, hooks.hooks...)
		return
	}

	h.hooks = append(h.hooks, hook)
}

// Do executes all hooks in the collection sequentially on the given response.
// If any hook returns an error, execution stops immediately and that error is returned.
// Subsequent hooks are not executed after an error occurs.
//
// Returns nil if all hooks execute successfully, or the first error encountered.
func (h *ResponseHooks) Do(req *http.Response) (err error) {
	for _, hook := range h.hooks {
		err = hook.Do(req)
		if err != nil {
			return
		}
	}
	return
}

// Len returns the number of hooks in the collection.
func (h *ResponseHooks) Len() int {
	return len(h.hooks)
}

// Clone creates a deep copy of the ResponseHooks instance.
// The returned copy contains the same hooks but in a new slice,
// so modifications to one instance won't affect the other.
func (h *ResponseHooks) Clone() *ResponseHooks {
	hooks := make([]ResponseHook, len(h.hooks))
	copy(hooks, h.hooks)
	return &ResponseHooks{hooks: hooks}
}

// ResponseDumperHook is a ResponseHook that dumps the HTTP response to a writer.
// It uses httputil.DumpResponse to capture the complete response including headers and body.
// This is useful for debugging and logging purposes.
//
// Example:
//
//	dumper := &httpagent.ResponseDumperHook{Writer: os.Stderr}
//	agent.ResponseHooks.Append(dumper)
type ResponseDumperHook struct {
	Writer io.Writer
}

// Do implements the ResponseHook interface for ResponseDumperHook.
// It dumps the response to the configured Writer and returns any write errors.
// The response dump includes the status line, headers, and body.
func (h *ResponseDumperHook) Do(res *http.Response) error {
	dump, err := httputil.DumpResponse(res, true)
	if err != nil {
		return err
	}
	dump = append(dump, '\n')

	var n, wrote int
	for wrote < len(dump) {
		n, err = h.Writer.Write(dump)
		if err != nil {
			return err
		}

		wrote += n
	}
	return err
}
