package gleif

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

const appleListJSON = `{
  "data": [
    {
      "id": "HWUPKR0MPOU8FGXBT394",
      "type": "lei-records",
      "attributes": {
        "lei": "HWUPKR0MPOU8FGXBT394",
        "entity": {
          "legalName": {"name": "APPLE INC."},
          "legalAddress": {"country": "US", "region": "US-CA", "city": "CUPERTINO"},
          "jurisdiction": "US-CA",
          "category": "GENERAL"
        },
        "registration": {
          "status": "ISSUED",
          "initialRegistrationDate": "2012-11-01T00:00:00.000Z",
          "lastUpdateDate": "2025-01-01T00:00:00.000Z",
          "nextRenewalDate": "2025-02-01T00:00:00.000Z"
        }
      }
    }
  ],
  "meta": {"pagination": {"total": 1}}
}`

const appleSingleJSON = `{
  "data": {
    "id": "HWUPKR0MPOU8FGXBT394",
    "type": "lei-records",
    "attributes": {
      "lei": "HWUPKR0MPOU8FGXBT394",
      "entity": {
        "legalName": {"name": "APPLE INC."},
        "legalAddress": {"country": "US", "region": "US-CA", "city": "CUPERTINO"},
        "jurisdiction": "US-CA",
        "category": "GENERAL"
      },
      "registration": {
        "status": "ISSUED",
        "initialRegistrationDate": "2012-11-01T00:00:00.000Z",
        "lastUpdateDate": "2025-01-01T00:00:00.000Z",
        "nextRenewalDate": "2025-02-01T00:00:00.000Z"
      }
    }
  }
}`

func TestGet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Error("request carried no User-Agent")
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	c := NewClient()
	c.Rate = 0

	body, err := c.Get(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "ok" {
		t.Errorf("body = %q, want ok", body)
	}
}

func TestGetRetriesOn503(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if hits < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte("recovered"))
	}))
	defer srv.Close()

	c := NewClient()
	c.Rate = 0
	c.Retries = 5

	start := time.Now()
	body, err := c.Get(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "recovered" {
		t.Errorf("body = %q after retries", body)
	}
	if hits != 3 {
		t.Errorf("server saw %d hits, want 3", hits)
	}
	if time.Since(start) < 500*time.Millisecond {
		t.Error("retries did not back off")
	}
}

func TestSearch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/lei-records" {
			t.Errorf("path = %q, want /lei-records", r.URL.Path)
		}
		q := r.URL.Query()
		if q.Get("filter[fulltext]") == "" {
			t.Error("missing filter[fulltext] parameter")
		}
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = w.Write([]byte(appleListJSON))
	}))
	defer srv.Close()

	c := NewClient()
	c.Rate = 0
	c.BaseURL = srv.URL

	results, err := c.Search(context.Background(), "Apple Inc", 5, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	e := results[0]
	if e.LEI != "HWUPKR0MPOU8FGXBT394" {
		t.Errorf("LEI = %q", e.LEI)
	}
	if e.Name != "APPLE INC." {
		t.Errorf("Name = %q", e.Name)
	}
	if e.Country != "US" {
		t.Errorf("Country = %q", e.Country)
	}
	if e.Status != "ISSUED" {
		t.Errorf("Status = %q", e.Status)
	}
}

func TestGetByLEI(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/lei-records/HWUPKR0MPOU8FGXBT394" {
			t.Errorf("path = %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = w.Write([]byte(appleSingleJSON))
	}))
	defer srv.Close()

	c := NewClient()
	c.Rate = 0
	c.BaseURL = srv.URL

	e, err := c.GetByLEI(context.Background(), "HWUPKR0MPOU8FGXBT394")
	if err != nil {
		t.Fatal(err)
	}
	if e.LEI != "HWUPKR0MPOU8FGXBT394" {
		t.Errorf("LEI = %q", e.LEI)
	}
	if e.Name != "APPLE INC." {
		t.Errorf("Name = %q", e.Name)
	}
	if e.City != "CUPERTINO" {
		t.Errorf("City = %q", e.City)
	}
	if e.InitialDate != "2012-11-01" {
		t.Errorf("InitialDate = %q", e.InitialDate)
	}
}

func TestGetByLEINotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := NewClient()
	c.Rate = 0
	c.BaseURL = srv.URL

	_, err := c.GetByLEI(context.Background(), "NOTFOUND00000000000001")
	if err == nil {
		t.Error("expected error for 404, got nil")
	}
}

func TestSearchByName(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("filter[entity.legalName]") == "" {
			t.Error("missing filter[entity.legalName] parameter")
		}
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = w.Write([]byte(appleListJSON))
	}))
	defer srv.Close()

	c := NewClient()
	c.Rate = 0
	c.BaseURL = srv.URL

	results, err := c.SearchByName(context.Background(), "APPLE INC.")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if results[0].Name != "APPLE INC." {
		t.Errorf("Name = %q", results[0].Name)
	}
}
