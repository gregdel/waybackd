package main

import (
	"fmt"
	"net/url"
)

type zoneRecord struct {
	FieldType string `json:"fieldType"`
	Subdomain string `json:"subDomain"`
	TTL       uint   `json:"ttl"`
	Target    string `json:"target"`
}

func (a *app) newZoneRecord(ip string) *zoneRecord {
	return &zoneRecord{
		Subdomain: a.config.SubDomain,
		TTL:       a.config.TTL,
		FieldType: "A",
		Target:    ip,
	}
}

func (a *app) refreshZoneRecord() error {
	url := "/domain/zone/" + a.config.Domain + "/refresh"
	if err := a.client.Post(url, nil, nil); err != nil {
		return fmt.Errorf("failed to refresh the zone: %w", err)
	}

	fmt.Println("DNS zone refreshed")
	return nil
}

func (a *app) fetchZoneRecordID() (int, error) {
	baseURL := "/domain/zone/" + a.config.Domain

	v := url.Values{}
	v.Add("fieldType", "A")
	v.Add("subDomain", a.config.SubDomain)
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

func (a *app) updateZoneRecord(ip string) (*zoneRecord, error) {
	baseURL := "/domain/zone/" + a.config.Domain + "/record"

	id, err := a.fetchZoneRecordID()
	if err != nil {
		return nil, err
	}

	var record *zoneRecord
	if id == 0 {
		fmt.Println("Creating a new zone record...")
		record = a.newZoneRecord(ip)
		if err := a.client.Post(baseURL, record, record); err != nil {
			return nil, fmt.Errorf("failed to create the zone record: %w", err)
		}
	} else {
		record = &zoneRecord{}

		url := fmt.Sprintf("%s/%d", baseURL, id)
		if err := a.client.Get(url, record); err != nil {
			return nil, fmt.Errorf("failed to get the zone record: %w", err)
		}

		if record.Target == ip {
			fmt.Println("DNS target is already good")
			return record, nil
		}

		fmt.Printf("IP %s does not match the current DNS target %s, updating...\n",
			ip, record.Target)

		record = a.newZoneRecord(ip)
		if err := a.client.Put(url, record, nil); err != nil {
			return nil, fmt.Errorf("failed to update the zone record: %w", err)
		}
	}

	err = a.refreshZoneRecord()
	return record, err
}
