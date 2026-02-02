package main

import (
	"encoding/json"
	"fmt"
	"net/netip"
	"testing"
	"time"
)

type mockOVHClient struct {
	getCalls  []string
	postCalls []string
	putCalls  []string

	getFunc  func(url string, resType any) error
	postFunc func(url string, reqBody, resType any) error
	putFunc  func(url string, reqBody, resType any) error
}

func (m *mockOVHClient) Get(url string, resType any) error {
	m.getCalls = append(m.getCalls, url)
	if m.getFunc != nil {
		return m.getFunc(url, resType)
	}
	return nil
}

func (m *mockOVHClient) Post(url string, reqBody, resType any) error {
	m.postCalls = append(m.postCalls, url)
	if m.postFunc != nil {
		return m.postFunc(url, reqBody, resType)
	}
	return nil
}

func (m *mockOVHClient) Put(url string, reqBody, resType any) error {
	m.putCalls = append(m.putCalls, url)
	if m.putFunc != nil {
		return m.putFunc(url, reqBody, resType)
	}
	return nil
}

// jsonInto marshals src then unmarshals into dst, simulating how the OVH
// client populates response types.
func jsonInto(src, dst any) {
	b, _ := json.Marshal(src)
	json.Unmarshal(b, dst)
}

func testDomain() domain {
	return domain{Domain: "example.com", SubDomain: "home", TTL: 300 * time.Second}
}

func testApp(client *mockOVHClient) *app {
	return &app{
		config: config{Domains: []domain{testDomain()}},
		client: client,
	}
}

func TestFetchZoneRecordID(t *testing.T) {
	tests := []struct {
		name    string
		ids     []int
		wantID  int
		wantErr bool
	}{
		{
			name:   "no records",
			ids:    []int{},
			wantID: 0,
		},
		{
			name:   "one record",
			ids:    []int{42},
			wantID: 42,
		},
		{
			name:    "multiple records",
			ids:     []int{1, 2},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mock := &mockOVHClient{
				getFunc: func(url string, resType any) error {
					jsonInto(tc.ids, resType)
					return nil
				},
			}

			a := testApp(mock)
			id, err := a.fetchZoneRecordID(testDomain())

			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if id != tc.wantID {
				t.Fatalf("got %d, want %d", id, tc.wantID)
			}
		})
	}
}

func TestUpdateZoneRecord(t *testing.T) {
	ip := netip.MustParseAddr("203.0.113.1")

	t.Run("create new record", func(t *testing.T) {
		mock := &mockOVHClient{
			getFunc: func(url string, resType any) error {
				// fetchZoneRecordID returns empty list
				jsonInto([]int{}, resType)
				return nil
			},
		}

		a := testApp(mock)
		record, err := a.updateZoneRecord(testDomain(), ip)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if record.Target != ip.String() {
			t.Fatalf("got target %s, want %s", record.Target, ip)
		}
		if record.FieldType != "A" {
			t.Fatalf("got field type %s, want A", record.FieldType)
		}
		if len(mock.postCalls) != 2 {
			t.Fatalf("expected 2 POST calls (create + refresh), got %d", len(mock.postCalls))
		}
	})

	t.Run("update existing record", func(t *testing.T) {
		callNum := 0
		mock := &mockOVHClient{
			getFunc: func(url string, resType any) error {
				callNum++
				switch callNum {
				case 1:
					// fetchZoneRecordID
					jsonInto([]int{42}, resType)
				case 2:
					// get existing record
					jsonInto(&zoneRecord{
						Target:    "198.51.100.1",
						FieldType: "A",
						Subdomain: "home",
						TTL:       300,
					}, resType)
				}
				return nil
			},
		}

		a := testApp(mock)
		record, err := a.updateZoneRecord(testDomain(), ip)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if record.Target != ip.String() {
			t.Fatalf("got target %s, want %s", record.Target, ip)
		}
		if len(mock.putCalls) != 1 {
			t.Fatalf("expected 1 PUT call, got %d", len(mock.putCalls))
		}
	})

	t.Run("existing record already matches", func(t *testing.T) {
		callNum := 0
		mock := &mockOVHClient{
			getFunc: func(url string, resType any) error {
				callNum++
				switch callNum {
				case 1:
					jsonInto([]int{42}, resType)
				case 2:
					jsonInto(&zoneRecord{
						Target:    ip.String(),
						FieldType: "A",
						Subdomain: "home",
						TTL:       300,
					}, resType)
				}
				return nil
			},
		}

		a := testApp(mock)
		record, err := a.updateZoneRecord(testDomain(), ip)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if record.Target != ip.String() {
			t.Fatalf("got target %s, want %s", record.Target, ip)
		}
		if len(mock.putCalls) != 0 {
			t.Fatalf("expected 0 PUT calls, got %d", len(mock.putCalls))
		}
		if len(mock.postCalls) != 0 {
			t.Fatalf("expected 0 POST calls (no refresh), got %d", len(mock.postCalls))
		}
	})

	t.Run("fetch record ID fails", func(t *testing.T) {
		mock := &mockOVHClient{
			getFunc: func(url string, resType any) error {
				return fmt.Errorf("api error")
			},
		}

		a := testApp(mock)
		_, err := a.updateZoneRecord(testDomain(), ip)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}
