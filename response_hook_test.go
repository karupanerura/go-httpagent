package httpagent

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	mockhttp "github.com/karupanerura/go-mock-http-response"
)

func mustNewResponse(t *testing.T, method, u string, body io.Reader) *http.Response {
	req, err := http.NewRequest(method, u, body)
	if err != nil {
		t.Fatal(err)
	}

	return mockhttp.NewResponseMock(http.StatusOK, map[string]string{
		"Content-Type": "text/plain",
	}, []byte("OK")).MakeResponse(req)
}

func cloneResponse(r1 *http.Response) *http.Response {
	r2 := http.Response(*r1)
	return &r2
}

func TestNopResponseHook(t *testing.T) {
	origRes := mustNewResponse(t, http.MethodGet, "http://example.com/", nil)

	res := cloneResponse(origRes)
	err := NopResponseHook.Do(res)
	if err != nil {
		t.Error(err)
	}

	if diff := cmp.Diff(res, origRes, cmpopts.IgnoreUnexported(http.Response{}, http.Request{}, bytes.Reader{})); diff != "" {
		t.Errorf("Shoud no diff, but got: %s", diff)
	}
}

func TestResponseHookFunc(t *testing.T) {
	res := mustNewResponse(t, http.MethodGet, "http://example.com/", nil)

	hook := ResponseHookFunc(func(res *http.Response) error {
		res.Header.Set("Foo", "bar")
		return nil
	})
	err := hook.Do(res)
	if err != nil {
		t.Error(err)
	}

	if foo := res.Header.Get("Foo"); foo != "bar" {
		t.Errorf("Foo header should be bar, but got: %#v", res.Header)
	}
}

func TestResponseHooks(t *testing.T) {
	t.Run("Simple", func(t *testing.T) {
		res := mustNewResponse(t, http.MethodGet, "http://example.com/", nil)

		hooks := NewResponseHooks(
			NopResponseHook,
			ResponseHookFunc(func(res *http.Response) error {
				res.Header.Set("Foo", "bar")
				return nil
			}),
		)
		hooks.Append(
			NewResponseHooks(
				ResponseHookFunc(func(res *http.Response) error {
					res.Header.Set("Bar", "bar")
					return nil
				}),
				ResponseHookFunc(func(res *http.Response) error {
					res.Header.Set("Foo", "foo")
					return nil
				}),
				NopResponseHook,
			),
		)
		if hooks.Len() != 3 {
			t.Errorf("Should flatten hooks, but got: %#v", hooks)
		}

		err := hooks.Do(res)
		if err != nil {
			t.Error(err)
		}

		if foo := res.Header.Get("Foo"); foo != "foo" {
			t.Errorf("Foo header should be foo, but got: %#v", res.Header)
		}
		if bar := res.Header.Get("Bar"); bar != "bar" {
			t.Errorf("Bar header should be bar, but got: %#v", res.Header)
		}
	})

	t.Run("Error", func(t *testing.T) {
		res := mustNewResponse(t, http.MethodGet, "http://example.com/", nil)
		mockErr := fmt.Errorf("mock error")

		hooks := NewResponseHooks(
			ResponseHookFunc(func(res *http.Response) error {
				res.Header.Set("Foo", "bar")
				return nil
			}),
		)
		hooks.Append(
			NewResponseHooks(
				ResponseHookFunc(func(res *http.Response) error {
					return mockErr
				}),
				ResponseHookFunc(func(res *http.Response) error {
					res.Header.Set("Foo", "foo")
					return nil
				}),
			),
		)
		if hooks.Len() != 3 {
			t.Errorf("Should flatten hooks, but got: %#v", hooks)
		}

		err := hooks.Do(res)
		if err != mockErr {
			t.Error(err)
		}

		if foo := res.Header.Get("Foo"); foo != "bar" {
			t.Errorf("Foo header should be bar, but got: %#v", res.Header)
		}
		if bar := res.Header.Get("Bar"); bar != "" {
			t.Errorf("Bar header should be empty, but got: %#v", res.Header)
		}
	})
}

func TestResponseDumperHook(t *testing.T) {
	buf := &bytes.Buffer{}
	err := (&ResponseDumperHook{Writer: buf}).Do(mustNewResponse(t, http.MethodGet, "http://example.com/", nil))
	if err != nil {
		t.Error(err)
	}

	if dump := buf.String(); !strings.HasPrefix(dump, "HTTP/1.0 200 OK") {
		t.Errorf("Unexpected dump: %s", dump)
	}
}
