# GhanaGeo Python SDK

Typed synchronous and asynchronous access to GhanaGeo's public `v1` API. Public
reads are anonymous by default; an API key is only sent when explicitly supplied.

```python
from ghanageo import GhanaGeo

with GhanaGeo() as ghana:
    for page in ghana.region_pages(limit=20):
        for region in page.data:
            print(region.name)
```

```python
import asyncio
from ghanageo import AsyncGhanaGeo


async def main() -> None:
    async with AsyncGhanaGeo(telemetry=False) as ghana:
        result = await ghana.reverse_geocode(6.6885, -1.6244)
        print(result.dataset_version)


asyncio.run(main())
```

`GhanaGeoError` preserves `status`, canonical `code`, `request_id`, and structured
`details`. Safe GETs retry twice by default (maximum five), honor bounded
`Retry-After`, and never include authorization or bodies in optional telemetry.
Async task cancellation interrupts both in-flight requests and retry sleeps.

Dataset helpers are optional: install `ghanageo[pandas]` or
`ghanageo[geopandas]`, download an artifact with the client, then call
`read_pandas(...)` or `read_geopandas(...)`.

Artifact downloads stream through a configurable safety bound (256 MiB by
default), can verify a SHA-256 checksum, and replace destination files
atomically only after the full download validates:

```python
client = GhanaGeo(max_download_bytes=32 * 1024 * 1024)
client.download_dataset_artifact(
    "2026.08.3-ulid",
    "places",
    "json",
    destination="places.json",
    checksum="sha256:...",
)
```

## Development

```sh
python -m pip install -e '.[dev]'
ruff check .
mypy src conformance/runner.py
pytest
python -m build
```

The package supports the three most recent CPython releases: 3.12, 3.13, and
3.14. The thin runner in `conformance/runner.py` targets a live fixture server
and writes a report compatible with `contracts/conformance/report-schema.json`.
`contract.lock.json` pins the exact OpenAPI, protobuf, conformance, compatibility,
generator, and verification-tool inputs used by this release. Verify drift with
`python scripts/check_contract_lock.py`, or intentionally refresh it with
`python scripts/check_contract_lock.py --write` after an approved contract update.
