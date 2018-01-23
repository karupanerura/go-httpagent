package httpagent

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewAgent(t *testing.T) {
	client := &http.Client{}
	agent := NewAgent(client)

	if agent.Client != client {
		t.Errorf("agent.Client should be argument's client, but got: %#v", agent.Client)
	}
	if agent.Timeout != 0 {
		t.Errorf("agent.Timeout should be zero, but got: %#v", agent.Timeout)
	}
	if len(agent.DefaultHeader) != 0 {
		t.Errorf("agent.DefaultHeader should be empty, but got: %#v", agent.DefaultHeader)
	}

	if agent.RequestHooks.Len() != 1 {
		t.Errorf("agent.ResponseHooks should have a hook, but got: %#v", agent.RequestHooks)
	} else if _, ok := agent.RequestHooks.hooks[0].(*RequestHeaderHook); !ok {
		t.Errorf("agent.ResponseHooks should have a RequestHeaderHook and it should apply agent.DefaultHeader, but got: %#v", agent.RequestHooks)
	}

	if agent.ResponseHooks.Len() != 0 {
		t.Errorf("agent.ResponseHooks should be empty, but got: %#v", agent.ResponseHooks)
	}
}

func TestAgentDo(t *testing.T) {
	t.Run("Passthrough", func(t *testing.T) {
		ts := setupTestServer(t)
		defer ts.Close()

		req, err := http.NewRequest(http.MethodGet, ts.URL, nil)
		if err != nil {
			t.Fatal(err)
		}

		shouldBeOK(t, DefaultAgent, req, 1)
	})

	t.Run("WithDefaultHeader", func(t *testing.T) {
		ts := setupTestServer(t)
		defer ts.Close()

		agent := NewAgent(http.DefaultClient)
		agent.DefaultHeader.Set("Test-Increment", "100")

		t.Run("OK", func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, ts.URL, nil)
			if err != nil {
				t.Fatal(err)
			}

			shouldBeOK(t, agent, req, 101)
		})

		t.Run("NoOverwrite", func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, ts.URL, nil)
			if err != nil {
				t.Fatal(err)
			}

			req.Header.Set("Test-Increment", "1000")
			shouldBeOK(t, agent, req, 1102)
		})
	})

	t.Run("WithTimeout", func(t *testing.T) {
		agent := NewAgent(http.DefaultClient)
		agent.Timeout = 3 * time.Second

		t.Run("OK", func(t *testing.T) {
			ts := setupTestServer(t)
			defer ts.Close()

			req, err := http.NewRequest(http.MethodGet, ts.URL, nil)
			if err != nil {
				t.Fatal(err)
			}
			shouldBeOK(t, agent, req, 1)
		})

		t.Run("NG", func(t *testing.T) {
			ts := setupTestServer(t)
			defer ts.Close()

			req, err := http.NewRequest(http.MethodGet, ts.URL, nil)
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Test-Sleep", "4")
			shouldBeError(t, agent, req, &url.Error{Err: context.DeadlineExceeded})
		})

		t.Run("NestedContext", func(t *testing.T) {
			ts := setupTestServer(t)
			defer ts.Close()

			req, err := http.NewRequest(http.MethodGet, ts.URL, nil)
			if err != nil {
				t.Fatal(err)
			}

			ctx := context.Background()
			ctx, cancel := context.WithTimeout(ctx, 1*time.Second)
			defer cancel()

			req.Header.Set("Test-Sleep", "2")

			before := time.Now()
			shouldBeError(t, agent, req.WithContext(ctx), &url.Error{Err: context.DeadlineExceeded})
			after := time.Now()

			if d := after.Sub(before); d > time.Second*3 {
				t.Errorf("Unexpected timeout: %#v", d)
			}
		})
	})

	t.Run("RequestHook", func(t *testing.T) {
		t.Run("OK", func(t *testing.T) {
			ts := setupTestServer(t)
			defer ts.Close()

			var called int

			agent := NewAgent(http.DefaultClient)
			agent.RequestHooks.Append(RequestHookFunc(func(req *http.Request) error {
				called++
				return nil
			}))

			req, err := http.NewRequest(http.MethodGet, ts.URL, nil)
			if err != nil {
				t.Fatal(err)
			}
			shouldBeOK(t, agent, req, 1)
			if called != 1 {
				t.Errorf("Request hook should be called at once, but it called %d times", called)
			}
		})

		t.Run("NG", func(t *testing.T) {
			ts := setupTestServer(t)
			defer ts.Close()

			expectedErr := errors.New("oops")

			agent := NewAgent(http.DefaultClient)
			agent.RequestHooks.Append(RequestHookFunc(func(req *http.Request) error {
				return expectedErr
			}))

			req, err := http.NewRequest(http.MethodGet, ts.URL, nil)
			if err != nil {
				t.Fatal(err)
			}

			shouldBeError(t, agent, req, expectedErr)
			shouldBeOK(t, DefaultAgent, req, 1)
		})
	})

	t.Run("ResponseHook", func(t *testing.T) {
		t.Run("OK", func(t *testing.T) {
			ts := setupTestServer(t)
			defer ts.Close()

			var called int

			agent := NewAgent(http.DefaultClient)
			agent.ResponseHooks.Append(ResponseHookFunc(func(res *http.Response) error {
				called++
				return nil
			}))

			req, err := http.NewRequest(http.MethodGet, ts.URL, nil)
			if err != nil {
				t.Fatal(err)
			}
			shouldBeOK(t, agent, req, 1)
			if called != 1 {
				t.Errorf("Request hook should be called at once, but it called %d times", called)
			}
		})

		t.Run("NG", func(t *testing.T) {
			ts := setupTestServer(t)
			defer ts.Close()

			expectedErr := errors.New("oops")

			agent := NewAgent(http.DefaultClient)
			agent.ResponseHooks.Append(ResponseHookFunc(func(res *http.Response) error {
				return expectedErr
			}))

			req, err := http.NewRequest(http.MethodGet, ts.URL, nil)
			if err != nil {
				t.Fatal(err)
			}

			shouldBeError(t, agent, req, expectedErr)
			shouldBeOK(t, DefaultAgent, req, 2)
		})
	})
}

