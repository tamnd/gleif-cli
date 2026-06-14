package gleif

import (
	"testing"
)

// These tests are offline: they exercise the URI driver's pure string
// functions, which need no network.

func TestDomainInfo(t *testing.T) {
	info := Domain{}.Info()
	if info.Scheme != "gleif" {
		t.Errorf("Scheme = %q, want gleif", info.Scheme)
	}
	if len(info.Hosts) == 0 || info.Hosts[0] != Host {
		t.Errorf("Hosts = %v, want [%s]", info.Hosts, Host)
	}
	if info.Identity.Binary != "gleif" {
		t.Errorf("Identity.Binary = %q, want gleif", info.Identity.Binary)
	}
}

func TestClassify(t *testing.T) {
	cases := []struct {
		in  string
		typ string
		id  string
	}{
		// LEI codes: 20-char all-caps alphanumeric
		{"HWUPKR0MPOU8FGXBT394", "lei", "HWUPKR0MPOU8FGXBT394"},
		{"EVK05KS7XY1DEII3R011", "lei", "EVK05KS7XY1DEII3R011"},
		// Name searches
		{"Apple Inc", "search", "Apple Inc"},
		{"Goldman Sachs", "search", "Goldman Sachs"},
	}
	for _, tc := range cases {
		typ, id, err := Domain{}.Classify(tc.in)
		if err != nil || typ != tc.typ || id != tc.id {
			t.Errorf("Classify(%q) = (%q, %q, %v), want (%q, %q, nil)",
				tc.in, typ, id, err, tc.typ, tc.id)
		}
	}
}

func TestClassifyEmpty(t *testing.T) {
	_, _, err := Domain{}.Classify("")
	if err == nil {
		t.Error("Classify(\"\") should return an error")
	}
}

func TestLocate(t *testing.T) {
	got, err := Domain{}.Locate("lei", "HWUPKR0MPOU8FGXBT394")
	want := "https://www.gleif.org/lei/HWUPKR0MPOU8FGXBT394"
	if err != nil || got != want {
		t.Errorf("Locate = (%q, %v), want (%q, nil)", got, err, want)
	}
}

func TestLocateSearch(t *testing.T) {
	got, err := Domain{}.Locate("search", "Apple")
	if err != nil || got == "" {
		t.Errorf("Locate(search) = (%q, %v), want non-empty URL", got, err)
	}
}

func TestLocateUnknownType(t *testing.T) {
	_, err := Domain{}.Locate("unknown", "foo")
	if err == nil {
		t.Error("Locate(unknown) should return an error")
	}
}

func TestIsLEI(t *testing.T) {
	cases := []struct {
		s    string
		want bool
	}{
		{"HWUPKR0MPOU8FGXBT394", true},
		{"EVK05KS7XY1DEII3R011", true},
		{"hwupkr0mpou8fgxbt394", false}, // lowercase
		{"HWUPKR0MPOU8FGXBT39", false},  // 19 chars
		{"HWUPKR0MPOU8FGXBT3945", false}, // 21 chars
		{"Apple Inc", false},
	}
	for _, tc := range cases {
		got := isLEI(tc.s)
		if got != tc.want {
			t.Errorf("isLEI(%q) = %v, want %v", tc.s, got, tc.want)
		}
	}
}

func TestDateOnly(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"2012-11-01T00:00:00.000Z", "2012-11-01"},
		{"2025-01-01", "2025-01-01"},
		{"", ""},
		{"2025", "2025"},
	}
	for _, tc := range cases {
		got := dateOnly(tc.in)
		if got != tc.want {
			t.Errorf("dateOnly(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestToEntity(t *testing.T) {
	r := wireRecord{
		ID: "HWUPKR0MPOU8FGXBT394",
		Attributes: wireAttributes{
			LEI: "HWUPKR0MPOU8FGXBT394",
			Entity: wireEntity{
				LegalName: wireName{Name: "APPLE INC."},
				LegalAddress: wireAddr{
					Country: "US",
					Region:  "US-CA",
					City:    "CUPERTINO",
				},
				Jurisdiction: "US-CA",
				Category:     "GENERAL",
			},
			Registration: wireReg{
				Status:                  "ISSUED",
				InitialRegistrationDate: "2012-11-01T00:00:00.000Z",
				LastUpdateDate:          "2025-01-01T00:00:00.000Z",
				NextRenewalDate:         "2025-02-01T00:00:00.000Z",
			},
		},
	}

	e := toEntity(r)
	if e.LEI != "HWUPKR0MPOU8FGXBT394" {
		t.Errorf("LEI = %q", e.LEI)
	}
	if e.Name != "APPLE INC." {
		t.Errorf("Name = %q", e.Name)
	}
	if e.Country != "US" {
		t.Errorf("Country = %q", e.Country)
	}
	if e.City != "CUPERTINO" {
		t.Errorf("City = %q", e.City)
	}
	if e.Status != "ISSUED" {
		t.Errorf("Status = %q", e.Status)
	}
	if e.InitialDate != "2012-11-01" {
		t.Errorf("InitialDate = %q", e.InitialDate)
	}
	if e.LastUpdate != "2025-01-01" {
		t.Errorf("LastUpdate = %q", e.LastUpdate)
	}
	if e.NextRenewal != "2025-02-01" {
		t.Errorf("NextRenewal = %q", e.NextRenewal)
	}
}
