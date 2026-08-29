// Command ghanageo is the public command-line interface to GhanaGeo.
//
// GhanaGeo is free, so no API key is required and anonymous use is a
// first-class path. A key is accepted only so a heavy consumer can be
// identified for fair-use accounting; it never unlocks anything.
//
//	ghanageo search "tema"
//	ghanageo districts --region 01KDVDNA00JR6256MY7B23EX4J --json | jq '.[].name'
//	ghanageo regions --csv > regions.csv
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/ghanageo/ghanageo-cli/internal/client"
	"github.com/ghanageo/ghanageo-cli/internal/render"
)

// Release metadata is overwritten at build time with -ldflags. Keeping the
// API target explicit lets operators see compatibility without making a
// request first.
var (
	version    = "dev"
	apiVersion = "v1"
)

const usage = `ghanageo — Ghana's location data, from your terminal

GhanaGeo is free. No account, no API key, no rate-limit upgrade to buy.

USAGE
  ghanageo <command> [flags]

COMMANDS
  regions                       List Ghana's 16 regions
  districts                     List districts (MMDAs)
  places                        List localities
  search <query>                Typo-tolerant search across all geography
  suggest <prefix>              Fast typeahead suggestions
  nearby <lat> <lng>            Places near a coordinate
  reverse <lat> <lng>           What geography contains a coordinate
  version                       Print the CLI version
  help                          Show this help

OUTPUT
  --json                        Emit JSON (for jq and scripts)
  --csv                         Emit CSV (for spreadsheets)
  default                       An aligned table for humans

COMMON FLAGS
  --limit N                     Maximum rows (default 20)
  --region ID                   Restrict to a region
  --district ID                 Restrict to a district
  --type TYPE                   Restrict to a place type (SUBURB, TOWN, …)
  --base-url URL                Point at a self-hosted instance
  --api-key KEY                 Optional; also read from GHANAGEO_API_KEY

EXAMPLES
  ghanageo search "tema comm 25"
  ghanageo districts --region 01KDVDNA00A63NSRPVSM94SCQD
  ghanageo regions --json | jq -r '.[].capital'
  ghanageo nearby 5.556 -0.182 --radius 10000
  ghanageo reverse 6.688 -1.624

GhanaGeo is free public infrastructure. If it is useful to you, consider
supporting it: https://geo.digitalghana.dev/support
`

type options struct {
	format     render.Format
	limit      int
	regionID   string
	districtID string
	placeType  string
	radius     int
	baseURL    string
	apiKey     string
	cursor     string
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		var apiErr *client.APIError
		if errors.As(err, &apiErr) {
			render.Warn("error: %s", apiErr.Message)
			render.Warn("  code: %s", apiErr.Code)
			if apiErr.Docs != "" {
				render.Warn("  docs: https://geo.digitalghana.dev%s", apiErr.Docs)
			}
			if apiErr.RequestID != "" {
				render.Warn("  request: %s", apiErr.RequestID)
			}
			os.Exit(2)
		}
		render.Warn("error: %v", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		fmt.Print(usage)
		return nil
	}

	cmd, rest := args[0], args[1:]
	switch cmd {
	case "help", "-h", "--help":
		fmt.Print(usage)
		return nil
	case "version", "--version", "-v":
		fmt.Printf("ghanageo %s (targets GhanaGeo API %s)\n", version, apiVersion)
		return nil
	}

	opts, positional, err := parseFlags(rest)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	c := client.New(opts.baseURL, opts.apiKey, version)

	switch cmd {
	case "regions":
		return cmdRegions(ctx, c, opts)
	case "districts":
		return cmdDistricts(ctx, c, opts, positional)
	case "places":
		return cmdPlaces(ctx, c, opts, positional)
	case "search":
		return cmdSearch(ctx, c, opts, positional)
	case "suggest":
		return cmdSuggest(ctx, c, opts, positional)
	case "nearby":
		return cmdNearby(ctx, c, opts, positional)
	case "reverse":
		return cmdReverse(ctx, c, opts, positional)
	default:
		fmt.Print(usage)
		return fmt.Errorf("unknown command %q", cmd)
	}
}