func setupTestServer(t *testing.T) *httptest.Server {
	var c int32
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&c, 1)
		if slp := r.Header.Get("Test-Sleep"); slp != "" {
			t.Logf("Test-Sleep: %s", slp)
			i, err := strconv.Atoi(slp)
			if err != nil {
				t.Fatal(err)
			}
			time.Sleep(time.Duration(i) * time.Second)
		}
		if incr := r.Header.Get("Test-Increment"); incr != "" {
			t.Logf("Test-Increment: %s", incr)
			i, err := strconv.Atoi(incr)
			if err != nil {
				t.Fatal(err)
			}
			count = atomic.AddInt32(&c, int32(i))
		}

		w.Header().Set("Foo", "Bar")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "OK: count=%d", count)
	}))
}

func shouldBeOK(t *testing.T, agent *Agent, req *http.Request, count int) {
	res, err := agent.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusOK {
		t.Errorf("Unexpected response: %#v", res)
	}

	b, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	if s := string(b); s != fmt.Sprintf("OK: count=%d", count) {
		t.Errorf("Count should be %d, but got: %s", count, s)
	}
}

func shouldBeError(t *testing.T, agent *Agent, req *http.Request, expectedErr error) {
	res, err := agent.Do(req)
	if res != nil {
		t.Errorf("Should be no response, but got: %#v", res)
	}
	if exuerr, ok := expectedErr.(*url.Error); ok {
		if uerr := err.(*url.Error); !(ok && uerr.Err == exuerr.Err) {
			t.Errorf("Unexpected error is occurred: %#v", err)
		}
	} else {
		if err != expectedErr {
			t.Errorf("Unexpected error is occurred: %#v", err)
		}
	}
}
