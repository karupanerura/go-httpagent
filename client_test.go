package httpagent

import (
	"context"
	"net/http"
	"testing"

	mockhttp "github.com/karupanerura/go-mock-http-response"
)

func TestClientFunc(t *testing.T) {
	client := ClientFunc(func(req *http.Request) (*http.Response, error) {
		res := mockhttp.NewResponseMock(http.StatusOK, map[string]string{
			"Content-Type": "text/plain",
		}, []byte("OK")).MakeResponse(req)
		return res, nil
	})

	req := &http.Request{}
	res, _ := client.Do(req)
	if res.StatusCode != http.StatusOK {
		t.Errorf("Unexpected response: %#v", res)
	}
}

func TestContextWithClient(t *testing.T) {
	t.Run("OK", func(t *testing.T) {
		ctx1 := context.Background()
		if client := contextClient(ctx1); client != nil {
			t.Errorf("Initial context is invalid: %#v", ctx1)
		}

		ctx2 := ContextWithClient(ctx1, http.DefaultClient)
		if ctx1 == ctx2 {
			t.Errorf("Same context is returned: %#v", ctx2)
		}

		client := contextClient(ctx2)
		if httpClient, ok := client.(*http.Client); !ok {
			t.Errorf("Client is invalid: %#v", client)
		} else if httpClient != http.DefaultClient {
			t.Errorf("Client is invalid: %#v", httpClient)
		}
	})

	t.Run("Panic", func(t *testing.T) {
		ctx := context.Background()
		if ctx == nil {
			t.Fatal("ctx is should not be nil")
		}

		defer func() {
			if r := recover(); r == nil {
				t.Errorf("The code did not panic")
			}
		}()
		ContextWithClient(ctx, nil)
	})
}
