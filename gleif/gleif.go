// Package gleif is the library behind the gleif command line:
// the HTTP client, request shaping, and the typed data models for the GLEIF
// LEI (Legal Entity Identifier) API.
//
// The Client here is the spine every command shares. It sets a real
// User-Agent, paces requests so a busy session stays polite, and retries the
// transient failures (429 and 5xx) that any public API throws under load.
package gleif

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// Host is the API host this client talks to.
const Host = "api.gleif.org"

// BaseURL is the root every request is built from.
const BaseURL = "https://api.gleif.org/api/v1"

// DefaultUserAgent identifies the client to GLEIF. An honest User-Agent is
// both polite and the thing most likely to keep you unblocked.
const DefaultUserAgent = "gleif-cli/0.1 (tamnd87@gmail.com)"

// Client talks to the GLEIF API over HTTP.
type Client struct {
	HTTP      *http.Client
	UserAgent string
	BaseURL   string
	// Rate is the minimum gap between requests. Zero means no pacing.
	Rate    time.Duration
	Retries int

	last time.Time
}

// NewClient returns a Client with sensible defaults: a 15s timeout, a 200ms
// minimum gap between requests, and three retries on transient errors.
func NewClient() *Client {
	return &Client{
		HTTP:      &http.Client{Timeout: 15 * time.Second},
		UserAgent: DefaultUserAgent,
		BaseURL:   BaseURL,
		Rate:      200 * time.Millisecond,
		Retries:   3,
	}
}

// Get fetches u and returns the response body. It paces and retries according
// to the client's settings. The caller owns nothing extra; the body is read
// fully and closed here.
func (c *Client) Get(ctx context.Context, u string) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt <= c.Retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff(attempt)):
			}
		}
		body, retry, err := c.do(ctx, u)
		if err == nil {
			return body, nil
		}
		lastErr = err
		if !retry {
			return nil, err
		}
	}
	return nil, fmt.Errorf("get %s: %w", u, lastErr)
}

func (c *Client) do(ctx context.Context, u string) (body []byte, retry bool, err error) {
	c.pace()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("User-Agent", c.UserAgent)
	req.Header.Set("Accept", "application/vnd.api+json")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, true, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return nil, true, fmt.Errorf("http %d", resp.StatusCode)
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, false, fmt.Errorf("not found")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("http %d", resp.StatusCode)
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, true, err
	}
	return b, false, nil
}

// pace blocks until at least Rate has passed since the previous request.
func (c *Client) pace() {
	if c.Rate <= 0 {
		return
	}
	if wait := c.Rate - time.Since(c.last); wait > 0 {
		time.Sleep(wait)
	}
	c.last = time.Now()
}

func backoff(attempt int) time.Duration {
	d := time.Duration(attempt) * 500 * time.Millisecond
	if d > 5*time.Second {
		d = 5 * time.Second
	}
	return d
}

// --- Entity: the canonical output type ---

// Entity is one legal entity record from the GLEIF LEI registry.
type Entity struct {
	LEI          string `kit:"id" json:"lei"`
	Name         string `json:"name"`
	Country      string `json:"country"`
	Region       string `json:"region"`
	City         string `json:"city"`
	Jurisdiction string `json:"jurisdiction"`
	Category     string `json:"category"`
	Status       string `json:"status"`
	InitialDate  string `json:"initial_date"`
	LastUpdate   string `json:"last_update"`
	NextRenewal  string `json:"next_renewal"`
}

// --- Wire types (JSON API format) ---

type wireListResp struct {
	Data []wireRecord `json:"data"`
}

type wireSingleResp struct {
	Data wireRecord `json:"data"`
}

type wireRecord struct {
	ID         string         `json:"id"`
	Attributes wireAttributes `json:"attributes"`
}

type wireAttributes struct {
	LEI          string     `json:"lei"`
	Entity       wireEntity `json:"entity"`
	Registration wireReg    `json:"registration"`
}

type wireEntity struct {
	LegalName    wireName `json:"legalName"`
	LegalAddress wireAddr `json:"legalAddress"`
	Jurisdiction string   `json:"jurisdiction"`
	Category     string   `json:"category"`
}

type wireName struct {
	Name string `json:"name"`
}

type wireAddr struct {
	Country string `json:"country"`
	Region  string `json:"region"`
	City    string `json:"city"`
}

type wireReg struct {
	Status                  string `json:"status"`
	InitialRegistrationDate string `json:"initialRegistrationDate"`
	LastUpdateDate          string `json:"lastUpdateDate"`
	NextRenewalDate         string `json:"nextRenewalDate"`
}

func toEntity(r wireRecord) Entity {
	attr := r.Attributes
	return Entity{
		LEI:          attr.LEI,
		Name:         attr.Entity.LegalName.Name,
		Country:      attr.Entity.LegalAddress.Country,
		Region:       attr.Entity.LegalAddress.Region,
		City:         attr.Entity.LegalAddress.City,
		Jurisdiction: attr.Entity.Jurisdiction,
		Category:     attr.Entity.Category,
		Status:       attr.Registration.Status,
		InitialDate:  dateOnly(attr.Registration.InitialRegistrationDate),
		LastUpdate:   dateOnly(attr.Registration.LastUpdateDate),
		NextRenewal:  dateOnly(attr.Registration.NextRenewalDate),
	}
}

func dateOnly(s string) string {
	if len(s) >= 10 {
		return s[:10]
	}
	return s
}

// --- API methods ---

// Search searches for entities by full-text query.
func (c *Client) Search(ctx context.Context, query string, pageSize, page int) ([]Entity, error) {
	if pageSize <= 0 {
		pageSize = 10
	}
	if page <= 0 {
		page = 1
	}
	params := url.Values{}
	params.Set("filter[fulltext]", query)
	params.Set("page[size]", strconv.Itoa(pageSize))
	params.Set("page[number]", strconv.Itoa(page))
	u := c.BaseURL + "/lei-records?" + params.Encode()

	body, err := c.Get(ctx, u)
	if err != nil {
		return nil, err
	}

	var resp wireListResp
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}

	out := make([]Entity, 0, len(resp.Data))
	for _, r := range resp.Data {
		out = append(out, toEntity(r))
	}
	return out, nil
}

// GetByLEI fetches a single entity by its LEI code.
func (c *Client) GetByLEI(ctx context.Context, lei string) (*Entity, error) {
	u := c.BaseURL + "/lei-records/" + url.PathEscape(lei)
	body, err := c.Get(ctx, u)
	if err != nil {
		return nil, err
	}

	var resp wireSingleResp
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}

	e := toEntity(resp.Data)
	return &e, nil
}

// SearchByName searches for entities by exact legal name.
func (c *Client) SearchByName(ctx context.Context, name string) ([]Entity, error) {
	params := url.Values{}
	params.Set("filter[entity.legalName]", name)
	params.Set("page[size]", "10")
	u := c.BaseURL + "/lei-records?" + params.Encode()

	body, err := c.Get(ctx, u)
	if err != nil {
		return nil, err
	}

	var resp wireListResp
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}

	out := make([]Entity, 0, len(resp.Data))
	for _, r := range resp.Data {
		out = append(out, toEntity(r))
	}
	return out, nil
}
