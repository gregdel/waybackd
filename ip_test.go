package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
)

func TestIPProviderGet(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		statusCode int
		wantAddr   netip.Addr
		wantErr    bool
	}{
		{
			name:       "valid ip",
			body:       "203.0.113.1",
			statusCode: http.StatusOK,
			wantAddr:   netip.MustParseAddr("203.0.113.1"),
		},
		{
			name:       "valid ip with trailing newline",
			body:       "203.0.113.1\n",
			statusCode: http.StatusOK,
			wantAddr:   netip.MustParseAddr("203.0.113.1"),
		},
		{
			name:       "valid ipv6",
			body:       "2001:db8::1",
			statusCode: http.StatusOK,
			wantAddr:   netip.MustParseAddr("2001:db8::1"),
		},
		{
			name:       "server error",
			body:       "",
			statusCode: http.StatusInternalServerError,
			wantErr:    true,
		},
		{
			name:       "invalid ip",
			body:       "not-an-ip",
			statusCode: http.StatusOK,
			wantErr:    true,
		},
		{
			name:       "empty body",
			body:       "",
			statusCode: http.StatusOK,
			wantAddr:   netip.Addr{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.statusCode)
				w.Write([]byte(tc.body))
			}))
			defer srv.Close()

			p := &ipProvider{client: srv.Client()}
			addr, err := p.Get(context.Background(), srv.URL)

			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if addr != tc.wantAddr {
				t.Fatalf("got %v, want %v", addr, tc.wantAddr)
			}
		})
	}
}
