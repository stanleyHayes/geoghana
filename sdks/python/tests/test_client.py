from __future__ import annotations

import asyncio
import hashlib
import json
import sys
from pathlib import Path
from typing import Any

import httpx
import pytest

from ghanageo import AsyncGhanaGeo, GhanaGeo, GhanaGeoError, RetryConfig, read_geopandas, read_json, read_pandas

VERSION = "2026.08.3-ulid"
REF = {"id": "gh-region-ashanti", "name": "Ashanti"}
PROVENANCE = {"sourceId": "gss-2024", "sourceUrl": "https://example.test/gss", "retrievedAt": "2026-08-01"}
REGION = {
    "id": "gh-region-ashanti",
    "countryCode": "GH",
    "name": "Ashanti",
    "capital": "Kumasi",
    "code": "06",
    "status": "ACTIVE",
    "verificationStatus": "CANONICAL",
    "centroid": {"latitude": 6.6885, "longitude": -1.6244},
    "provenance": PROVENANCE,
    "datasetVersion": VERSION,
}
DISTRICT = {
    "id": "gh-district-kumasi",
    "name": "Kumasi Metropolitan",
    "type": "Metropolitan",
    "region": REF,
    "status": "ACTIVE",
    "verificationStatus": "CANONICAL",
    "centroid": {"latitude": 6.69, "longitude": -1.62},
    "provenance": PROVENANCE,
    "datasetVersion": VERSION,
}
PLACE = {
    "id": "gh-place-kumasi",
    "name": "Kumasi",
    "normalizedName": "kumasi",
    "type": "CITY",
    "region": REF,
    "district": {"id": "gh-district-kumasi", "name": "Kumasi Metropolitan"},
    "aliases": [{"value": "Kumase", "language": "tw", "isPreferred": False}],
    "centroid": {"latitude": 6.6885, "longitude": -1.6244},
    "population": 3600000,
    "status": "ACTIVE",
    "verificationStatus": "CANONICAL",
    "provenance": PROVENANCE,
    "datasetVersion": VERSION,
}


def response(status: int, body: Any, headers: dict[str, str] | None = None) -> httpx.Response:
    return httpx.Response(status, json=body, headers=headers)


def body_for(path: str) -> Any:
    if path.endswith("/regions"):
        return {"data": [REGION], "datasetVersion": VERSION}
    if "/regions/" in path and path.endswith("/districts"):
        return {"data": [DISTRICT], "datasetVersion": VERSION}
    if "/regions/" in path:
        return REGION
    if path.endswith("/districts"):
        return {"data": [DISTRICT], "datasetVersion": VERSION}
    if "/districts/" in path and path.endswith("/places"):
        return {"data": [PLACE], "datasetVersion": VERSION}
    if "/districts/" in path:
        return DISTRICT
    if path.endswith("/places"):
        return {"data": [PLACE], "datasetVersion": VERSION}
    if "/places/" in path:
        return PLACE
    if path.endswith(("/search", "/autocomplete", "/geocode")):
        return {"data": [{**PLACE, "score": 0.98, "matchReason": "name match"}], "datasetVersion": VERSION}
    if path.endswith("/reverse"):
        return {"region": REF, "district": DISTRICT["region"], "nearby": [PLACE], "datasetVersion": VERSION}
    if path.endswith("/nearby"):
        return {"data": [PLACE], "datasetVersion": VERSION}
    if "/boundaries/" in path:
        return {
            "type": "Feature",
            "geometry": {"type": "Polygon", "coordinates": []},
            "properties": {"id": REF["id"], "datasetVersion": VERSION},
        }
    if path.endswith("/datasets"):
        return {"data": [{"version": VERSION, "status": "published"}]}
    if path.endswith("/downloads"):
        return {
            "version": VERSION,
            "downloads": [
                {"format": "json", "url": "https://example.test/data.json", "checksum": "a" * 64, "sizeBytes": 2}
            ],
        }
    if path.endswith(("/roads", "/pois")):
        return {"data": []}
    raise AssertionError(f"unhandled path: {path}")


def handler(request: httpx.Request) -> httpx.Response:
    assert "authorization" not in request.headers
    return response(200, body_for(request.url.path))


