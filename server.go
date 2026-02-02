package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
)

func (a *app) serverMode(ctx context.Context) error {
	server := http.Server{
		Addr:    a.config.ServerAddress,
		Handler: a,
	}

	go func() {
		<-ctx.Done()
		fmt.Println("Server is shutting down...")
		if err := server.Shutdown(context.Background()); err != nil {
			fmt.Printf("failed to shutdown server: %s\n", err)
		}
	}()

	fmt.Printf("Server running on %s\n", a.config.ServerAddress)
	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		return err
	}

	return nil
}

// ServeHTTP implements the http.Handler interface.
func (a *app) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	headers := []string{"X-Real-IP", "X-Forwarded-For"}
	for _, h := range headers {
		from := req.Header.Get(h)
		if from != "" {
			rw.WriteHeader(http.StatusOK)
			_, err := rw.Write([]byte(from))
			if err != nil {
				fmt.Fprintf(os.Stderr, "failed to write response: %s\n", err)
			}
			return
		}
	}

	rw.WriteHeader(http.StatusNotFound)
}
