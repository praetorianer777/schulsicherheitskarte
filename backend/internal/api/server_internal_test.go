package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// clientAddress runs a request through the real middleware chain and reports
// the address the handlers end up seeing.
func clientAddress(t *testing.T, options Options, forwardedFor string) string {
	t.Helper()

	server := &Server{options: options}
	router := server.router()

	var seen string
	router.Get("/probe", func(w http.ResponseWriter, r *http.Request) {
		seen = r.RemoteAddr
	})

	request := httptest.NewRequest(http.MethodGet, "/probe", nil)
	request.RemoteAddr = "10.0.0.1:1234"
	if forwardedFor != "" {
		request.Header.Set("X-Forwarded-For", forwardedFor)
	}
	router.ServeHTTP(httptest.NewRecorder(), request)
	return seen
}

// Without a proxy in front, the header is whatever the caller chose to send.
// Believing it would let anyone claim any address — and the rate limiting on
// reports will key on exactly this value.
func TestForwardedAddressIsIgnoredByDefault(t *testing.T) {
	got := clientAddress(t, Options{}, "203.0.113.7")
	if got != "10.0.0.1:1234" {
		t.Errorf("client address = %q; a forged X-Forwarded-For was believed", got)
	}
}

// Behind a proxy the header is the only way to see the client at all: without
// it every request appears to come from the proxy, and a rate limit would count
// all visitors as one.
func TestForwardedAddressIsUsedWhenTheProxyIsTrusted(t *testing.T) {
	got := clientAddress(t, Options{TrustProxyHeaders: true}, "203.0.113.7")
	if got != "203.0.113.7" {
		t.Errorf("client address = %q, want the forwarded 203.0.113.7", got)
	}
}

func TestTrustedProxyWithoutTheHeaderKeepsTheDirectAddress(t *testing.T) {
	got := clientAddress(t, Options{TrustProxyHeaders: true}, "")
	if got != "10.0.0.1:1234" {
		t.Errorf("client address = %q, want the direct address", got)
	}
}