def test_models_match_realistic_openapi_payloads_and_all_sync_paths() -> None:
    with GhanaGeo(transport=httpx.MockTransport(handler), retry=False) as client:
        assert client.regions().data[0].centroid.latitude == 6.6885
        assert client.region(REGION["id"]).provenance.source_id == "gss-2024"
        assert client.region_districts(REGION["id"]).data[0].region.name == "Ashanti"
        assert client.districts(region_id=REGION["id"]).data[0].type == "Metropolitan"
        assert client.district(DISTRICT["id"]).region.id == REGION["id"]
        assert client.district_places(DISTRICT["id"]).data[0].aliases[0].language == "tw"
        assert client.places().data[0].district.name == "Kumasi Metropolitan"
        assert client.place(PLACE["id"]).normalized_name == "kumasi"
        assert client.search("Kumasi").data[0].score == 0.98
        assert client.autocomplete("Kum").data[0].name == "Kumasi"
        assert client.geocode("Kumasi").data[0].match_reason == "name match"
        assert client.reverse_geocode(6.68, -1.62).nearby[0].type == "CITY"
        assert client.nearby(6.68, -1.62).data[0].population == 3600000
        assert client.boundary(REGION["id"]).geometry.type == "Polygon"
        assert client.datasets().data[0].status == "published"
        assert client.dataset_downloads(VERSION).downloads[0].size_bytes == 2
        assert client.roads() == {"data": []}
        assert client.points_of_interest() == {"data": []}


@pytest.mark.asyncio
async def test_async_facade_parity_for_all_model_families() -> None:
    async with AsyncGhanaGeo(transport=httpx.MockTransport(handler), retry=False) as client:
        assert (await client.regions()).data[0].country_code == "GH"
        assert (await client.region(REGION["id"])).code == "06"
        assert (await client.region_districts(REGION["id"])).data[0].region.id == REGION["id"]
        assert (await client.districts()).data[0].name == DISTRICT["name"]
        assert (await client.district(DISTRICT["id"])).centroid.longitude == -1.62
        assert (await client.district_places(DISTRICT["id"])).data[0].aliases[0].value == "Kumase"
        assert (await client.places()).data[0].region.name == "Ashanti"
        assert (await client.place(PLACE["id"])).status == "ACTIVE"
        assert (await client.search("Kumasi")).data[0].score == 0.98
        assert (await client.autocomplete("Kum")).dataset_version == VERSION
        assert (await client.geocode("Kumasi")).data[0].type == "CITY"
        assert (await client.reverse_geocode(6.68, -1.62)).region.name == "Ashanti"
        assert (await client.nearby(6.68, -1.62)).data[0].id == PLACE["id"]
        assert (await client.boundary(REGION["id"])).type == "Feature"
        assert (await client.datasets()).data[0].version == VERSION
        assert (await client.dataset_downloads(VERSION)).downloads[0].format == "json"
        assert await client.roads() == {"data": []}
        assert await client.points_of_interest() == {"data": []}


def test_explicit_key_typed_and_malformed_errors() -> None:
    calls = 0

    def failure(request: httpx.Request) -> httpx.Response:
        nonlocal calls
        assert request.headers["authorization"] == "Bearer caller-key"
        calls += 1
        if calls == 1:
            return response(
                404,
                {
                    "error": {
                        "code": "NOT_FOUND",
                        "message": "missing",
                        "requestId": "req-1",
                        "details": {"id": "x"},
                        "docs": "/docs/errors/NOT_FOUND",
                    }
                },
            )
        return response(500, {"error": "malformed"})

    with GhanaGeo(api_key="caller-key", transport=httpx.MockTransport(failure), retry=False) as client:
        with pytest.raises(GhanaGeoError) as captured:
            client.place("x")
        assert (captured.value.code, captured.value.request_id, captured.value.details) == (
            "NOT_FOUND",
            "req-1",
            {"id": "x"},
        )
        with pytest.raises(GhanaGeoError) as malformed:
            client.regions()
        assert malformed.value.status == 500 and malformed.value.code is None


