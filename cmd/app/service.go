package main

import "runtime"

// App is the root Wails service: the bridge between the Go backend and the
// frontend. It is intentionally thin — all real functionality lives in the
// framework-agnostic packages under internal/ (core, library, opds…). Feature
// services will be registered alongside it in later sessions.
type App struct{}

// PingResult is what Ping returns. Exposing a struct (rather than a bare
// string) exercises the binding generator's model generation end to end and
// gives the UI something concrete to display.
type PingResult struct {
	Message   string `json:"message"`
	GoVersion string `json:"goVersion"`
	Platform  string `json:"platform"`
}

// Ping is a health-check binding used to validate the full Go→JS chain: the
// frontend calls it on load and renders the reply.
func (a *App) Ping() PingResult {
	return PingResult{
		Message:   "pong",
		GoVersion: runtime.Version(),
		Platform:  runtime.GOOS + "/" + runtime.GOARCH,
	}
}
