# go-httpagent

[![Build Status](https://travis-ci.org/karupanerura/go-httpagent.svg?branch=master)](https://travis-ci.org/karupanerura/go-httpagent)
[![codecov](https://codecov.io/gh/karupanerura/go-httpagent/branch/master/graph/badge.svg)](https://codecov.io/gh/karupanerura/go-httpagent)
[![GoDoc](https://godoc.org/github.com/karupanerura/go-httpagent?status.svg)](http://godoc.org/github.com/karupanerura/go-httpagent)

HTTP Agent for go programming language.
Provides hooks, timeout, and default header for http.Client and any other similar interface.

## Example

```go
agent := httpagent.NewAgent(http.DefaultClient)
agent.DefaultTimeout = 10 * time.Second
agent.DefaultHeader.Set("User-Agent", "go-httpagent/0.1")
agent.RequestHooks.Append(&httpagent.RequestDumperHook{Writer: os.Stderr})
agent.ResponseHooks.Append(&httpagent.ResponseDumperHook{Writer: os.Stderr})

req, _ := http.NewRequest("GET", "https://karupas.org/", nil)

ctx := context.Background()
req = req.WithContext(req)
res, err := agent.Do(req)
```
