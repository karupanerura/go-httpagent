package httpagent

import (
	"io"
	"net/http"
	"net/http/httputil"
)

// RequestHook is an interface for hooks that can modify or inspect HTTP requests
// before they are sent. Hooks can be used for logging, adding headers, authentication,
// request validation, or any other pre-request processing.
//
// The Do method is called with the request and should return an error if the
// request should not proceed. If an error is returned, the request is aborted.
type RequestHook interface {
	Do(*http.Request) error
}

// RequestHookFunc is a function type that implements the RequestHook interface.
// It allows any function with the signature func(*http.Request) error to be used
// as a RequestHook, enabling easy creation of custom hooks without defining new types.
//
// Example:
//
//	logHook := httpagent.RequestHookFunc(func(req *http.Request) error {
//	    log.Printf("Request to: %s", req.URL)
//	    return nil
//	})
type RequestHookFunc func(*http.Request) error

// Do implements the RequestHook interface for RequestHookFunc.
// It executes the underlying function to process the request.
func (h RequestHookFunc) Do(req *http.Request) error {
	return h(req)
}

// NopRequestHook is a no-operation RequestHook that does nothing.
// It can be used as a placeholder or to conditionally disable hooks.
// When appending to RequestHooks, NopRequestHook is automatically skipped for optimization.
var NopRequestHook = nopRequestHook{}

type nopRequestHook struct{}

func (h nopRequestHook) Do(_ *http.Request) error {
	return nil
}

// RequestHooks is a collection of RequestHook instances that are executed sequentially.
// It provides methods to add, execute, and manage multiple request hooks.
// Hooks are executed in the order they were added.
type RequestHooks struct {
	hooks []RequestHook
}

// NewRequestHooks creates a new RequestHooks instance with the provided hooks.
// The hooks are added in the order they appear in the arguments.
//
// Example:
//
//	hooks := httpagent.NewRequestHooks(hook1, hook2, hook3)
func NewRequestHooks(hooks ...RequestHook) (h *RequestHooks) {
	h = &RequestHooks{}
	for _, hook := range hooks {
		h.Append(hook)
	}
	return
}

// Append adds a RequestHook to the end of the hooks collection.
// 
// Panics if the provided hook is nil.
//
// The method performs two optimizations:
//  1. NopRequestHook instances are skipped and not added to the collection
//  2. If the hook is itself a RequestHooks instance, its hooks are flattened
//     and added individually to avoid nested hook collections
func (h *RequestHooks) Append(hook RequestHook) {
	if hook == nil {
		panic("nil hook")
	}

	// Optimize: skip to add nop
	if hook == NopRequestHook {
		return
	}

	// Optimize: flatten
	if hooks, ok := hook.(*RequestHooks); ok {
		h.hooks = append(h.hooks, hooks.hooks...)
		return
	}

	h.hooks = append(h.hooks, hook)
}

// Do executes all hooks in the collection sequentially on the given request.
// If any hook returns an error, execution stops immediately and that error is returned.
// Subsequent hooks are not executed after an error occurs.
//
// Returns nil if all hooks execute successfully, or the first error encountered.
func (h *RequestHooks) Do(req *http.Request) (err error) {
	for _, hook := range h.hooks {
		err = hook.Do(req)
		if err != nil {
			return
		}
	}
	return
}

// Len returns the number of hooks in the collection.
func (h *RequestHooks) Len() int {
	return len(h.hooks)
}

// Clone creates a deep copy of the RequestHooks instance.
// The returned copy contains the same hooks but in a new slice,
// so modifications to one instance won't affect the other.
func (h *RequestHooks) Clone() *RequestHooks {
	hooks := make([]RequestHook, len(h.hooks))
	copy(hooks, h.hooks)
	return &RequestHooks{hooks: hooks}
}

// RequestDumperHook is a RequestHook that dumps the HTTP request to a writer.
// It uses httputil.DumpRequestOut to capture the complete request including headers and body.
// This is useful for debugging and logging purposes.
//
// Example:
//
//	dumper := &httpagent.RequestDumperHook{Writer: os.Stderr}
//	agent.RequestHooks.Append(dumper)
type RequestDumperHook struct {
	Writer io.Writer
}

// Do implements the RequestHook interface for RequestDumperHook.
// It dumps the request to the configured Writer and returns any write errors.
// The request dump includes the request line, headers, and body.
func (h *RequestDumperHook) Do(req *http.Request) error {
	dump, err := httputil.DumpRequestOut(req, true)
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

// RequestHeaderHook is a RequestHook that adds or sets headers on HTTP requests.
// It provides flexible header manipulation with options to add, set, or conditionally set headers.
//
// Fields:
//   - Header: The headers to apply to the request
//   - Add: If true, headers are added (allowing multiple values); if false, headers are set (replacing existing values)
//   - SkipIfExists: If true, headers are only set if they don't already exist in the request
//
// Example:
//
//	hook := &httpagent.RequestHeaderHook{
//	    Header: http.Header{"User-Agent": []string{"my-app/1.0"}},
//	    SkipIfExists: true,
//	}
type RequestHeaderHook struct {
	Header       http.Header
	Add          bool
	SkipIfExists bool
}

// Do implements the RequestHook interface for RequestHeaderHook.
// It applies the configured headers to the request according to the Add and SkipIfExists settings.
//
// The method iterates through all headers in the Hook's Header field:
//   - If SkipIfExists is true and the header already exists in the request, it is skipped
//   - If Add is true, the header is added to the request (allowing multiple values)
//   - If Add is false, the header is set in the request (replacing any existing value)
func (h *RequestHeaderHook) Do(req *http.Request) error {
	for key := range h.Header {
		if h.SkipIfExists {
			if _, ok := req.Header[http.CanonicalHeaderKey(key)]; ok {
				continue
			}
		}

		value := h.Header.Get(key)
		if h.Add {
			req.Header.Add(key, value)
		} else {
			req.Header.Set(key, value)
		}
	}

	return nil
}
