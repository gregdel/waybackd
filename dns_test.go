package main

import (
	"context"
	"net"
	"net/netip"
	"testing"

	"golang.org/x/net/dns/dnsmessage"
)

// testDNSServer starts a UDP DNS server that responds based on the handler.
// The handler returns A record IPs for the query, or nil for NXDOMAIN.
func testDNSServer(t *testing.T, handler func(name string) []netip.Addr) string {
	t.Helper()

	conn, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })

	go func() {
		buf := make([]byte, 512)
		for {
			n, addr, err := conn.ReadFrom(buf)
			if err != nil {
				return
			}

			var msg dnsmessage.Message
			if err := msg.Unpack(buf[:n]); err != nil {
				continue
			}

			resp := dnsmessage.Message{
				Header: dnsmessage.Header{
					ID:       msg.ID,
					Response: true,
				},
				Questions: msg.Questions,
			}

			if len(msg.Questions) > 0 {
				name := msg.Questions[0].Name.String()
				ips := handler(name)
				if ips == nil {
					resp.Header.RCode = dnsmessage.RCodeNameError
				} else {
					for _, ip := range ips {
						raw := ip.As4()
						resp.Answers = append(resp.Answers, dnsmessage.Resource{
							Header: dnsmessage.ResourceHeader{
								Name:  msg.Questions[0].Name,
								Type:  dnsmessage.TypeA,
								Class: dnsmessage.ClassINET,
								TTL:   60,
							},
							Body: &dnsmessage.AResource{A: raw},
						})
					}
				}
			}

			packed, err := resp.Pack()
			if err != nil {
				continue
			}
			conn.WriteTo(packed, addr)
		}
	}()

	return conn.LocalAddr().String()
}

func TestDNSProviderLookup(t *testing.T) {
	tests := []struct {
		name     string
		host     string
		ips      []netip.Addr // nil = NXDOMAIN
		wantAddr netip.Addr
		wantErr  bool
	}{
		{
			name:     "single A record",
			host:     "example.com.",
			ips:      []netip.Addr{netip.MustParseAddr("203.0.113.1")},
			wantAddr: netip.MustParseAddr("203.0.113.1"),
		},
		{
			name: "not found",
			host: "missing.example.com.",
		},
		{
			name:    "multiple A records",
			host:    "multi.example.com.",
			ips:     []netip.Addr{netip.MustParseAddr("203.0.113.1"), netip.MustParseAddr("203.0.113.2")},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			addr := testDNSServer(t, func(name string) []netip.Addr {
				if name == tc.host {
					return tc.ips
				}
				return nil
			})

			dns := newDNSProvider(addr)
			got, err := dns.Lookup(context.Background(), tc.host)

			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got != tc.wantAddr {
				t.Fatalf("got %v, want %v", got, tc.wantAddr)
			}
		})
	}
}
