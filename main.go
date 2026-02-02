package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	var configPath string
	var setup bool
	flag.StringVar(&configPath, "config", "config.yaml", "config file path")
	flag.BoolVar(&setup, "setup", false, "request an OVH consumer key")
	flag.Parse()

	var err error
	if setup {
		err = runSetup(configPath)
	} else {
		err = run(configPath)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}
}

func run(configPath string) error {
	app, err := newApp(configPath)
	if err != nil {
		return err
	}

	return app.run()
}
