package main

import (
	"context"
	"flag"
	"fmt"
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
	OVH           struct {
		ApplicationKey    string `yaml:"application_key"`
		ApplicationSecret string `yaml:"application_secret"`
		ConsumerKey       string `yaml:"consumer_key"`
		Endpoint          string `yaml:"endpoint"`
	} `yaml:"ovh"`
}

type app struct {
	config      config
	client      *ovh.Client
	dnsProvider DNSProvider
	ipProvider  IPProvider
}

func newApp() (*app, error) {
	var configPath string
	flag.StringVar(&configPath, "config", "config.yaml", "config file path")
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

	app.dnsProvider = newDNSProvider(app.config.DNSProvider + ":53")
	app.ipProvider = newIpProvider()

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

	ticker := time.NewTicker(a.config.CheckInterval)
	defer ticker.Stop()

	fmt.Println("Starting daemon mode")

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
	ip, err := a.ipProvider.Get(ctx, a.config.Provider)
	if err != nil {
		return err
	}

	host := a.config.SubDomain + "." + a.config.Domain

	dnsIP, err := a.dnsProvider.Lookup(ctx, host)
	if err != nil {
		return err
	}

	if ip == dnsIP {
		// All good
		return nil
	}

	fmt.Printf("Local IP: %s\n", ip)
	fmt.Printf("DNS IP:   %s\n", dnsIP)

	_, err = a.updateZoneRecord(ip)
	return err
}