def test_error_envelopes_are_bounded_before_parsing() -> None:
    oversized = json.dumps({"error": {"message": "x" * 200}}).encode()
    with GhanaGeo(
        transport=httpx.MockTransport(lambda _: httpx.Response(500, content=oversized)),
        retry=False,
        max_error_body_bytes=64,
    ) as client:
        with pytest.raises(GhanaGeoError) as captured:
            client.regions()
    assert captured.value.code == "ERROR_RESPONSE_TOO_LARGE"
    assert captured.value.details == {"maxBytes": 64}
    with GhanaGeo(
        transport=httpx.MockTransport(lambda _: httpx.Response(500, content=oversized)),
        retry=False,
        max_error_body_bytes=64,
    ) as client:
        with pytest.raises(GhanaGeoError) as download_error:
            client.download_dataset_artifact(VERSION, "regions", "json")
    assert download_error.value.code == "ERROR_RESPONSE_TOO_LARGE"


@pytest.mark.asyncio
async def test_async_error_envelopes_are_bounded_before_parsing() -> None:
    oversized = json.dumps({"error": {"message": "x" * 200}}).encode()
    async with AsyncGhanaGeo(
        transport=httpx.MockTransport(lambda _: httpx.Response(400, content=oversized)),
        retry=False,
        max_error_body_bytes=64,
    ) as client:
        with pytest.raises(GhanaGeoError) as captured:
            await client.regions()
    assert captured.value.code == "ERROR_RESPONSE_TOO_LARGE"
    assert captured.value.status == 400


def test_retry_after_is_honored_and_telemetry_has_no_sensitive_fields() -> None:
    calls, delays, events = 0, [], []

    def retrying(_: httpx.Request) -> httpx.Response:
        nonlocal calls
        calls += 1
        return (
            response(429, {}, {"retry-after": "2"})
            if calls == 1
            else response(200, {"data": [], "datasetVersion": VERSION})
        )

    with GhanaGeo(
        transport=httpx.MockTransport(retrying),
        retry=RetryConfig(max_retries=5, sleep=delays.append),
        telemetry=lambda event: events.append(dict(event)),
    ) as client:
        assert client.regions().data == []
    assert calls == 2 and delays == [2.0]
    assert all("authorization" not in item and "body" not in item for item in events)


@pytest.mark.asyncio
async def test_async_transport_retry_and_retry_after() -> None:
    calls, delays = 0, []

    async def no_wait(delay: float) -> None:
        delays.append(delay)

    def retrying(_: httpx.Request) -> httpx.Response:
        nonlocal calls
        calls += 1
        return (
            response(503, {}, {"retry-after": "1"})
            if calls == 1
            else response(200, {"data": [], "datasetVersion": VERSION})
        )

    async with AsyncGhanaGeo(
        transport=httpx.MockTransport(retrying),
        retry=RetryConfig(max_retries=1, async_sleep=no_wait),
    ) as client:
        assert (await client.regions()).data == []
    assert calls == 2 and delays == [1.0]


def test_lazy_pages_detect_duplicate_cursor_and_are_lazy() -> None:
    requested: list[str | None] = []

    def pages(request: httpx.Request) -> httpx.Response:
        requested.append(request.url.params.get("cursor"))
        return response(200, {"data": [REGION], "datasetVersion": VERSION, "nextCursor": "opaque-cursor"})

    with GhanaGeo(transport=httpx.MockTransport(pages), retry=False) as client:
        iterator = client.region_pages()
        next(iterator)
        assert requested == [None]
        next(iterator)
        with pytest.raises(GhanaGeoError, match="repeated"):
            next(iterator)


@pytest.mark.asyncio
async def test_task_cancellation_propagates() -> None:
    async def delayed(_: httpx.Request) -> httpx.Response:
        await asyncio.sleep(60)
        return response(200, {"data": [], "datasetVersion": VERSION})

    async with AsyncGhanaGeo(transport=httpx.MockTransport(delayed), retry=False) as client:
        task = asyncio.create_task(client.regions())
        await asyncio.sleep(0)
        task.cancel()
        with pytest.raises(asyncio.CancelledError):
            await task


