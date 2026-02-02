package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/netip"
)

type IPProvider interface {
	Get(ctx context.Context, provider string) (netip.Addr, error)
}

type ipProvider struct {
	client *http.Client
}

func newIpProvider() *ipProvider {
	return &ipProvider{client: http.DefaultClient}
}

func (p *ipProvider) Get(ctx context.Context, provider string) (netip.Addr, error) {
	var addr netip.Addr
	req, err := http.NewRequestWithContext(ctx, "GET", provider, nil)
	if err != nil {
		return addr, err
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return addr, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return addr, fmt.Errorf("invalid response from server: %s", resp.Status)
	}

	bodyReader := io.LimitReader(resp.Body, 64)

	body, err := io.ReadAll(bodyReader)
	if err != nil {
		return addr, err
	}
	body = bytes.TrimRight(body, "\n")

	if err := addr.UnmarshalText(body); err != nil {
		return addr, err
	}

	if !addr.IsValid() {
		return addr, fmt.Errorf("invalid IP from provider")
	}

	return addr, nil
}
