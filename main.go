package main

import (
	"fmt"
	"os"
)

func main() {
	if err := run(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func run() error {
	app, err := newApp()
	if err != nil {
		return err
	}

	return app.run()
}