def test_bounded_atomic_checksum_download(tmp_path: Path) -> None:
    content = b"fixture-bytes"
    digest = hashlib.sha256(content).hexdigest()
    transport = httpx.MockTransport(lambda _: httpx.Response(200, content=content))
    destination = tmp_path / "nested" / "regions.json"
    with GhanaGeo(transport=transport, retry=False, max_download_bytes=len(content)) as client:
        assert (
            client.download_dataset_artifact(
                VERSION, "regions", "json", destination=destination, checksum=f"sha256:{digest}"
            )
            == content
        )
    assert destination.read_bytes() == content
    with GhanaGeo(transport=transport, retry=False, max_download_bytes=len(content) - 1) as client:
        with pytest.raises(GhanaGeoError) as too_large:
            client.download_dataset_artifact(VERSION, "regions", "json")
    assert too_large.value.code == "DOWNLOAD_TOO_LARGE"
    assert not list(tmp_path.rglob(".*.json.*"))


@pytest.mark.asyncio
async def test_async_download_checksum_mismatch() -> None:
    async with AsyncGhanaGeo(
        transport=httpx.MockTransport(lambda _: httpx.Response(200, content=b"x")), retry=False
    ) as client:
        with pytest.raises(GhanaGeoError) as mismatch:
            await client.download_dataset_artifact(VERSION, "regions", "json", checksum="0" * 64)
    assert mismatch.value.code == "CHECKSUM_MISMATCH"


def test_checksum_failure_does_not_replace_destination(tmp_path: Path) -> None:
    destination = tmp_path / "regions.json"
    destination.write_bytes(b"existing")
    with GhanaGeo(transport=httpx.MockTransport(lambda _: httpx.Response(200, content=b"new")), retry=False) as client:
        with pytest.raises(GhanaGeoError):
            client.download_dataset_artifact(VERSION, "regions", "json", destination=destination, checksum="0" * 64)
    assert destination.read_bytes() == b"existing"


def test_resource_ids_are_encoded_as_single_segments() -> None:
    def encoded(request: httpx.Request) -> httpx.Response:
        assert request.url.raw_path == b"/v1/places/a%2Fb"
        return response(200, {**PLACE, "id": "a/b"})

    with GhanaGeo(transport=httpx.MockTransport(encoded), retry=False) as client:
        assert client.place("a/b").id == "a/b"


def test_missing_and_malformed_parameters_fail_with_wire_evidence() -> None:
    def invalid_coordinates(request: httpx.Request) -> httpx.Response:
        assert dict(request.url.params) == {"lat": "91", "lng": "0"}
        return response(
            400,
            {
                "error": {
                    "code": "INVALID_COORDINATES",
                    "message": "outside valid coordinate range",
                    "requestId": "req-invalid-coordinates",
                    "docs": "/docs/errors/INVALID_COORDINATES",
                    "details": {"latitude": 91, "longitude": 0},
                }
            },
        )

    with GhanaGeo(transport=httpx.MockTransport(invalid_coordinates), retry=False) as client:
        with pytest.raises(GhanaGeoError) as malformed:
            client.reverse_geocode(91, 0)
        assert malformed.value.code == "INVALID_COORDINATES"
        with pytest.raises(ValueError, match="non-empty"):
            client.place("")
        with pytest.raises(TypeError):
            client.search()  # type: ignore[call-arg]


def test_dataset_helpers_and_optional_dependency_message(monkeypatch: pytest.MonkeyPatch, tmp_path: Path) -> None:
    artifact = tmp_path / "data.json"
    artifact.write_text(json.dumps([{"id": 1}]))
    assert read_json(artifact) == [{"id": 1}]
    monkeypatch.setitem(sys.modules, "pandas", None)
    with pytest.raises(ImportError, match=r"ghanageo\[pandas\]"):
        read_pandas(b"id\n1\n")
    monkeypatch.setitem(sys.modules, "geopandas", None)
    with pytest.raises(ImportError, match=r"ghanageo\[geopandas\]"):
        read_geopandas(b'{"type":"FeatureCollection","features":[]}')


def test_versions_are_current() -> None:
    assert GhanaGeo.api_version == "v1"
    assert GhanaGeo.tested_dataset_version == VERSION
