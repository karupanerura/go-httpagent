package httpagent

import (
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
