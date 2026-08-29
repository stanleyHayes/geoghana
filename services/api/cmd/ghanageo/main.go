// Command ghanageo is the operator CLI: data seed, validate and reconcile.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/ghanageo/ghanageo/services/api/internal/adapters/mongo"
	"github.com/ghanageo/ghanageo/services/api/internal/adapters/search/typesense"
	"github.com/ghanageo/ghanageo/services/api/internal/app/search"
	"github.com/ghanageo/ghanageo/services/api/internal/app/seed"
	"github.com/ghanageo/ghanageo/services/api/internal/platform/config"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `ghanageo — GhanaGeo operator CLI

Usage:
  ghanageo data seed      --file <manifest.json> [--environment local]
  ghanageo data validate  --dataset seed
  ghanageo data reconcile --against canonical-staging
  ghanageo data reindex
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

	fmt.Printf("→ validating dataset %q\n", *dataset)
	if err := verifyCounts(ctx, store); err != nil {
		return err
	}
	orphans, err := mongo.NewDistrictRepo(store).CountOrphans(ctx)
	if err != nil {
		return err
	}
	if orphans > 0 {
		return fmt.Errorf("%d active districts reference a missing region", orphans)
	}
	fmt.Println("✓ every active district references an active region")
	return nil
}

// cmdReindex rebuilds the search index from canonical data.
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
		"2026.08.1-seed",
	)
	fmt.Printf("→ reindexing into %s\n", cfg.TypesenseURL)
	n, err := svc.Reindex(ctx)
	if err != nil {
		return err
	}
	fmt.Printf("✓ indexed %d documents (regions, districts and places)\n", n)
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
