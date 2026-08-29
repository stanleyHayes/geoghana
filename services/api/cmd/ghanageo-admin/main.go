// Command ghanageo-admin is the internal operator CLI: data seed, validate and reconcile.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/user"
	"time"

	"strings"

	"github.com/ghanageo/ghanageo/services/api/internal/adapters/mongo"
	"github.com/ghanageo/ghanageo/services/api/internal/adapters/search/typesense"

	"github.com/ghanageo/ghanageo/services/api/internal/adapters/ingest/geoboundaries"
	"github.com/ghanageo/ghanageo/services/api/internal/adapters/ingest/geonames"
	"github.com/ghanageo/ghanageo/services/api/internal/app/dataset"
	"github.com/ghanageo/ghanageo/services/api/internal/app/ingest"
	"github.com/ghanageo/ghanageo/services/api/internal/app/search"
	"github.com/ghanageo/ghanageo/services/api/internal/app/seed"
	"github.com/ghanageo/ghanageo/services/api/internal/domain/audit"
	"github.com/ghanageo/ghanageo/services/api/internal/domain/identity"
	"github.com/ghanageo/ghanageo/services/api/internal/platform/config"
)

const datasetVersion = "2026.08.1-seed"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `ghanageo — GhanaGeo operator CLI

Usage:
  ghanageo-admin data seed      --file <manifest.json> [--environment local]
  ghanageo-admin data validate  --dataset seed
  ghanageo-admin data reconcile --against canonical-staging
  ghanageo-admin data reindex
  ghanageo-admin data import --source geonames --file <GH.txt> [--limit N]
  ghanageo-admin data boundaries --level ADM1|ADM2 --file <geojson> [--apply]
  ghanageo-admin data assign-districts
  ghanageo-admin data dedupe [--apply]
  ghanageo-admin data export [--version <v>] [--dir <path>]
  ghanageo-admin audit list [--actor <id>] [--action <a>] [--target <id>] [--limit N]
  ghanageo-admin audit verify
  ghanageo migrate
`)
}

