package gleif

import (
	"context"
	"strings"
	"unicode"

	"github.com/tamnd/any-cli/kit"
	"github.com/tamnd/any-cli/kit/errs"
)

// domain.go exposes the GLEIF LEI registry as a kit Domain: a driver that a
// multi-domain host (ant) enables with a single blank import,
//
//	import _ "github.com/tamnd/gleif-cli/gleif"
//
// The init below registers it; the host then dereferences gleif:// URIs by
// routing to the operations Register installs. The same Domain also builds the
// standalone gleif binary (see cli.NewApp), so the binary and a host share one
// source of truth.
func init() { kit.Register(Domain{}) }

// Domain is the gleif driver. It carries no state; the per-run client is
// built by the factory Register hands kit.
type Domain struct{}

// Info describes the scheme, the hostnames a pasted link is matched against,
// and the identity reused for the binary's help and version.
func (Domain) Info() kit.DomainInfo {
	return kit.DomainInfo{
		Scheme: "gleif",
		Hosts:  []string{Host},
		Identity: kit.Identity{
			Binary: "gleif",
			Short:  "Query the GLEIF LEI registry for legal entity data.",
			Long: `gleif reads public Legal Entity Identifier (LEI) data from the GLEIF
API (api.gleif.org) and prints clean records that pipe into the rest of your
tools. No API key required.`,
			Site: "gleif.org",
			Repo: "https://github.com/tamnd/gleif-cli",
		},
	}
}

// Register installs the client factory and every operation onto app.
func (Domain) Register(app *kit.App) {
	app.SetClient(newClient)

	// search: full-text search by name.
	kit.Handle(app, kit.OpMeta{
		Name:    "search",
		Group:   "read",
		List:    true,
		Summary: "Search entities by name",
		Args:    []kit.Arg{{Name: "query", Help: "name or keyword to search for"}},
	}, searchOp)

	// lei: look up a single entity by its LEI code.
	kit.Handle(app, kit.OpMeta{
		Name:     "lei",
		Group:    "read",
		Single:   true,
		Summary:  "Look up an entity by LEI code",
		URIType:  "lei",
		Resolver: true,
		Args:     []kit.Arg{{Name: "lei", Help: "20-character LEI code"}},
	}, leiOp)

	// entity: search by exact legal name.
	kit.Handle(app, kit.OpMeta{
		Name:    "entity",
		Group:   "read",
		List:    true,
		Summary: "Search entities by exact legal name",
		Args:    []kit.Arg{{Name: "name", Help: "exact legal name"}},
	}, entityOp)
}

// newClient builds the client from the host-resolved config.
func newClient(_ context.Context, cfg kit.Config) (any, error) {
	c := NewClient()
	if cfg.UserAgent != "" {
		c.UserAgent = cfg.UserAgent
	}
	if cfg.Rate > 0 {
		c.Rate = cfg.Rate
	}
	if cfg.Retries > 0 {
		c.Retries = cfg.Retries
	}
	if cfg.Timeout > 0 {
		c.HTTP.Timeout = cfg.Timeout
	}
	return c, nil
}

// --- inputs ---

type searchInput struct {
	Query    string  `kit:"arg"            help:"name or keyword to search for"`
	PageSize int     `kit:"flag"           help:"number of results per page"`
	Page     int     `kit:"flag"           help:"page number (1-based)"`
	Client   *Client `kit:"inject"`
}

type leiInput struct {
	LEI    string  `kit:"arg"    help:"20-character LEI code"`
	Client *Client `kit:"inject"`
}

type entityInput struct {
	Name   string  `kit:"arg"    help:"exact legal name"`
	Client *Client `kit:"inject"`
}

// --- handlers ---

func searchOp(ctx context.Context, in searchInput, emit func(*Entity) error) error {
	results, err := in.Client.Search(ctx, in.Query, in.PageSize, in.Page)
	if err != nil {
		return mapErr(err)
	}
	for i := range results {
		if err := emit(&results[i]); err != nil {
			return err
		}
	}
	return nil
}

func leiOp(ctx context.Context, in leiInput, emit func(*Entity) error) error {
	e, err := in.Client.GetByLEI(ctx, in.LEI)
	if err != nil {
		return mapErr(err)
	}
	return emit(e)
}

func entityOp(ctx context.Context, in entityInput, emit func(*Entity) error) error {
	results, err := in.Client.SearchByName(ctx, in.Name)
	if err != nil {
		return mapErr(err)
	}
	for i := range results {
		if err := emit(&results[i]); err != nil {
			return err
		}
	}
	return nil
}

// --- Resolver: the URI-native string functions, pure and network-free ---

// Classify turns any accepted input into the canonical (type, id).
// A 20-character all-caps alphanumeric string is treated as a LEI code.
// Anything else is treated as a search query.
func (Domain) Classify(input string) (uriType, id string, err error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", "", errs.Usage("empty GLEIF reference")
	}
	if isLEI(input) {
		return "lei", input, nil
	}
	return "search", input, nil
}

// Locate is the inverse: the live https URL for a (type, id).
func (Domain) Locate(uriType, id string) (string, error) {
	switch uriType {
	case "lei":
		return "https://www.gleif.org/lei/" + id, nil
	case "search":
		return "https://www.gleif.org/en/lei-data/global-lei-index/lei-search#?q=" + id, nil
	default:
		return "", errs.Usage("gleif has no resource type %q", uriType)
	}
}

// --- helpers ---

// isLEI returns true if s looks like a LEI code: exactly 20 uppercase
// alphanumeric characters.
func isLEI(s string) bool {
	if len(s) != 20 {
		return false
	}
	for _, r := range s {
		if !unicode.IsUpper(r) && !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

// mapErr converts a library error into the kit error kind that carries the
// right exit code.
func mapErr(err error) error {
	if err != nil && strings.Contains(err.Error(), "not found") {
		return errs.NotFound("%s", err.Error())
	}
	return err
}
