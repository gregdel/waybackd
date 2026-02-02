package main

import (
	"context"
	"flag"
	"fmt"
	"net/netip"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/ovh/go-ovh/ovh"
)

type domain struct {
	Domain    string        `yaml:"domain"`
	SubDomain string        `yaml:"sub_domain"`
	TTL       time.Duration `yaml:"ttl"`
}

func (d domain) hostname() string {
	return d.SubDomain + "." + d.Domain
}

type config struct {
	Provider      string        `yaml:"provider"`
	DNSProvider   string        `yaml:"dns_provider"`
	CheckInterval time.Duration `yaml:"check_interval"`
	Domains       []domain      `yaml:"domains"`
	OVH           struct {
		ApplicationKey    string `yaml:"application_key"`
		ApplicationSecret string `yaml:"application_secret"`
		ConsumerKey       string `yaml:"consumer_key"`
		Endpoint          string `yaml:"endpoint"`
	} `yaml:"ovh"`
}

type app struct {
	config      config
	client      OVHClient
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

	if len(app.config.Domains) == 0 {
		return nil, fmt.Errorf("no domains configured")
	}

	app.dnsProvider = newDNSProvider(app.config.DNSProvider + ":53")
	app.ipProvider = newIpProvider()

	// Ensure the check interval is greater or equal to the minimum TTL
	var minTTL time.Duration
	for _, d := range app.config.Domains {
		if minTTL == 0 || d.TTL < minTTL {
			minTTL = d.TTL
		}
	}
	if app.config.CheckInterval < minTTL {
		app.config.CheckInterval = minTTL
		fmt.Printf("Using the minimum TTL as the check interval: %s\n", app.config.CheckInterval)
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

	a.tryUpdateDomainsIfNeeded(ctx)

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			a.tryUpdateDomainsIfNeeded(ctx)
		}
	}
}

func (a *app) tryUpdateDomainsIfNeeded(ctx context.Context) {
	ip, err := a.ipProvider.Get(ctx, a.config.Provider)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to get IP: %s\n", err)
		return
	}

	if !ip.IsValid() {
		fmt.Fprintf(os.Stderr, "got invalid IP from provider\n")
		return
	}

	for _, d := range a.config.Domains {
		if err := a.updateDomainIfNeeded(ctx, d, ip); err != nil {
			fmt.Fprintf(os.Stderr, "%s: failed to update domain: %s\n", d.hostname(), err)
		}
	}
}

func (a *app) updateDomainIfNeeded(ctx context.Context, d domain, ip netip.Addr) error {
	dnsIP, err := a.dnsProvider.Lookup(ctx, d.hostname())
	if err != nil {
		return err
	}

	if ip == dnsIP {
		return nil
	}

	fmt.Printf("%s: local IP: %s, DNS IP: %s\n", d.hostname(), ip, dnsIP)

	_, err = a.updateZoneRecord(d, ip)
	return err
}
