package main

import (
	"context"
	"errors"
	"fmt"
	"net"
)

func (a *app) newResolver() *net.Resolver {
	resolver := &net.Resolver{PreferGo: true}
	resolver.Dial = func(ctx context.Context, network, address string) (net.Conn, error) {
		var d net.Dialer
		return d.DialContext(ctx, "udp", a.config.DNSProvider+":53")
	}

	return resolver
}

func (a *app) dnsLookup(ctx context.Context) (string, error) {
	addrs, err := a.resolver.LookupHost(ctx, a.config.SubDomain+"."+a.config.Domain)
	if err != nil {
		var dnsError *net.DNSError
		if !errors.As(err, &dnsError) {
			return "", err
		}

		if dnsError.IsTimeout {
			return "", fmt.Errorf("dns timeout: %w", err)
		}

		if dnsError.IsNotFound {
			return "", nil
		}

		return "", err
	}

	if len(addrs) != 1 {
		return "", fmt.Errorf("expected 1 dns address found: %v", addrs)
	}

	return addrs[0], nil
}
