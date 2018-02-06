package httpagent

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	mockhttp "github.com/karupanerura/go-mock-http-response"
)

func TestNewAgent(t *testing.T) {
	client := &http.Client{}
	agent := NewAgent(client)

	if agent.Client != client {
		t.Errorf("agent.Client should be argument's client, but got: %#v", agent.Client)
	}
	if agent.DefaultTimeout != 0 {
		t.Errorf("agent.DefaultTimeout should be zero, but got: %#v", agent.DefaultTimeout)
	}
	if len(agent.DefaultHeader) != 0 {
		t.Errorf("agent.DefaultHeader should be empty, but got: %#v", agent.DefaultHeader)
	}

	if agent.RequestHooks.Len() != 0 {
		t.Errorf("agent.ResponseHooks should have a hook, but got: %#v", agent.RequestHooks)
	}
	if agent.ResponseHooks.Len() != 0 {
		t.Errorf("agent.ResponseHooks should be empty, but got: %#v", agent.ResponseHooks)
	}
}

func TestAgentWithClient(t *testing.T) {
	h1 := http.Header{}
	h2 := h1
	if reflect.ValueOf(h1).Pointer() != reflect.ValueOf(h2).Pointer() {
		t.Fatal("http.Header is not pointer type now")
	}

	client1 := &http.Client{}
	agent1 := NewAgent(client1)

	client2 := &http.Client{}
	agent2 := agent1.WithClient(client2)

	if agent2.Client == agent1.Client {
		t.Errorf("agent.Client should be changed, but got: %#v", agent2.Client)
	}
	if reflect.ValueOf(agent2.DefaultHeader).Pointer() == reflect.ValueOf(agent1.DefaultHeader).Pointer() {
		// SEE ALSO (Japanese...): https://qiita.com/karupanerura/items/03d6766fd8568c15fc90
		t.Errorf("agent.DefaultHeader should be changed, but got: %#v", agent2.DefaultHeader)
	}
	if agent2.RequestHooks == agent1.RequestHooks {
		t.Errorf("agent.RequestHooks should be changed, but got: %#v", agent2.RequestHooks)
	}
	if agent2.ResponseHooks == agent1.ResponseHooks {
		t.Errorf("agent.ResponseHooks should be changed, but got: %#v", agent2.ResponseHooks)
	}
}

func TestAgentDo(t *testing.T) {
	t.Run("Passthrough", func(t *testing.T) {
		ts := setupTestServer(t)
		defer ts.Close()

		req := mustNewRequest(t, http.MethodGet, ts.URL, nil)
		shouldBeOK(t, DefaultAgent, req, 1)
	})

	t.Run("WithContextClient", func(t *testing.T) {
		ts := setupTestServer(t)
		defer ts.Close()

		client := mockhttp.NewResponseMock(http.StatusAccepted, map[string]string{
			"Content-Type": "text/plain",
		}, []byte("Accepted")).MakeClient()

		req := mustNewRequest(t, http.MethodGet, ts.URL, nil)
		req = req.WithContext(ContextWithClient(req.Context(), client))
		res, err := DefaultAgent.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		if res.StatusCode != http.StatusAccepted {
			t.Errorf("Unexpected response: %#v", res)
		}
	})

	t.Run("WithDefaultHeader", func(t *testing.T) {
		ts := setupTestServer(t)
		defer ts.Close()

		agent := NewAgent(http.DefaultClient)
		agent.DefaultHeader.Set("Test-Increment", "100")

		t.Run("OK", func(t *testing.T) {
			req := mustNewRequest(t, http.MethodGet, ts.URL, nil)
			shouldBeOK(t, agent, req, 101)
		})

		t.Run("NoOverwrite", func(t *testing.T) {
			req := mustNewRequest(t, http.MethodGet, ts.URL, nil)
			req.Header.Set("Test-Increment", "1000")
			shouldBeOK(t, agent, req, 1102)
		})
	})

	t.Run("WithDefaultTimeout", func(t *testing.T) {
		agent := NewAgent(http.DefaultClient)
		agent.DefaultTimeout = 3 * time.Second

		t.Run("OK", func(t *testing.T) {
			ts := setupTestServer(t)
			defer ts.Close()

			req := mustNewRequest(t, http.MethodGet, ts.URL, nil)
			shouldBeOK(t, agent, req, 1)
		})

		t.Run("NG", func(t *testing.T) {
			ts := setupTestServer(t)
			defer ts.Close()

			req := mustNewRequest(t, http.MethodGet, ts.URL, nil)
			req.Header.Set("Test-Sleep", "4")
			shouldBeError(t, agent, req, &url.Error{Err: context.DeadlineExceeded})
		})

		t.Run("NestedContext", func(t *testing.T) {
			ts := setupTestServer(t)
			defer ts.Close()

			ctx := context.Background()
			ctx, cancel := context.WithTimeout(ctx, 1*time.Second)
			defer cancel()

			req := mustNewRequest(t, http.MethodGet, ts.URL, nil)
			req.Header.Set("Test-Sleep", "2")
			req = req.WithContext(ctx)

			before := time.Now()
			shouldBeError(t, agent, req, &url.Error{Err: context.DeadlineExceeded})
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

			req := mustNewRequest(t, http.MethodGet, ts.URL, nil)
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

			req := mustNewRequest(t, http.MethodGet, ts.URL, nil)
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

			req := mustNewRequest(t, http.MethodGet, ts.URL, nil)
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

			req := mustNewRequest(t, http.MethodGet, ts.URL, nil)
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

func mustNewRequest(t *testing.T, method, u string, body io.Reader) *http.Request {
	req, err := http.NewRequest(method, u, body)
	if err != nil {
		t.Fatal(err)
	}

	return req
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
