# go-httpagent

```go
agent := httpagent.NewAgent(http.DefaultClient)
agent.DefaultHeader.Set("User-Agent", "go-httpagent/0.1")
agent.RequestHooks.Append(&httpagent.RequestDumperHook{Writer: os.Stderr})
agent.ResponseHooks.Append(&httpagent.ResponseDumperHook{Writer: os.Stderr})

req, _ := http.NewRequest("GET", "https://karupas.org/", nil)

ctx := context.Background()
req = req.WithContext(req)
res, err := agent.Do(req)
```