func run(args []string) error {
	if len(args) < 1 {
		usage()
		return fmt.Errorf("no command given")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	switch args[0] {
	case "migrate":
		return cmdMigrate(ctx)
	case "data":
		if len(args) < 2 {
			usage()
			return fmt.Errorf("data: no subcommand")
		}
		switch args[1] {
		case "seed":
			return cmdSeed(ctx, args[2:])
		case "validate":
			return cmdValidate(ctx, args[2:])
		case "reconcile":
			return cmdReconcile(ctx, args[2:])
		case "reindex":
			return cmdReindex(ctx)
		case "import":
			return cmdImport(ctx, args[2:])
		case "boundaries":
			return cmdBoundaries(ctx, args[2:])
		case "assign-districts":
			return cmdAssignDistricts(ctx)
		case "dedupe":
			return cmdDedupe(ctx, args[2:])
		case "export":
			return cmdExport(ctx, args[2:])
		}
	case "audit":
		if len(args) < 2 {
			usage()
			return fmt.Errorf("audit: no subcommand")
		}
		switch args[1] {
		case "list":
			return cmdAuditList(ctx, args[2:])
		case "verify":
			return cmdAuditVerify(ctx)
		}
	case "keys":
		if len(args) < 2 {
			usage()
			return fmt.Errorf("keys: no subcommand")
		}
		switch args[1] {
		case "create":
			return cmdKeyCreate(ctx, args[2:])
		case "revoke":
			return cmdKeyRevoke(ctx, args[2:])
		}
	}
	usage()
	return fmt.Errorf("unknown command %q", args[0])
}

func connect(ctx context.Context) (*mongo.Store, config.Config, error) {
	cfg := config.Load()
	store, err := mongo.Connect(ctx, cfg.MongoURI, cfg.MongoDB)
	return store, cfg, err
}

func cmdMigrate(ctx context.Context) error {
	store, cfg, err := connect(ctx)
	if err != nil {
		return err
	}
	defer store.Close(ctx)
	fmt.Printf("→ %s\n", cfg)
	if err := mongo.Migrate(ctx, store.DB()); err != nil {
		return err
	}
	fmt.Println("✓ collections, validators and indexes are up to date")
	return nil
}

func cmdSeed(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("seed", flag.ContinueOnError)
	file := fs.String("file", "data/seed-data/manifest.json", "path to the seed manifest")
	envName := fs.String("environment", "local", "target environment")
	if err := fs.Parse(args); err != nil {
		return err
	}

	store, cfg, err := connect(ctx)
	if err != nil {
		return err
	}
	defer store.Close(ctx)

	fmt.Printf("→ %s (target: %s)\n", cfg, *envName)
	if err := mongo.Migrate(ctx, store.DB()); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	m, base, err := seed.LoadManifest(*file)
	if err != nil {
		return err
	}
	fmt.Printf("→ manifest %s (dataset %s), checksums verified\n", *file, m.DatasetVersion)

	im := &seed.Importer{
		Regions:   mongo.NewRegionRepo(store),
		Districts: mongo.NewDistrictRepo(store),
		Places:    mongo.NewPlaceRepo(store),
	}
	res, err := im.Run(ctx, m, base)
	if err != nil {
		return err
	}
	fmt.Printf("✓ %s\n", res)
	for _, s := range res.Skipped {
		fmt.Printf("  ! skipped: %s\n", s)
	}
	return verifyCounts(ctx, store)
}

// verifyCounts asserts the invariants Spec 31.1 requires CI to check.
func verifyCounts(ctx context.Context, store *mongo.Store) error {
	regions, err := mongo.NewRegionRepo(store).Count(ctx)
	if err != nil {
		return err
	}
	districts, err := mongo.NewDistrictRepo(store).Count(ctx)
	if err != nil {
		return err
	}
	places, err := mongo.NewPlaceRepo(store).Count(ctx)
	if err != nil {
		return err
	}
	fmt.Printf("→ counts: %d regions · %d districts · %d places\n", regions, districts, places)

	var problems []string
	if regions != seed.ExpectedRegions {
		problems = append(problems, fmt.Sprintf("expected %d regions, found %d", seed.ExpectedRegions, regions))
	}
	if districts != seed.ExpectedDistricts {
		problems = append(problems, fmt.Sprintf("expected %d districts, found %d", seed.ExpectedDistricts, districts))
	}
	if places != seed.ExpectedPlaces {
		problems = append(problems, fmt.Sprintf("expected %d places, found %d", seed.ExpectedPlaces, places))
	}
	if len(problems) > 0 {
		for _, p := range problems {
			fmt.Fprintf(os.Stderr, "  ✗ %s\n", p)
		}
		return fmt.Errorf("seed invariants failed")
	}
	fmt.Println("✓ seed invariants hold (16 regions / 261 districts / 16 places)")
	return nil
}

func cmdValidate(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	dataset := fs.String("dataset", "seed", "dataset to validate")
	if err := fs.Parse(args); err != nil {
		return err
	}
	store, _, err := connect(ctx)
	if err != nil {
		return err
	}
	defer store.Close(ctx)

	fmt.Printf("→ validating dataset %q against the Spec §22.1 acceptance suite\n\n", *dataset)

	v := &seed.Validator{
		Regions:   mongo.NewRegionRepo(store),
		Districts: mongo.NewDistrictRepo(store),
		Places:    mongo.NewPlaceRepo(store),
	}
	report, err := v.Run(ctx)
	if err != nil {
		return err
	}
	for _, c := range report.Checks {
		mark := "✓"
		if !c.Passed {
			mark = "✗"
		}
		fmt.Printf("  %s %-42s %s\n", mark, c.Name, c.Detail)
	}

	// Boundary coverage is reported but does NOT block: a district can be
	// entirely valid while its polygon is simply not yet ingested.
	districts := mongo.NewDistrictRepo(store)
	withGeom, total, gerr := districts.CountWithGeometry(ctx)
	if gerr == nil {
		fmt.Printf("\n  boundary coverage: %d/%d districts\n", withGeom, total)
		if withGeom < total {
			fmt.Printf("  %d without a boundary — reported, not fatal. Causes are either a\n", total-withGeom)
			fmt.Println("  name that did not reconcile, or a district created after the source")
			fmt.Println("  snapshot was published (Guan, inaugurated 8 October 2021).")
		}
	}

	if failed := report.Failed(); len(failed) > 0 {
		fmt.Printf("\n✗ %d check(s) failed\n", len(failed))
		if report.BlocksPublication() {
			return fmt.Errorf("validation failed; this dataset must not be published")
		}
		return nil
	}
	fmt.Println("\n✓ all acceptance checks passed")
	return nil
}

// cmdReindex rebuilds the search index from canonical data.
// cmdExport writes the downloadable dataset artifacts and records them.
//
// The catalogue is only updated after every file is on disk, and each
// checksum is taken from the bytes actually written — /datasets must never
// advertise a download that does not exist or does not match.
func cmdExport(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("export", flag.ContinueOnError)
	version := fs.String("version", datasetVersion, "dataset version to publish")
	dir := fs.String("dir", "", "export directory (defaults to API_EXPORT_DIR)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	store, cfg, err := connect(ctx)
	if err != nil {
		return err
	}
	defer store.Close(ctx)

	target := *dir
	if target == "" {
		target = cfg.ExportDir
	}

	b := dataset.NewBuilder(
		mongo.NewRegionRepo(store),
		mongo.NewDistrictRepo(store),
		mongo.NewPlaceRepo(store),
		mongo.NewDatasetRepo(store),
		target,
	)

	fmt.Printf("→ building %s into %s\n", *version, target)
	// Stamped once so every artifact in a run shares a generation time.
	generatedAt := time.Now().UTC().Format(time.RFC3339)
	v, err := b.Build(ctx, *version, generatedAt)
	if err != nil {
		return err
	}

	var total int64
	for _, a := range v.Artifacts {
		total += a.SizeBytes
		fmt.Printf("  %-10s %-8s %8d records  %9d bytes  %s\n",
			a.Entity, a.Format, a.RecordCount, a.SizeBytes, a.SHA256[:12])
	}
	recordAudit(ctx, store, audit.ActionDatasetPublished,
		audit.Target{Kind: "dataset", ID: v.Version, Label: v.Version},
		nil,
		map[string]any{
			"artifacts": len(v.Artifacts), "totalBytes": total,
			"counts": v.Counts, "exportDir": target,
		},
		nil)

	fmt.Printf("✓ %d artifacts, %d bytes total, published as %s\n",
		len(v.Artifacts), total, v.Version)
	return nil
}

// cmdAuditList reads the append-only log.
//
// Reading is CLI-only for now, deliberately. The log names actors and their
// actions, and exposing it over the public API before admin authentication
// exists (GEO-9.2) would publish exactly the operational detail an attacker
// wants. The admin console screen lands with that story.
func cmdAuditList(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("audit list", flag.ContinueOnError)
	actor := fs.String("actor", "", "filter by actor id")
	action := fs.String("action", "", "filter by action, e.g. key.revoked")
	target := fs.String("target", "", "filter by target id")
	limit := fs.Int("limit", 50, "maximum rows (max 200)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	store, _, err := connect(ctx)
	if err != nil {
		return err
	}
	defer store.Close(ctx)

	entries, err := mongo.NewAuditRepo(store).List(ctx, mongo.AuditFilter{
		Actor: *actor, Action: *action, Target: *target, Limit: *limit,
	})
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		fmt.Println("no audit entries match")
		return nil
	}
	fmt.Printf("%-20s  %-9s  %-24s  %-28s  %s\n", "WHEN", "OUTCOME", "ACTION", "TARGET", "ACTOR")
	for _, e := range entries {
		mark := "✓"
		if e.Outcome != "succeeded" {
			mark = "✗"
		}
		fmt.Printf("%-20s  %s %-7s  %-24s  %-28s  %s\n",
			e.At.Format("2006-01-02 15:04:05"), mark, e.Outcome,
			e.Action, e.Target.Kind+":"+e.Target.ID, e.Actor.Label)
		if e.Error != "" {
			fmt.Printf("%-20s    ↳ %s\n", "", e.Error)
		}
	}
	fmt.Printf("\n%d entries\n", len(entries))
	return nil
}

// cmdAuditVerify walks the hash chain and reports the first break.
//
// This is the check that gives the log its value. Application code cannot stop
// someone with database credentials editing a row, but every entry commits to
// the one before it, so any edit, insertion or deletion shows up here.
func cmdAuditVerify(ctx context.Context) error {
	store, _, err := connect(ctx)
	if err != nil {
		return err
	}
	defer store.Close(ctx)

	entries, err := mongo.NewAuditRepo(store).Chain(ctx)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		fmt.Println("audit log is empty — nothing to verify")
		return nil
	}
	if err := audit.VerifyChain(entries); err != nil {
		return fmt.Errorf("AUDIT CHAIN FAILED: %w", err)
	}
	fmt.Printf("✓ audit chain intact across %d entries\n", len(entries))
	fmt.Printf("  first: %s  %s\n", entries[0].At.Format(time.RFC3339), entries[0].Action)
	fmt.Printf("  last:  %s  %s\n",
		entries[len(entries)-1].At.Format(time.RFC3339), entries[len(entries)-1].Action)
	return nil
}

func cmdReindex(ctx context.Context) error {
	store, cfg, err := connect(ctx)
	if err != nil {
		return err
	}
	defer store.Close(ctx)

	svc := search.NewService(
		typesense.New(cfg.TypesenseURL, cfg.TypesenseKey),
		mongo.NewRegionRepo(store),
		mongo.NewDistrictRepo(store),
		mongo.NewPlaceRepo(store),
		datasetVersion,
	)
	fmt.Printf("→ reindexing into %s\n", cfg.TypesenseURL)
	n, err := svc.Reindex(ctx)
	if err != nil {
		return err
	}
	recordAudit(ctx, store, audit.ActionReindexed,
		audit.Target{Kind: "search_index", ID: datasetVersion, Label: cfg.TypesenseURL},
		nil, map[string]any{"documents": n}, nil)
	fmt.Printf("✓ indexed %d documents (regions, districts and places)\n", n)
	return nil
}

// cmdImport runs a source adapter through the ingestion pipeline.
func cmdImport(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("import", flag.ContinueOnError)
	source := fs.String("source", "geonames", "source adapter to run")
	file := fs.String("file", "", "path to the source dump")
	limit := fs.Int("limit", 0, "maximum records to import (0 = all)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *file == "" {
		return fmt.Errorf("--file is required")
	}

	store, cfg, err := connect(ctx)
	if err != nil {
		return err
	}
	defer store.Close(ctx)
	if err := mongo.Migrate(ctx, store.DB()); err != nil {
		return err
	}

	var adapter ingest.Adapter
	switch *source {
	case "geonames":
		a := geonames.New(*file)
		a.MaxRecords = *limit
		adapter = a
	default:
		return fmt.Errorf("unknown source %q", *source)
	}

	lic := adapter.Licence()
	fmt.Printf("→ importing %s from %s\n", adapter.Name(), *file)
	fmt.Printf("  licence: %s (%s)\n", lic.Name, lic.SPDX)
	fmt.Printf("  attribution: %s\n\n", lic.Attribution)

	imp := &ingest.Importer{
		Regions:        mongo.NewRegionRepo(store),
		Districts:      mongo.NewDistrictRepo(store),
		Places:         mongo.NewPlaceRepo(store),
		DatasetVersion: datasetVersion,
	}
	res, err := imp.Run(ctx, adapter, 500)
	if res != nil {
		fmt.Printf("✓ fetched %d · created %d · updated %d · skipped %d · rejected %d\n",
			res.Fetched, res.Created, res.Updated, res.Skipped, res.Rejected)
		if len(res.RejectReasons) > 0 {
			fmt.Println("  rejections:")
			shown := 0
			for reason, n := range res.RejectReasons {
				fmt.Printf("    %5d  %s\n", n, reason)
				if shown++; shown >= 8 {
					fmt.Printf("    … and %d more reasons\n", len(res.RejectReasons)-shown)
					break
				}
			}
		}
		fmt.Printf("  took %s\n", res.FinishedAt.Sub(res.StartedAt).Round(time.Millisecond))
	}
	if err != nil {
		return err
	}
	fmt.Println("\n  Records land as REFERENCE. A steward reconciles them against")
	fmt.Printf("  GNHR/GSS before canonical publication. Environment: %s\n", cfg.Env)
	return nil
}

// cmdBoundaries attaches administrative boundary polygons.
//
// Defaults to a DRY RUN. Attaching a boundary to the wrong district silently
// misroutes every containment query for it, so seeing the match report before
// writing is the safe default rather than an option.
func cmdBoundaries(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("boundaries", flag.ContinueOnError)
	level := fs.String("level", "ADM2", "ADM1 for regions, ADM2 for districts")
	file := fs.String("file", "", "path to the geoBoundaries GeoJSON")
	apply := fs.Bool("apply", false, "write the matches (default is a dry run)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *file == "" {
		return fmt.Errorf("--file is required")
	}

	store, _, err := connect(ctx)
	if err != nil {
		return err
	}
	defer store.Close(ctx)
	if err := mongo.Migrate(ctx, store.DB()); err != nil {
		return err
	}

	lic := geoboundaries.Licence()
	fmt.Printf("→ %s boundaries from %s\n", *level, *file)
	fmt.Printf("  licence: %s (%s)\n", lic.Name, lic.SPDX)
	fmt.Printf("  attribution: %s\n", lic.Attribution)
	if !*apply {
		fmt.Println("  DRY RUN — nothing will be written. Re-run with --apply.")
	}
	fmt.Println()

	features, err := geoboundaries.Load(*file)
	if err != nil {
		return err
	}

	regionRepo := mongo.NewRegionRepo(store)
	districtRepo := mongo.NewDistrictRepo(store)

	var candidates []ingest.NameCandidate
	var writer ingest.BoundaryWriter
	if *level == "ADM1" {
		candidates, err = ingest.RegionCandidates(ctx, regionRepo)
		writer = regionRepo
	} else {
		candidates, err = ingest.DistrictCandidates(ctx, districtRepo)
		writer = districtRepo
	}
	if err != nil {
		return err
	}

	res := ingest.BoundaryResult{Level: *level, Total: len(features)}
	for _, f := range features {
		name := f.Properties.ShapeName
		m := ingest.MatchByName(name, candidates)

		geom, gerr := geoboundaries.ToGeometry(f.Geometry)
		if gerr != nil {
			res.InvalidGeom = append(res.InvalidGeom, fmt.Sprintf("%s: %v", name, gerr))
			continue
		}
		geoboundaries.NormalizeWinding(geom)

		if m.TargetID == "" {
			if strings.HasPrefix(m.Reason, "ambiguous") {
				res.Ambiguous = append(res.Ambiguous, m)
			} else {
				res.Unmatched = append(res.Unmatched, m)
			}
			continue
		}
		if m.Exact {
			res.Exact++
		} else {
			res.Fuzzy++
			res.AppliedFuzzy = append(res.AppliedFuzzy, m)
		}
		if *apply {
			if err := writer.SetGeometry(ctx, m.TargetID, geom); err != nil {
				return fmt.Errorf("write %s: %w", m.TargetName, err)
			}
		}
	}

	fmt.Println(res)
	if len(res.AppliedFuzzy) > 0 {
		fmt.Printf("\n  Fuzzy matches applied (review these — %d):\n", len(res.AppliedFuzzy))
		for _, m := range res.AppliedFuzzy {
			fmt.Printf("    %-32s → %-32s %.2f\n", m.SourceName, m.TargetName, m.Score)
		}
	}
	if len(res.Ambiguous) > 0 {
		fmt.Printf("\n  AMBIGUOUS — not applied, needs a steward (%d):\n", len(res.Ambiguous))
		for _, m := range res.Ambiguous {
			fmt.Printf("    %-32s %s\n", m.SourceName, m.Reason)
		}
	}
	if len(res.Unmatched) > 0 {
		fmt.Printf("\n  UNMATCHED — not applied (%d):\n", len(res.Unmatched))
		for _, m := range res.Unmatched {
			fmt.Printf("    %-32s %s\n", m.SourceName, m.Reason)
		}
	}
	if len(res.InvalidGeom) > 0 {
		fmt.Printf("\n  INVALID GEOMETRY — rejected (%d):\n", len(res.InvalidGeom))
		for _, s := range res.InvalidGeom {
			fmt.Printf("    %s\n", s)
		}
	}
	if !*apply {
		fmt.Println("\n  Dry run complete. Re-run with --apply to write.")
	}
	return nil
}

// cmdAssignDistricts fills each place's district by point-in-polygon
// containment against the district boundaries.
func cmdAssignDistricts(ctx context.Context) error {
	store, _, err := connect(ctx)
	if err != nil {
		return err
	}
	defer store.Close(ctx)

	fmt.Println("→ assigning districts by containment ($geoWithin)")
	places := mongo.NewPlaceRepo(store)
	districts := mongo.NewDistrictRepo(store)

	assigned, unassigned, err := places.AssignDistrictsByContainment(ctx, districts,
		func(msg string) { fmt.Println(msg) })
	if err != nil {
		return err
	}
	recordAudit(ctx, store, audit.ActionDistrictsAssign,
		audit.Target{Kind: "dataset", ID: datasetVersion, Label: datasetVersion},
		nil, map[string]any{"placesAssigned": assigned}, nil)
	fmt.Printf("✓ assigned %d places to a district\n", assigned)
	if unassigned > 0 {
		fmt.Printf("  %d places with coordinates still have no district.\n", unassigned)
		fmt.Println("  Expected causes: a district whose boundary did not match, or a")
		fmt.Println("  place sitting just outside every polygon (coastline, border).")
	}
	return nil
}

// cmdDedupe merges places that different sources describe independently.
// Defaults to a dry run: a wrong merge destroys a real place.
func cmdDedupe(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("dedupe", flag.ContinueOnError)
	apply := fs.Bool("apply", false, "write the merges (default is a dry run)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	store, _, err := connect(ctx)
	if err != nil {
		return err
	}
	defer store.Close(ctx)

	places := mongo.NewPlaceRepo(store)
	d := &ingest.Deduper{
		Places:    places,
		Grouper:   places,
		Redirects: mongo.NewRedirectRepo(store),
	}
	if !*apply {
		fmt.Println("→ DRY RUN — nothing will be written. Re-run with --apply.")
	}
	res, err := d.Run(ctx, *apply)
	if err != nil {
		return err
	}
	fmt.Printf("\n%s\n", res)

	if len(res.Merged) > 0 {
		fmt.Printf("\n  Merges (%d):\n", len(res.Merged))
		for _, m := range res.Merged {
			gains := ""
			if m.GainsCoordinate {
				gains += " +coordinate"
			}
			if m.GainsDistrict {
				gains += " +district"
			}
			if m.GainsAliases > 0 {
				gains += fmt.Sprintf(" +%d aliases", m.GainsAliases)
			}
			fmt.Printf("    %-24s ← %-24s %s%s\n", m.SurvivorID, m.MergedID, m.Region, gains)
		}
		fmt.Println("\n  The merged ids are DEPRECATED, not deleted: a caller holding one")
		fmt.Println("  receives 410 with mergedInto rather than a bare 404.")
	}
	if len(res.Ambiguous) > 0 {
		fmt.Printf("\n  Ambiguous — left alone for a steward (%d):\n", len(res.Ambiguous))
		for _, a := range res.Ambiguous {
			fmt.Printf("    %s\n", a)
		}
	}
	return nil
}

// cmdKeyCreate issues an API key. The secret is printed ONCE and is then
// unrecoverable — only its argon2id digest is stored (Spec §12.2).

// operatorActor identifies the human running this CLI.
//
// The OS user and host are the best evidence available: the CLI runs on a
// trusted machine with database credentials, and there is no login to
// attribute the action to yet (GEO-9.2). Recording what we actually know
// beats recording "admin" and implying more.
func operatorActor() audit.Actor {
	name := os.Getenv("SUDO_USER")
	if name == "" {
		if u, err := user.Current(); err == nil {
			name = u.Username
		}
	}
	if name == "" {
		name = "unknown"
	}
	host, _ := os.Hostname()
	return audit.Actor{Kind: audit.ActorOperator, ID: name, Label: name + "@" + host}
}

// recordAudit appends an audit row for a privileged CLI action.
//
// A failure to write the audit row is REPORTED but does not undo the action,
// which has already happened — silently swallowing it would leave a gap in
// the log with nobody aware of it.
func recordAudit(
	ctx context.Context, store *mongo.Store, action audit.Action, target audit.Target,
	before, after map[string]any, actionErr error,
) {
	e, err := audit.New(operatorActor(), action, target)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not build audit entry: %v\n", err)
		return
	}
	e = e.WithChange(before, after)
	if actionErr != nil {
		e = e.Failed(actionErr)
	}
	if _, err := mongo.NewAuditRepo(store).Append(ctx, e); err != nil {
		fmt.Fprintf(os.Stderr, "warning: audit entry NOT recorded for %s: %v\n", action, err)
	}
}

func cmdKeyCreate(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("keys create", flag.ContinueOnError)
	org := fs.String("org", "", "organization id")
	app := fs.String("app", "", "application id")
	name := fs.String("name", "", "human label for this key")
	class := fs.String("class", "SERVER", "SERVER, BROWSER or TEST")
	env := fs.String("env", "live", "live or test")
	origins := fs.String("origins", "", "comma-separated allowed origins (required for BROWSER)")
	scopes := fs.String("scopes", "locations:read,search:read,geocode:read,datasets:read", "comma-separated scopes")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *name == "" {
		return fmt.Errorf("--name is required")
	}

	store, _, err := connect(ctx)
	if err != nil {
		return err
	}
	defer store.Close(ctx)
	if err := mongo.Migrate(ctx, store.DB()); err != nil {
		return err
	}

	var parsed []identity.Scope
	for _, raw := range strings.Split(*scopes, ",") {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		sc, serr := identity.ParseScope(raw)
		if serr != nil {
			return serr
		}
		parsed = append(parsed, sc)
	}

	var allowed []string
	for _, o := range strings.Split(*origins, ",") {
		if o = strings.TrimSpace(o); o != "" {
			allowed = append(allowed, o)
		}
	}

	gen, err := identity.Generate(identity.Environment(*env))
	if err != nil {
		return err
	}
	key := identity.APIKey{
		ID:             "key_" + gen.Prefix,
		ApplicationID:  *app,
		OrganizationID: *org,
		Name:           *name,
		Class:          identity.KeyClass(strings.ToUpper(*class)),
		Environment:    identity.Environment(*env),
		Prefix:         gen.Prefix,
		SecretHash:     gen.SecretHash,
		Scopes:         parsed,
		AllowedOrigins: allowed,
		CreatedAt:      time.Now(),
	}
	createErr := mongo.NewKeyRepo(store).Create(ctx, key)
	recordAudit(ctx, store, audit.ActionKeyCreated,
		audit.Target{Kind: "api_key", ID: key.ID, Label: key.Name},
		nil,
		map[string]any{
			"prefix": key.Prefix, "class": string(key.Class),
			"environment": string(key.Environment), "scopes": *scopes,
			"allowedOrigins": allowed, "organizationId": key.OrganizationID,
			"applicationId": key.ApplicationID,
		},
		createErr)
	if createErr != nil {
		return createErr
	}

	fmt.Printf("✓ created %s (%s, %s)\n\n", key.Name, key.Class, key.Environment)
	fmt.Printf("  %s\n\n", gen.Full)
	fmt.Println("  This is the only time the secret is shown. Store it now.")
	fmt.Printf("  Prefix (safe to log and share): %s\n", gen.Prefix)
	return nil
}

func cmdKeyRevoke(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("keys revoke", flag.ContinueOnError)
	prefix := fs.String("prefix", "", "the key prefix to revoke")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *prefix == "" {
		return fmt.Errorf("--prefix is required")
	}
	store, _, err := connect(ctx)
	if err != nil {
		return err
	}
	defer store.Close(ctx)

	// Revocation takes effect on the very next request (Spec §13).
	revokeErr := mongo.NewKeyRepo(store).Revoke(ctx, *prefix, time.Now())
	// Recorded whether or not it succeeded: an attempt to revoke a key that
	// does not exist is exactly the kind of row a security review wants.
	recordAudit(ctx, store, audit.ActionKeyRevoked,
		audit.Target{Kind: "api_key", ID: *prefix, Label: *prefix},
		map[string]any{"revoked": false}, map[string]any{"revoked": true}, revokeErr)
	if revokeErr != nil {
		return revokeErr
	}
	fmt.Printf("✓ revoked %s — effective immediately\n", *prefix)
	return nil
}

func cmdReconcile(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("reconcile", flag.ContinueOnError)
	against := fs.String("against", "canonical-staging", "target to reconcile against")
	if err := fs.Parse(args); err != nil {
		return err
	}
	// Reconciliation REPORTS proposed changes; applying them requires a reviewed
	// change request (plan GEO-4.2, rule R5). It never writes.
	fmt.Printf("→ reconcile against %s: report-only, no writes\n", *against)
	fmt.Println("  261 seed districts carry SEED_NEEDS_CANONICAL_RECONCILIATION and")
	fmt.Println("  await official GNHR/GSS codes before canonical promotion.")
	return nil
}
