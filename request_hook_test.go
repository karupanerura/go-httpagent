package httpagent

import (
	"bytes"
	"fmt"
	"net/http"
	"net/textproto"
	"net/url"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func cloneRequest(r1 *http.Request) *http.Request {
	r2 := http.Request(*r1)
	u2 := url.URL(*r1.URL)
	r2.URL = &u2
	return &r2
}

func TestNopRequestHook(t *testing.T) {
	origReq := mustNewRequest(t, http.MethodGet, "http://example.com/", nil)

	req := cloneRequest(origReq)
	err := NopRequestHook.Do(req)
	if err != nil {
		t.Error(err)
	}

	if diff := cmp.Diff(req, origReq, cmpopts.IgnoreUnexported(http.Request{})); diff != "" {
		t.Errorf("Shoud no diff, but got: %s", diff)
	}
}

func TestRequestHookFunc(t *testing.T) {
	req := mustNewRequest(t, http.MethodGet, "http://example.com/", nil)

	hook := RequestHookFunc(func(req *http.Request) error {
		req.Header.Set("Foo", "bar")
		return nil
	})
	err := hook.Do(req)
	if err != nil {
		t.Error(err)
	}

	if foo := req.Header.Get("Foo"); foo != "bar" {
		t.Errorf("Foo header should be bar, but got: %#v", req.Header)
	}
}

func TestRequestHooks(t *testing.T) {
	t.Run("Simple", func(t *testing.T) {
		req := mustNewRequest(t, http.MethodGet, "http://example.com/", nil)

		hooks := NewRequestHooks(
			NopRequestHook,
			RequestHookFunc(func(req *http.Request) error {
				req.Header.Set("Foo", "bar")
				return nil
			}),
		)
		hooks.Append(
			NewRequestHooks(
				RequestHookFunc(func(req *http.Request) error {
					req.Header.Set("Bar", "bar")
					return nil
				}),
				RequestHookFunc(func(req *http.Request) error {
					req.Header.Set("Foo", "foo")
					return nil
				}),
				NopRequestHook,
			),
		)
		if hooks.Len() != 3 {
			t.Errorf("Should flatten hooks, but got: %#v", hooks)
		}

		err := hooks.Do(req)
		if err != nil {
			t.Error(err)
		}

		if foo := req.Header.Get("Foo"); foo != "foo" {
			t.Errorf("Foo header should be foo, but got: %#v", req.Header)
		}
		if bar := req.Header.Get("Bar"); bar != "bar" {
			t.Errorf("Bar header should be bar, but got: %#v", req.Header)
		}
	})

	t.Run("Error", func(t *testing.T) {
		req := mustNewRequest(t, http.MethodGet, "http://example.com/", nil)
		mockErr := fmt.Errorf("mock error")

		hooks := NewRequestHooks(
			RequestHookFunc(func(req *http.Request) error {
				req.Header.Set("Foo", "bar")
				return nil
			}),
		)
		hooks.Append(
			NewRequestHooks(
				RequestHookFunc(func(req *http.Request) error {
					return mockErr
				}),
				RequestHookFunc(func(req *http.Request) error {
					req.Header.Set("Foo", "foo")
					return nil
				}),
			),
		)
		if hooks.Len() != 3 {
			t.Errorf("Should flatten hooks, but got: %#v", hooks)
		}

		err := hooks.Do(req)
		if err != mockErr {
			t.Error(err)
		}

		if foo := req.Header.Get("Foo"); foo != "bar" {
			t.Errorf("Foo header should be bar, but got: %#v", req.Header)
		}
		if bar := req.Header.Get("Bar"); bar != "" {
			t.Errorf("Bar header should be empty, but got: %#v", req.Header)
		}
	})
}

func TestRequestDumperHook(t *testing.T) {
	buf := &bytes.Buffer{}
	err := (&RequestDumperHook{Writer: buf}).Do(mustNewRequest(t, http.MethodGet, "http://example.com/", nil))
	if err != nil {
		t.Error(err)
	}

	if dump := buf.String(); !strings.HasPrefix(dump, "GET /") {
		t.Errorf("Unexpected dump: %s", dump)
	}
}

func TestRequestHeaderHook(t *testing.T) {
	t.Run("Set", func(t *testing.T) {
		hook := &RequestHeaderHook{Header: http.Header{}}
		hook.Header.Set("Foo", "hoge")
		hook.Header.Set("Bar", "fuga")

		req := mustNewRequest(t, http.MethodGet, "http://example.com/", nil)
		req.Header.Set("Bar", "piyo")
		err := hook.Do(req)
		if err != nil {
			t.Error(err)
		}

		if req.Header.Get("Foo") != "hoge" {
			t.Errorf("Foo header should be hoge, but got: %#v", req.Header)
		}
		if req.Header.Get("Bar") != "fuga" {
			t.Errorf("Bar header should be fuga, but got: %#v", req.Header)
		}
	})

	t.Run("Add", func(t *testing.T) {
		hook := &RequestHeaderHook{Header: http.Header{}, Add: true}
		hook.Header.Set("Foo", "hoge")
		hook.Header.Set("Bar", "fuga")

		req := mustNewRequest(t, http.MethodGet, "http://example.com/", nil)
		req.Header.Set("Bar", "piyo")
		err := hook.Do(req)
		if err != nil {
			t.Error(err)
		}

		if req.Header.Get("Foo") != "hoge" {
			t.Errorf("Foo header should be hoge, but got: %#v", req.Header)
		}
		if req.Header.Get("Bar") != "piyo" {
			t.Errorf("Bar header should be piyo, but got: %#v", req.Header)
		}
		if bar := req.Header[textproto.CanonicalMIMEHeaderKey("Bar")]; !cmp.Equal(bar, []string{"piyo", "fuga"}) {
			t.Errorf(`Bar header should be ["piyo", "fuga"], but got:  %#v`, bar)
		}
	})

	t.Run("SkipIfExists", func(t *testing.T) {
		hook := &RequestHeaderHook{Header: http.Header{}, SkipIfExists: true}
		hook.Header.Set("Foo", "hoge")
		hook.Header.Set("Bar", "fuga")

		req := mustNewRequest(t, http.MethodGet, "http://example.com/", nil)
		req.Header.Set("Bar", "piyo")
		err := hook.Do(req)
		if err != nil {
			t.Error(err)
		}

		if req.Header.Get("Foo") != "hoge" {
			t.Errorf("Foo header should be hoge, but got: %#v", req.Header)
		}
		if req.Header.Get("Bar") != "piyo" {
			t.Errorf("Bar header should be piyo, but got: %#v", req.Header)
		}
		if bar := req.Header[textproto.CanonicalMIMEHeaderKey("Bar")]; !cmp.Equal(bar, []string{"piyo"}) {
			t.Errorf(`Bar header should be ["piyo"], but got:  %#v`, bar)
		}
	})
}
