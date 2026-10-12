//go:build js && wasm

package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"syscall/js"
	"time"
)

// The js/wasm entry accepts HTTP envelopes and an AbortSignal from its host.
// It uses only standard runtime APIs; authentication and provider routing belong
// to the host adapter. All MCP and rendering behavior stays in the shared Go code.
func main() {
	app, err := newApplication(config{Format: "png", ASCIIMode: "extended", Timeout: 30 * time.Second})
	if err != nil {
		panic(err)
	}
	handler := httpHandler(app.server(), "http")
	js.Global().Set("d2MCP", js.FuncOf(func(_ js.Value, args []js.Value) any {
		input, signal := args[0].String(), args[1]
		executor := js.FuncOf(func(_ js.Value, callbacks []js.Value) any {
			resolve := callbacks[0]
			go func() {
				var envelope struct {
					Method, URL, Body string
					Headers           map[string]string
				}
				response := httptest.NewRecorder()
				if err := json.Unmarshal([]byte(input), &envelope); err != nil {
					http.Error(response, "invalid HTTP envelope", http.StatusBadRequest)
				} else {
					ctx, cancel := context.WithCancel(context.Background())
					defer cancel()
					abort := js.FuncOf(func(js.Value, []js.Value) any { cancel(); return nil })
					defer abort.Release()
					signal.Call("addEventListener", "abort", abort)
					defer signal.Call("removeEventListener", "abort", abort)
					if signal.Get("aborted").Bool() {
						cancel()
					}
					request, err := http.NewRequestWithContext(ctx, envelope.Method, envelope.URL, strings.NewReader(envelope.Body))
					if err != nil {
						http.Error(response, "invalid HTTP request", http.StatusBadRequest)
					} else {
						for key, value := range envelope.Headers {
							request.Header.Set(key, value)
						}
						if request.URL.Scheme == "https" {
							request.TLS = &tls.ConnectionState{}
						}
						handler.ServeHTTP(response, request)
					}
				}
				result := struct {
					Status  int         `json:"status"`
					Headers http.Header `json:"headers"`
					Body    string      `json:"body"`
				}{response.Code, response.Header(), response.Body.String()}
				encoded, _ := json.Marshal(result)
				resolve.Invoke(string(encoded))
			}()
			return nil
		})
		defer executor.Release()
		return js.Global().Get("Promise").New(executor)
	}))
	select {}
}
