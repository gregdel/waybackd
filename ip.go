package main

import (
	"bytes"
	"context"
	"fmt"
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

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("invalid response from server: %s", resp.Status)
	}

	bodyReader := io.LimitReader(resp.Body, 64)

	body, err := io.ReadAll(bodyReader)
	if err != nil {
		return "", err
	}
	body = bytes.TrimRight(body, "\n")

	return string(body), nil
}
