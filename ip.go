package main

import (
	"bytes"
	"context"
	"io"
	"net/http"
)

func (a *app) currentIP(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", a.config.Provider, nil)
	if err != nil {
		return "", err
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	body = bytes.TrimRight(body, "\n")

	return string(body), nil
}
