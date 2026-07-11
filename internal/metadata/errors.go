package metadata

import "fmt"

// httpError reports a non-200 response from a provider without leaking the body.
type httpError struct {
	provider string
	status   int
}

func (e *httpError) Error() string {
	return fmt.Sprintf("%s: unexpected status %d", e.provider, e.status)
}
