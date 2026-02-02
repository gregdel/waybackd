package main

import (
	"context"
	"fmt"
	"net/netip"
	"testing"
	"time"
)

type mockDNSProvider struct {
	addr    netip.Addr
	err     error
	lookups []string
}

func (m *mockDNSProvider) Lookup(_ context.Context, host string) (netip.Addr, error) {
	m.lookups = append(m.lookups, host)
	return m.addr, m.err
}

type mockIPProvider struct {
	addr netip.Addr
	err  error
	gets int
}

func (m *mockIPProvider) Get(_ context.Context, _ string) (netip.Addr, error) {
	m.gets++
	return m.addr, m.err
}

func TestUpdateDomainIfNeeded(t *testing.T) {
	ip := netip.MustParseAddr("203.0.113.1")
	oldIP := netip.MustParseAddr("198.51.100.1")
	d := testDomain()

	tests := []struct {
		name       string
		ip         netip.Addr
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
					jsonInto([]int{}, resType)
					return nil
				},
			}

			a := &app{
				config:      config{Domains: []domain{d}},
				client:      ovhMock,
				dnsProvider: &mockDNSProvider{addr: tc.dnsIP, err: tc.dnsErr},
			}

			err := a.updateDomainIfNeeded(context.Background(), d, tc.ip)

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

func TestTryUpdateDomainsIfNeeded(t *testing.T) {
	ip := netip.MustParseAddr("203.0.113.1")

	t.Run("ip provider error", func(t *testing.T) {
		dns := &mockDNSProvider{}
		a := &app{
			config:      config{Domains: []domain{testDomain()}},
			ipProvider:  &mockIPProvider{err: fmt.Errorf("connection refused")},
			dnsProvider: dns,
		}

		a.tryUpdateDomainsIfNeeded(context.Background())

		if len(dns.lookups) != 0 {
			t.Fatalf("expected no DNS lookups, got %d", len(dns.lookups))
		}
	})

	t.Run("ip provider returns zero addr", func(t *testing.T) {
		dns := &mockDNSProvider{}
		a := &app{
			config:      config{Domains: []domain{testDomain()}},
			ipProvider:  &mockIPProvider{},
			dnsProvider: dns,
		}

		a.tryUpdateDomainsIfNeeded(context.Background())

		if len(dns.lookups) != 0 {
			t.Fatalf("expected no DNS lookups, got %d", len(dns.lookups))
		}
	})

	t.Run("multiple domains, one IP fetch", func(t *testing.T) {
		d1 := domain{Domain: "example.com", SubDomain: "a", TTL: 60 * time.Second}
		d2 := domain{Domain: "example.org", SubDomain: "b", TTL: 60 * time.Second}

		dns := &mockDNSProvider{addr: ip} // matches → no update needed
		ipMock := &mockIPProvider{addr: ip}

		a := &app{
			config:      config{Domains: []domain{d1, d2}},
			client:      &mockOVHClient{},
			ipProvider:  ipMock,
			dnsProvider: dns,
		}

		a.tryUpdateDomainsIfNeeded(context.Background())

		if ipMock.gets != 1 {
			t.Fatalf("expected 1 IP fetch, got %d", ipMock.gets)
		}
		if len(dns.lookups) != 2 {
			t.Fatalf("expected 2 DNS lookups, got %d", len(dns.lookups))
		}
		if dns.lookups[0] != "a.example.com" {
			t.Fatalf("expected first lookup a.example.com, got %s", dns.lookups[0])
		}
		if dns.lookups[1] != "b.example.org" {
			t.Fatalf("expected second lookup b.example.org, got %s", dns.lookups[1])
		}
	})
}
