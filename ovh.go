package main

import (
	"fmt"
	"net/netip"
	"net/url"
)

type OVHClient interface {
	Get(url string, resType any) error
	Post(url string, reqBody, resType any) error
	Put(url string, reqBody, resType any) error
}

type zoneRecord struct {
	FieldType string `json:"fieldType"`
	Subdomain string `json:"subDomain"`
	TTL       uint   `json:"ttl"`
	Target    string `json:"target"`
}

func newZoneRecord(d domain, ip string) *zoneRecord {
	return &zoneRecord{
		Subdomain: d.SubDomain,
		TTL:       uint(d.TTL.Seconds()),
		FieldType: "A",
		Target:    ip,
	}
}

func (a *app) refreshZoneRecord(d domain) error {
	url := "/domain/zone/" + d.Domain + "/refresh"
	if err := a.client.Post(url, nil, nil); err != nil {
		return fmt.Errorf("failed to refresh the zone: %w", err)
	}

	fmt.Printf("%s: DNS zone refreshed\n", d.hostname())
	return nil
}

func (a *app) fetchZoneRecordID(d domain) (int, error) {
	baseURL := "/domain/zone/" + d.Domain

	v := url.Values{}
	v.Add("fieldType", "A")
	v.Add("subDomain", d.SubDomain)
	url := fmt.Sprintf("%s/record?%s", baseURL, v.Encode())
	recordIDs := []int{}
	if err := a.client.Get(url, &recordIDs); err != nil {
		return 0, err
	}

	switch len(recordIDs) {
	case 0:
		return 0, nil
	case 1:
		return recordIDs[0], nil
	default:
		return 0, fmt.Errorf("multiple ids for this record, something's wrong")
	}
}

func (a *app) updateZoneRecord(d domain, ip netip.Addr) (*zoneRecord, error) {
	baseURL := "/domain/zone/" + d.Domain + "/record"

	id, err := a.fetchZoneRecordID(d)
	if err != nil {
		return nil, err
	}

	var record *zoneRecord
	if id == 0 {
		fmt.Printf("%s: creating a new zone record...\n", d.hostname())
		record = newZoneRecord(d, ip.String())
		if err := a.client.Post(baseURL, record, record); err != nil {
			return nil, fmt.Errorf("failed to create the zone record: %w", err)
		}
	} else {
		record = &zoneRecord{}

		url := fmt.Sprintf("%s/%d", baseURL, id)
		if err := a.client.Get(url, record); err != nil {
			return nil, fmt.Errorf("failed to get the zone record: %w", err)
		}

		if record.Target == ip.String() {
			fmt.Printf("%s: DNS target is already good\n", d.hostname())
			return record, nil
		}

		fmt.Printf("%s: IP %s does not match the current DNS target %s, updating...\n",
			d.hostname(), ip, record.Target)

		record = newZoneRecord(d, ip.String())
		if err := a.client.Put(url, record, nil); err != nil {
			return nil, fmt.Errorf("failed to update the zone record: %w", err)
		}
	}

	err = a.refreshZoneRecord(d)
	return record, err
}
