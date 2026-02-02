package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ovh/go-ovh/ovh"
	"gopkg.in/yaml.v3"
)

type config struct {
	Provider      string        `yaml:"provider"`
	Domain        string        `yaml:"domain"`
	SubDomain     string        `yaml:"sub_domain"`
	TTL           uint          `yaml:"ttl"`
	DNSProvider   string        `yaml:"dns_provider"`
	CheckInterval time.Duration `yaml:"check_interval"`
	ServerAddress string        `yaml:"server_address"`
	OVH           struct {
		ApplicationKey    string `yaml:"application_key"`
		ApplicationSecret string `yaml:"application_secret"`
		ConsumerKey       string `yaml:"consumer_key"`
		Endpoint          string `yaml:"endpoint"`
	} `yaml:"ovh"`
}

type appMode int

const (
	appModeUnconfigured appMode = iota
	appModeServer
	appModeDaemon
	appModeClean
)

type app struct {
	config     config
	client     *ovh.Client
	resolver   *net.Resolver
	httpClient *http.Client
	mode       appMode
}

func newApp() (*app, error) {
	var configPath string
	var clean, daemon, server bool

	flag.StringVar(&configPath, "config", "config.yaml", "config file path")
	flag.BoolVar(&clean, "clean", false, "cleanup dns records and exit")
	flag.BoolVar(&daemon, "daemon", false, "run in daemon mode")
	flag.BoolVar(&server, "server", false, "run server mode")
	flag.Parse()

	file, err := os.Open(configPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	app := &app{}

	if err := yaml.NewDecoder(file).Decode(&app.config); err != nil {
		return nil, fmt.Errorf("Failed to decode config file: %w", err)
	}

	if server {
		if app.config.ServerAddress == "" {
			return nil, fmt.Errorf("Missing server address")
		}
	} else {
		app.resolver = app.newResolver()
		app.httpClient = http.DefaultClient

		// Ensure the check interval is greater or equal to the TTL
		if app.config.CheckInterval.Seconds() < float64(app.config.TTL) {
			app.config.CheckInterval = time.Duration(app.config.TTL) * time.Second
			fmt.Printf("Using the TTL as the check interval: %s\n", app.config.CheckInterval)
		}

		app.client, err = ovh.NewClient(
			app.config.OVH.Endpoint, app.config.OVH.ApplicationKey,
			app.config.OVH.ApplicationSecret, app.config.OVH.ConsumerKey)
		if err != nil {
			return nil, err
		}
	}

	switch {
	case daemon:
		app.mode = appModeDaemon
	case server:
		app.mode = appModeServer
	case clean:
		app.mode = appModeClean
	default:
		return app, fmt.Errorf("Please select a mode")
	}

	return app, nil
}

func (a *app) run() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		cancel()
	}()

	switch a.mode {
	case appModeDaemon:
		return a.daemonMode(ctx)
	case appModeServer:
		return a.serverMode(ctx)
	case appModeClean:
		return a.cleanMode()
	default:
		return fmt.Errorf("Invalid app mode")
	}
}

func (a *app) daemonMode(ctx context.Context) error {
	ticker := time.NewTicker(a.config.CheckInterval)
	defer ticker.Stop()

	fmt.Println("Starting deamon mode")

	a.tryUpdateDomainIfNeeded(ctx)

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			a.tryUpdateDomainIfNeeded(ctx)
		}
	}
}

func (a *app) tryUpdateDomainIfNeeded(ctx context.Context) {
	err := a.updateDomainIfNeeded(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to update domain: %s\n", err)
	}
}

func (a *app) updateDomainIfNeeded(ctx context.Context) error {
	ip, err := a.currentIP(ctx)
	if err != nil {
		return err
	}

	dnsIP, err := a.dnsLookup(ctx)
	if err != nil {
		return err
	}

	if ip == dnsIP {
		// All good
		return nil
	}

	if dnsIP == "" {
		dnsIP = "not configured"
	}

	fmt.Printf("Local IP: %s\n", ip)
	fmt.Printf("DNS IP:   %s\n", dnsIP)

	_, err = a.updateZoneRecord(ip)
	return err
}

func (a *app) cleanMode() error {
	return a.deleteZoneRecord()
}
