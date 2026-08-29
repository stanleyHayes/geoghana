package seed

import (
	"context"
	"fmt"
	"sort"

	"github.com/ghanageo/ghanageo/services/api/internal/domain/geography"
	"github.com/ghanageo/ghanageo/services/api/internal/ports"
)

// OfficialDistrictsPerRegion is Ghana's published MMDA count per region.
//
// Source: Wikipedia "Districts of Ghana", which records 261 MMDAs following the
// inauguration of Guan District on 8 October 2021, with this per-region split.
// Verified against our seed on 2026-08-29: all sixteen match exactly.
//
// This lives in code rather than a comment so the numbers are ASSERTED on every
// run. A silent drift in district count is the kind of error that only surfaces
// when somebody's application returns the wrong answer.
var OfficialDistrictsPerRegion = map[string]int{
	"Ahafo": 6, "Ashanti": 43, "Bono": 12, "Bono East": 11,
	"Central": 22, "Eastern": 33, "Greater Accra": 29, "North East": 6,
	"Northern": 16, "Oti": 9, "Savannah": 7, "Upper East": 15,
	"Upper West": 11, "Volta": 18, "Western": 14, "Western North": 9,
}

// Check is one acceptance assertion and its outcome.
type Check struct {
	Name   string
	Passed bool
	Detail string
	Fatal  bool // a failed fatal check blocks publication
}

// Report is the outcome of a full validation run.
type Report struct{ Checks []Check }

func (r *Report) add(name string, passed, fatal bool, format string, args ...any) {
	r.Checks = append(r.Checks, Check{
		Name: name, Passed: passed, Fatal: fatal, Detail: fmt.Sprintf(format, args...),
	})
}

func (r Report) Failed() []Check {
	var out []Check
	for _, c := range r.Checks {
		if !c.Passed {
			out = append(out, c)
		}
	}
	return out
}

// BlocksPublication reports whether any fatal check failed.
func (r Report) BlocksPublication() bool {
	for _, c := range r.Checks {
		if !c.Passed && c.Fatal {
			return true
		}
	}
	return false
}

// Validator runs the Spec §22.1 domain acceptance suite.
type Validator struct {
	Regions   ports.RegionRepository
	Districts ports.DistrictRepository
	Places    ports.PlaceRepository
}

// Run executes every check. It never stops at the first failure: a partial
// report is far less useful than knowing everything that is wrong.
func (v *Validator) Run(ctx context.Context) (*Report, error) {
	r := &Report{}

	regions, err := v.Regions.List(ctx, ports.ListParams{Limit: 100})
	if err != nil {
		return r, err
	}
	r.add("region count", len(regions.Data) == ExpectedRegions, true,
		"found %d, expected %d", len(regions.Data), ExpectedRegions)

	// Every region named in the published figures must exist, and vice versa.
	seen := map[string]bool{}
	for _, reg := range regions.Data {
		seen[reg.Name] = true
	}
	var missing, unexpected []string
	for name := range OfficialDistrictsPerRegion {
		if !seen[name] {
			missing = append(missing, name)
		}
	}
	for name := range seen {
		if _, ok := OfficialDistrictsPerRegion[name]; !ok {
			unexpected = append(unexpected, name)
		}
	}
	sort.Strings(missing)
	sort.Strings(unexpected)
	r.add("region names match the published list", len(missing) == 0 && len(unexpected) == 0, true,
		"missing %v, unexpected %v", missing, unexpected)

	// District counts, in total and per region.
	perRegion := map[string]int{}
	total := 0
	orphans := 0
	cursor := ""
	for {
		page, derr := v.Districts.List(ctx, ports.DistrictFilter{
			ListParams: ports.ListParams{Limit: 100, Cursor: cursor},
		})
		if derr != nil {
			return r, derr
		}
		for _, d := range page.Data {
			total++
			perRegion[d.RegionName]++
			if !seen[d.RegionName] {
				orphans++
			}
		}
		if page.NextCursor == "" {
			break
		}
		cursor = page.NextCursor
	}

	r.add("district count", total == ExpectedDistricts, true,
		"found %d, expected %d", total, ExpectedDistricts)

	var wrong []string
	for name, want := range OfficialDistrictsPerRegion {
		if got := perRegion[name]; got != want {
			wrong = append(wrong, fmt.Sprintf("%s has %d, published figure is %d", name, got, want))
		}
	}
	sort.Strings(wrong)
	r.add("districts per region match published figures", len(wrong) == 0, true,
		"%d regions disagree: %v", len(wrong), wrong)

	// Spec §22.1: every active district must reference an active region.
	r.add("no orphan districts", orphans == 0, true,
		"%d districts reference a region that does not exist", orphans)

	// Coordinates must be plausible for Ghana.
	badCoords := 0
	checked := 0
	cursor = ""
	for {
		page, perr := v.Places.List(ctx, ports.PlaceFilter{
			ListParams: ports.ListParams{Limit: 100, Cursor: cursor},
		})
		if perr != nil {
			return r, perr
		}
		for _, p := range page.Data {
			if p.Centroid == nil {
				continue
			}
			checked++
			if err := p.Centroid.Validate(); err != nil {
				badCoords++
			}
		}
		if page.NextCursor == "" {
			break
		}
		cursor = page.NextCursor
	}
	r.add("coordinates fall within Ghana", badCoords == 0, true,
		"%d of %d places have an implausible coordinate", badCoords, checked)

	return r, nil
}

// ValidateGeometry is a standalone check used by the ingestion gate.
func ValidateGeometry(g *geography.Geometry) error { return g.Validate() }
