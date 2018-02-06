package httpagent

import (
	"io"
	"net/http"
	"net/http/httputil"
)

type ResponseHook interface {
	Do(*http.Response) error
}

type ResponseHookFunc func(*http.Response) error

func (h ResponseHookFunc) Do(req *http.Response) error {
	return h(req)
}

var NopResponseHook = nopResponseHook{}

type nopResponseHook struct{}

func (h nopResponseHook) Do(_ *http.Response) error {
	return nil
}

type ResponseHooks struct {
	hooks []ResponseHook
}

func NewResponseHooks(hooks ...ResponseHook) (h *ResponseHooks) {
	h = &ResponseHooks{}
	for _, hook := range hooks {
		h.Append(hook)
	}
	return
}

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

func (h *ResponseHooks) Do(req *http.Response) (err error) {
	for _, hook := range h.hooks {
		err = hook.Do(req)
		if err != nil {
			return
		}
	}
	return
}

func (h *ResponseHooks) Len() int {
	return len(h.hooks)
}

func (h *ResponseHooks) Clone() *ResponseHooks {
	hooks := make([]ResponseHook, len(h.hooks))
	copy(hooks, h.hooks)
	return &ResponseHooks{hooks: hooks}
}

type ResponseDumperHook struct {
	Writer io.Writer
}

func (h *ResponseDumperHook) Do(res *http.Response) error {
	dump, err := httputil.DumpResponse(res, true)
	if err != nil {
		return err
	}

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
