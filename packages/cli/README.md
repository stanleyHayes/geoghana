# `ghanageo` — Ghana's location data, from your terminal

GhanaGeo is **free**. No account, no API key, no rate-limit upgrade to buy.

```bash
ghanageo search "tema"
ghanageo districts --region gh-region-ashanti
ghanageo regions --json | jq -r '.[].capital'
```

## Install

```bash
# Go
go install github.com/ghanageo/ghanageo-cli@latest

# npm — no global install needed
npx ghanageo search "kumasi"

# Homebrew
brew install ghanageo/tap/ghanageo
```

Or download a binary for your platform from the releases page and put it on
your `PATH`.

## Commands

| Command | What it does |
|---|---|
| `ghanageo regions` | Ghana's 16 regions |
| `ghanageo districts` | Districts (MMDAs), filterable by region |
| `ghanageo places` | Localities |
| `ghanageo search <query>` | Typo-tolerant search across all geography |
| `ghanageo suggest <prefix>` | Fast typeahead suggestions |
| `ghanageo nearby <lat> <lng>` | Places near a coordinate |
| `ghanageo reverse <lat> <lng>` | What geography contains a coordinate |

## Output

Default output is an aligned table for humans. For scripts:

```bash
ghanageo regions --json | jq -r '.[] | "\(.name): \(.capital)"'
ghanageo districts --csv > districts.csv
```

Colour is suppressed automatically when output is piped, and `NO_COLOR` is
honoured.

## Search understands Ghanaian input

Typos, abbreviations and Ghanaian orthography all resolve:

```bash
ghanageo search "kumsai"        # → Kumasi
ghanageo search "tema comm 25"  # comm expands to community
ghanageo search "kwabɛnya"      # matches "kwabenya" and vice versa
```

Every result carries a confidence score and an explanation of *why* it
matched, so an ambiguous query gives you candidates rather than a guess.

## Configuration

| Variable | Purpose |
|---|---|
| `GHANAGEO_BASE_URL` | Point at a self-hosted instance |
| `GHANAGEO_API_KEY` | Optional. Identifies heavy use for fair-use accounting; it does not unlock anything |
| `NO_COLOR` | Disable colour |

## Exit codes

| Code | Meaning |
|---|---|
| `0` | Success |
| `1` | Local error — bad arguments, network unreachable |
| `2` | API error — the message includes the stable error code and a docs link |

## Supporting GhanaGeo

GhanaGeo is free public infrastructure and intends to stay that way. If it is
useful to you, consider supporting it: <https://geo.digitalghana.dev/support>

Donating does not change your rate limits. Everyone gets the same allowance.
