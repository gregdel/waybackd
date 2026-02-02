package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
)

type DNSProvider interface {
	Lookup(ctx context.Context, provider string) (netip.Addr, error)
}

type dnsProvider struct {
	resolver *net.Resolver
}

func newDNSProvider(provider string) *dnsProvider {
	dns := &dnsProvider{
		resolver: &net.Resolver{PreferGo: true},
	}
	dns.resolver.Dial = func(ctx context.Context, network, address string) (net.Conn, error) {
		var d net.Dialer
		return d.DialContext(ctx, "udp", provider)
	}

	return dns
}

func (dns *dnsProvider) Lookup(ctx context.Context, host string) (netip.Addr, error) {
	var addr netip.Addr
	addrs, err := dns.resolver.LookupHost(ctx, host)
	if err != nil {
		var dnsError *net.DNSError
		if !errors.As(err, &dnsError) {
			return addr, err
		}

		if dnsError.IsTimeout {
			return addr, fmt.Errorf("dns timeout: %w", err)
		}

		if dnsError.IsNotFound {
			return addr, nil
		}

		return addr, err
	}

	if len(addrs) != 1 {
		return addr, fmt.Errorf("expected 1 dns address found: %v", addrs)
	}

	return netip.ParseAddr(addrs[0])
}
