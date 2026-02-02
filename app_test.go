package main

import (
	"context"
	"fmt"
	"net/netip"
	"testing"
)

type mockDNSProvider struct {
	addr netip.Addr
	err  error
}

func (m *mockDNSProvider) Lookup(_ context.Context, _ string) (netip.Addr, error) {
	return m.addr, m.err
}

type mockIPProvider struct {
	addr netip.Addr
	err  error
}

func (m *mockIPProvider) Get(_ context.Context, _ string) (netip.Addr, error) {
	return m.addr, m.err
}

func TestUpdateDomainIfNeeded(t *testing.T) {
	ip := netip.MustParseAddr("203.0.113.1")
	oldIP := netip.MustParseAddr("198.51.100.1")

	tests := []struct {
		name       string
		ip         netip.Addr
		ipErr      error
		dnsIP      netip.Addr
		dnsErr     error
		wantUpdate bool
		wantErr    bool
	}{
		{
			name:  "ip matches dns, no update",
			ip:    ip,
			dnsIP: ip,
		},
		{
			name:       "ip differs from dns, update",
			ip:         ip,
			dnsIP:      oldIP,
			wantUpdate: true,
		},
		{
			name:       "dns not found, update",
			ip:         ip,
			dnsIP:      netip.Addr{},
			wantUpdate: true,
		},
		{
			name:    "ip provider error",
			ipErr:   fmt.Errorf("connection refused"),
			wantErr: true,
		},
		{
			name:    "ip provider returns zero addr",
			ip:      netip.Addr{},
			wantErr: true,
		},
		{
			name:    "dns provider error",
			ip:      ip,
			dnsErr:  fmt.Errorf("dns timeout"),
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ovhMock := &mockOVHClient{
				getFunc: func(url string, resType interface{}) error {
					// fetchZoneRecordID returns empty → create path
					jsonInto([]int{}, resType)
					return nil
				},
			}

			a := &app{
				config: config{
					Domain:    "example.com",
					SubDomain: "home",
					TTL:       300,
				},
				client:      ovhMock,
				ipProvider:  &mockIPProvider{addr: tc.ip, err: tc.ipErr},
				dnsProvider: &mockDNSProvider{addr: tc.dnsIP, err: tc.dnsErr},
			}

			err := a.updateDomainIfNeeded(context.Background())

			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			updated := len(ovhMock.postCalls) > 0 || len(ovhMock.putCalls) > 0
			if updated != tc.wantUpdate {
				t.Fatalf("update happened: %v, want: %v", updated, tc.wantUpdate)
			}
		})
	}
}
