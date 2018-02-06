package httpagent

import (
	"io"
	"net/http"
	"net/http/httputil"
)

type RequestHook interface {
	Do(*http.Request) error
}

type RequestHookFunc func(*http.Request) error

func (h RequestHookFunc) Do(req *http.Request) error {
	return h(req)
}

var NopRequestHook = nopRequestHook{}

type nopRequestHook struct{}

func (h nopRequestHook) Do(_ *http.Request) error {
	return nil
}

type RequestHooks struct {
	hooks []RequestHook
}

func NewRequestHooks(hooks ...RequestHook) (h *RequestHooks) {
	h = &RequestHooks{}
	for _, hook := range hooks {
		h.Append(hook)
	}
	return
}

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

func (h *RequestHooks) Do(req *http.Request) (err error) {
	for _, hook := range h.hooks {
		err = hook.Do(req)
		if err != nil {
			return
		}
	}
	return
}

func (h *RequestHooks) Len() int {
	return len(h.hooks)
}

func (h *RequestHooks) Clone() *RequestHooks {
	hooks := make([]RequestHook, len(h.hooks))
	copy(hooks, h.hooks)
	return &RequestHooks{hooks: hooks}
}

type RequestDumperHook struct {
	Writer io.Writer
}

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

type RequestHeaderHook struct {
	Header       http.Header
	Add          bool
	SkipIfExists bool
}

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