func parseFlags(args []string) (options, []string, error) {
	o := options{format: render.FormatTable, limit: 20}
	fs := flag.NewFlagSet("ghanageo", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	asJSON := fs.Bool("json", false, "emit JSON")
	asCSV := fs.Bool("csv", false, "emit CSV")
	fs.IntVar(&o.limit, "limit", 20, "maximum rows")
	fs.StringVar(&o.regionID, "region", "", "restrict to a region id")
	fs.StringVar(&o.districtID, "district", "", "restrict to a district id")
	fs.StringVar(&o.placeType, "type", "", "restrict to a place type")
	fs.IntVar(&o.radius, "radius", 5000, "radius in metres")
	fs.StringVar(&o.cursor, "cursor", "", "pagination cursor")
	fs.StringVar(&o.baseURL, "base-url", envOr("GHANAGEO_BASE_URL", client.DefaultBaseURL), "API base URL")
	fs.StringVar(&o.apiKey, "api-key", os.Getenv("GHANAGEO_API_KEY"), "optional API key")

	// Positional arguments may appear before flags, which Go's flag package
	// does not handle. Split them out first so `search accra --json` works.
	//
	// A leading "-" does NOT necessarily mean a flag: Ghana lies almost
	// entirely west of the prime meridian, so nearly every longitude in the
	// country is negative. Treating "-0.182" as a flag broke `nearby` and
	// `reverse` for practically the whole country.
	var positional []string
	var flagArgs []string
	for i := 0; i < len(args); i++ {
		if strings.HasPrefix(args[i], "-") && !isNegativeNumber(args[i]) {
			flagArgs = append(flagArgs, args[i:]...)
			break
		}
		positional = append(positional, args[i])
	}
	if err := fs.Parse(flagArgs); err != nil {
		return o, nil, err
	}
	positional = append(positional, fs.Args()...)

	if *asJSON && *asCSV {
		return o, nil, errors.New("choose either --json or --csv, not both")
	}
	if *asJSON {
		o.format = render.FormatJSON
	}
	if *asCSV {
		o.format = render.FormatCSV
	}
	return o, positional, nil
}

// isNegativeNumber reports whether an argument is a negative number rather
// than a flag. Without this, every Ghanaian longitude looks like a flag.
func isNegativeNumber(s string) bool {
	if !strings.HasPrefix(s, "-") || len(s) < 2 {
		return false
	}
	_, err := strconv.ParseFloat(s, 64)
	return err == nil
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// emit writes a table in whichever format was requested. JSON always renders
// the raw API objects rather than the flattened table, because a script wants
// the real shape.
func emit(o options, t render.Table, raw any) error {
	switch o.format {
	case render.FormatJSON:
		return render.WriteJSON(os.Stdout, raw)
	case render.FormatCSV:
		return render.WriteCSV(os.Stdout, t)
	default:
		if len(t.Rows) == 0 {
			fmt.Println("No results.")
			return nil
		}
		return render.WriteTable(os.Stdout, t)
	}
}

func datasetNote(version string) string {
	if version == "" {
		return ""
	}
	return "dataset " + version
}

func coordOf(c *client.Coordinate) string {
	if c == nil {
		return "—"
	}
	return fmt.Sprintf("%.4f, %.4f", c.Latitude, c.Longitude)
}

func cmdRegions(ctx context.Context, c *client.Client, o options) error {
	page, err := c.Regions(ctx, o.limit, o.cursor)
	if err != nil {
		return err
	}
	t := render.Table{
		Headers: []string{"id", "name", "capital", "status"},
		Note:    datasetNote(page.DatasetVersion),
	}
	for _, r := range page.Data {
		t.Rows = append(t.Rows, []string{r.ID, r.Name, dash(r.Capital), render.Status(r.VerificationStatus)})
	}
	return emit(o, t, page.Data)
}

func cmdDistricts(ctx context.Context, c *client.Client, o options, args []string) error {
	query := strings.Join(args, " ")
	page, err := c.Districts(ctx, o.regionID, query, o.limit, o.cursor)
	if err != nil {
		return err
	}
	t := render.Table{
		Headers: []string{"id", "name", "region", "capital", "status"},
		Note:    datasetNote(page.DatasetVersion),
	}
	for _, d := range page.Data {
		t.Rows = append(t.Rows, []string{d.ID, d.Name, d.Region.Name, dash(d.Capital), render.Status(d.VerificationStatus)})
	}
	return emit(o, t, page.Data)
}

func cmdPlaces(ctx context.Context, c *client.Client, o options, args []string) error {
	query := strings.Join(args, " ")
	page, err := c.Places(ctx, o.districtID, o.regionID, o.placeType, query, o.limit, o.cursor)
	if err != nil {
		return err
	}
	t := render.Table{
		Headers: []string{"id", "name", "type", "district", "region", "coordinates"},
		Note:    datasetNote(page.DatasetVersion),
	}
	for _, p := range page.Data {
		t.Rows = append(t.Rows, []string{
			p.ID, p.Name, p.Type, refName(p.District), refName(p.Region), coordOf(p.Centroid),
		})
	}
	return emit(o, t, page.Data)
}

func cmdSearch(ctx context.Context, c *client.Client, o options, args []string) error {
	if len(args) == 0 {
		return errors.New("search needs a query, e.g. ghanageo search \"tema\"")
	}
	page, err := c.Search(ctx, strings.Join(args, " "), o.regionID, o.placeType, o.limit)
	if err != nil {
		return err
	}
	t := render.Table{
		Headers: []string{"score", "name", "kind", "context", "why"},
		Note:    datasetNote(page.DatasetVersion),
	}
	for _, r := range page.Data {
		t.Rows = append(t.Rows, []string{
			strconv.FormatFloat(r.Score, 'f', 2, 64), r.Name, r.Kind, contextOf(r), r.MatchReason,
		})
	}
	return emit(o, t, page.Data)
}

func cmdSuggest(ctx context.Context, c *client.Client, o options, args []string) error {
	if len(args) == 0 {
		return errors.New("suggest needs a prefix, e.g. ghanageo suggest tem")
	}
	page, err := c.Autocomplete(ctx, strings.Join(args, " "), o.limit)
	if err != nil {
		return err
	}
	t := render.Table{Headers: []string{"name", "kind", "context"}, Note: datasetNote(page.DatasetVersion)}
	for _, r := range page.Data {
		t.Rows = append(t.Rows, []string{r.Name, r.Kind, contextOf(r)})
	}
	return emit(o, t, page.Data)
}

func cmdNearby(ctx context.Context, c *client.Client, o options, args []string) error {
	lat, lng, err := parseLatLng(args)
	if err != nil {
		return err
	}
	page, err := c.Nearby(ctx, lat, lng, o.radius, o.limit)
	if err != nil {
		return err
	}
	t := render.Table{
		Headers: []string{"id", "name", "type", "district", "coordinates"},
		Note:    datasetNote(page.DatasetVersion),
	}
	for _, p := range page.Data {
		t.Rows = append(t.Rows, []string{p.ID, p.Name, p.Type, refName(p.District), coordOf(p.Centroid)})
	}
	if len(page.Data) == 0 && o.format == render.FormatTable {
		render.Warn("No places within %dm. The bootstrap dataset carries no coordinates yet.", o.radius)
	}
	return emit(o, t, page.Data)
}

func cmdReverse(ctx context.Context, c *client.Client, o options, args []string) error {
	lat, lng, err := parseLatLng(args)
	if err != nil {
		return err
	}
	res, err := c.Reverse(ctx, lat, lng)
	if err != nil {
		return err
	}
	if o.format == render.FormatJSON {
		return render.WriteJSON(os.Stdout, res)
	}
	t := render.Table{Headers: []string{"level", "name"}, Note: datasetNote(res.DatasetVersion)}
	if res.Region != nil {
		t.Rows = append(t.Rows, []string{"region", res.Region.Name})
	}
	if res.District != nil {
		t.Rows = append(t.Rows, []string{"district", res.District.Name})
	}
	for _, p := range res.Nearby {
		t.Rows = append(t.Rows, []string{"nearby", p.Name})
	}
	if len(t.Rows) == 0 {
		fmt.Println("Nothing found at that coordinate.")
		return nil
	}
	if o.format == render.FormatCSV {
		return render.WriteCSV(os.Stdout, t)
	}
	return render.WriteTable(os.Stdout, t)
}

func parseLatLng(args []string) (float64, float64, error) {
	if len(args) < 2 {
		return 0, 0, errors.New("expected a latitude and a longitude, e.g. 5.556 -0.182")
	}
	lat, err := strconv.ParseFloat(args[0], 64)
	if err != nil {
		return 0, 0, fmt.Errorf("latitude %q is not a number", args[0])
	}
	lng, err := strconv.ParseFloat(args[1], 64)
	if err != nil {
		return 0, 0, fmt.Errorf("longitude %q is not a number", args[1])
	}
	return lat, lng, nil
}

func refName(r *client.Ref) string {
	if r == nil || r.Name == "" {
		return "—"
	}
	return r.Name
}

func contextOf(r client.SearchResult) string {
	parts := make([]string, 0, 2)
	if r.DistrictName != "" {
		parts = append(parts, r.DistrictName)
	}
	if r.RegionName != "" {
		parts = append(parts, r.RegionName)
	}
	if len(parts) == 0 {
		return "—"
	}
	return strings.Join(parts, " · ")
}

func dash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "—"
	}
	return s
}
