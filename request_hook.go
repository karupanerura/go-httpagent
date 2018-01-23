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

type DumperRequestHook struct {
	Writer io.Writer
}

func (h *DumperRequestHook) Do(req *http.Request) error {
	dump, err := httputil.DumpRequestOut(req, true)
	if err != nil {
		return err
	}

	var n, wrote int
	for wrote < len(dump) {
		n, err = h.Writer.Write(dump)
		wrote += n
	}
	return err
}

type RequestHeaderHook struct {
	Header http.Header
}

func (h *RequestHeaderHook) Do(req *http.Request) error {
	for key := range h.Header {
		value := h.Header.Get(key)
		req.Header.Set(key, value)
	}

	return nil
}
