"""Strict live-fixture REST conformance adapter for the public Python facade."""

from __future__ import annotations

import argparse
import asyncio
import hashlib
import json
import shutil
import subprocess
from pathlib import Path
from typing import Any

import httpx

from ghanageo import AsyncGhanaGeo, GhanaGeo, GhanaGeoError, __version__, api_version, tested_dataset_version

ROOT = Path(__file__).resolve().parents[3]


def stable(value: Any) -> str:
    return json.dumps(
        value,
        sort_keys=True,
        separators=(",", ":"),
        ensure_ascii=False,
        default=lambda item: (
            {"bytesSha256": hashlib.sha256(item).hexdigest()} if isinstance(item, bytes) else str(item)
        ),
    )


def digest(value: Any) -> str:
    return hashlib.sha256(stable(value).encode()).hexdigest()


class CaptureTransport(httpx.BaseTransport):
    def __init__(self) -> None:
        self.inner = httpx.HTTPTransport()
        self.request: dict[str, Any] = {}
        self.response: dict[str, Any] = {}
        self.requests: list[dict[str, Any]] = []
        self.responses: list[dict[str, Any]] = []

    def handle_request(self, request: httpx.Request) -> httpx.Response:
        self.request = {"url": str(request.url), "method": request.method, "headers": dict(request.headers)}
        self.requests.append(self.request)
        response = self.inner.handle_request(request)
        self.response = {"status": response.status_code, "headers": dict(response.headers)}
        self.responses.append(self.response)
        return response

    def close(self) -> None:
        self.inner.close()


class AsyncCaptureTransport(httpx.AsyncBaseTransport):
    def __init__(self) -> None:
        self.inner = httpx.AsyncHTTPTransport()
        self.request: dict[str, Any] = {}
        self.response: dict[str, Any] = {}
        self.requests: list[dict[str, Any]] = []
        self.responses: list[dict[str, Any]] = []

    async def handle_async_request(self, request: httpx.Request) -> httpx.Response:
        self.request = {"url": str(request.url), "method": request.method, "headers": dict(request.headers)}
        self.requests.append(self.request)
        response = await self.inner.handle_async_request(request)
        self.response = {"status": response.status_code, "headers": dict(response.headers)}
        self.responses.append(self.response)
        return response

    async def aclose(self) -> None:
        await self.inner.aclose()


def facade_call(client: GhanaGeo, case: dict[str, Any]) -> Any:
    operation = case["operation"]
    input_ = case.get("input", {})
    path, query = input_.get("path", {}), input_.get("query", {})
    calls = {
        "listRegions": lambda: client.regions(cursor=query.get("cursor"), limit=query.get("limit")),
        "getRegion": lambda: client.region(path["id"]),
        "listRegionDistricts": lambda: client.region_districts(
            path["id"], cursor=query.get("cursor"), limit=query.get("limit")
        ),
        "listDistricts": lambda: client.districts(
            region_id=query.get("regionId"),
            q=query.get("q"),
            cursor=query.get("cursor"),
            limit=query.get("limit"),
        ),
        "getDistrict": lambda: client.district(path["id"]),
        "listDistrictPlaces": lambda: client.district_places(
            path["id"], cursor=query.get("cursor"), limit=query.get("limit"), type=query.get("type")
        ),
        "listPlaces": lambda: client.places(
            region_id=query.get("regionId"),
            district_id=query.get("districtId"),
            type=query.get("type"),
            q=query.get("q"),
            cursor=query.get("cursor"),
            limit=query.get("limit"),
        ),
        "getPlace": lambda: client.place(path["id"]),
        "search": lambda: client.search(
            str(query.get("q", "")),
            region_id=query.get("regionId"),
            district_id=query.get("districtId"),
            type=query.get("type"),
            limit=query.get("limit"),
        ),
        "autocomplete": lambda: client.autocomplete(str(query.get("q", "")), limit=query.get("limit", 10)),
        "geocode": lambda: client.geocode(str(query.get("q", "")), limit=query.get("limit", 10)),
        "reverseGeocode": lambda: client.reverse_geocode(query.get("lat", 0), query.get("lng", 0)),
        "nearby": lambda: client.nearby(
            query.get("lat", 0),
            query.get("lng", 0),
            radius=query.get("radius", 5000),
            limit=query.get("limit", 20),
        ),
        "getBoundary": lambda: client.boundary(path["id"]),
        "listDatasets": client.datasets,
        "listDatasetDownloads": lambda: client.dataset_downloads(path["version"]),
        "downloadDatasetArtifact": lambda: client.download_dataset_artifact(
            path["version"], path["entity"], path["format"]
        ),
        "listRoads": client.roads,
        "listPointsOfInterest": client.points_of_interest,
    }
    if operation not in calls:
        raise KeyError(f"Python SDK has no REST mapping for {operation}")
    return calls[operation]()


def normalized(value: Any) -> Any:
    if isinstance(value, bytes):
        return value
    if hasattr(value, "model_dump"):
        return value.model_dump(by_alias=True, exclude_none=True)
    return value


def nested(value: Any, path: str) -> Any:
    current = value
    for segment in path.split("."):
        if not isinstance(current, dict) or segment not in current:
            return None
        current = current[segment]
    return current


def expected_type(path: str) -> type[Any]:
    leaf = path.split(".")[-1]
    if leaf in {"data", "aliases", "downloads", "nearby"}:
        return list
    if leaf in {"error", "provenance", "region", "geometry", "properties", "details"}:
        return dict
    if leaf in {
        "latitude",
        "longitude",
        "maxRadiusMeters",
        "minLength",
        "retryAfterSeconds",
        "limit",
        "cost",
        "maxCost",
        "depth",
        "maxDepth",
    }:
        return (int, float)  # type: ignore[return-value]
    return str


def validate_shape(subject: Any, shape: dict[str, Any]) -> list[str]:
    failures: list[str] = []
    if shape["type"] == "binary" and not isinstance(subject, bytes):
        failures.append("result is not binary")
    if shape["type"] == "object" and not isinstance(subject, dict):
        failures.append("result is not an object")
    for path in shape.get("required", []):
        value = nested(subject, path)
        if value is None:
            failures.append(f"missing result field {path}")
        elif not isinstance(value, expected_type(path)):
            failures.append(f"result field {path} has wrong type")
    for path, kind in shape.get("fieldTypes", {}).items():
        mapping: dict[str, type[Any] | tuple[type[Any], ...]] = {
            "object": dict,
            "array": list,
            "string": str,
            "number": (int, float),
            "boolean": bool,
        }
        if not isinstance(nested(subject, path), mapping[kind]):
            failures.append(f"result field {path} is not {kind}")
    return failures


async def cancellation_evidence(base_url: str, case: dict[str, Any]) -> tuple[dict[str, Any], dict[str, Any], bool]:
    transport = AsyncCaptureTransport()
    client = AsyncGhanaGeo(base_url=base_url, retry=False, transport=transport)
    client._client.headers["x-conformance-case"] = case["id"]  # noqa: SLF001 -- runner-only fixture header
    task = asyncio.create_task(client.search("Kumasi"))
    await asyncio.sleep(case["input"].get("cancelAfterMilliseconds", 10) / 1000)
    task.cancel()
    cancelled = False
    try:
        await task
    except asyncio.CancelledError:
        cancelled = True
    finally:
        await client.aclose()
    return {"calls": transport.requests}, {"calls": transport.responses or [{"aborted": True}]}, cancelled


def execute(
    base_url: str, case: dict[str, Any]
) -> tuple[Any, int | None, dict[str, Any], dict[str, Any], bool, dict[str, Any]]:
    if case["id"] == "semantic.cancellation-propagates":
        request, response, cancelled = asyncio.run(cancellation_evidence(base_url, case))
        return {}, None, request, response, cancelled, {"class": "normal", "units": 2}
    transport = CaptureTransport()
    auth = case.get("input", {}).get("auth")
    client = GhanaGeo(
        base_url=base_url, api_key=auth if auth and auth != "omitted" else None, retry=False, transport=transport
    )
    client._client.headers["x-conformance-case"] = case["id"]  # noqa: SLF001 -- runner-only fixture header
    try:
        value = normalized(facade_call(client, case))
    except GhanaGeoError as error:
        value = {
            "error": {
                "code": error.code,
                "message": str(error),
                "requestId": error.request_id,
                "docs": error.docs,
                "details": error.details,
            }
        }
    except Exception as error:  # noqa: BLE001 -- facade failures belong in the report
        value = {"__runnerFailure": f"{type(error).__name__}: {error}"}
    finally:
        client.close()
    quota = json.loads(transport.response.get("headers", {}).get("x-conformance-quota", "{}"))
    return value, transport.response.get("status"), transport.request, transport.response, False, quota


def run(base_url: str) -> dict[str, Any]:
    ruby = shutil.which("ruby")
    if ruby is None:
        raise RuntimeError("Ruby is required to export the shared conformance contract")
    exported = json.loads(subprocess.check_output([ruby, "tools/conformance/export_cases.rb"], cwd=ROOT, text=True))  # noqa: S603
    cases = [case for case in exported["cases"] if "rest" in case["protocols"]]
    results: list[dict[str, Any]] = []
    evidence: list[dict[str, Any]] = []
    observed_versions: set[str] = set()
    for case in cases:
        failures: list[str] = []
        value: Any
        status: int | None
        request: dict[str, Any]
        response: dict[str, Any]
        cancelled: bool
        quota: dict[str, Any]
        if case["id"] == "semantic.cursor-pagination":
            transport = CaptureTransport()
            client = GhanaGeo(base_url=base_url, retry=False, transport=transport)
            client._client.headers["x-conformance-case"] = case["id"]  # noqa: SLF001
            pages = client.place_pages(limit=2)
            first, second = next(pages), next(pages)
            replay = client.places(limit=2)
            client.close()
            first_ids, second_ids = {item.id for item in first.data}, {item.id for item in second.data}
            if first_ids & second_ids:
                failures.append("pagination returned duplicate identities")
            if not first.next_cursor or first.next_cursor.isdigit() or len(first.next_cursor) < 8:
                failures.append("cursor is not opaque")
            if replay.next_cursor != first.next_cursor:
                failures.append("cursor is not stable on replay")
            value, status = normalized(first), 200
            request, response = {"calls": transport.requests}, {"calls": transport.responses}
            cancelled, quota = False, {"class": "cheap", "units": 1}
        else:
            value, status, request, response, cancelled, quota = execute(base_url, case)
        expected = case["expect"]
        outcome = "error" if isinstance(value, dict) and "error" in value else "success"
        if isinstance(value, dict) and "__runnerFailure" in value:
            failures.append(str(value["__runnerFailure"]))
        if outcome != expected["outcome"]:
            failures.append(f"expected {expected['outcome']}, received {outcome}")
        if expected.get("httpStatus") is not None and status != expected["httpStatus"]:
            failures.append(f"expected HTTP {expected['httpStatus']}, received {status}")
        if quota != expected["quotaCost"]:
            failures.append(f"quota differs: {quota!r}")
        failures.extend(validate_shape(value, expected["shape"]))
        if "error" in expected:
            error = value.get("error", {}) if isinstance(value, dict) else {}
            if error.get("code") != expected["error"]["code"]:
                failures.append("canonical error code differs")
            for field in expected["error"]["requiredFields"]:
                if nested(error, field) is None:
                    failures.append(f"missing error field {field}")
                elif not isinstance(nested(error, field), expected_type(field)):
                    failures.append(f"error field {field} has wrong type")
        semantics = expected.get("semantics", [])
        if "datasetVersionPresent" in semantics:
            version = value.get("datasetVersion") if isinstance(value, dict) else None
            if not isinstance(version, str) or not version:
                failures.append("datasetVersion is absent")
            else:
                observed_versions.add(version)
        if "preservesGhanaianOrthography" in semantics and "Mampɔŋ" not in stable(value):
            failures.append("orthography was not preserved")
        if "anonymousByDefault" in semantics and "authorization" in request.get("headers", {}):
            failures.append("anonymous request sent authorization")
        if "cancellationPropagates" in semantics and not cancelled:
            failures.append("async task cancellation did not propagate")
        if "emptyOutsideGhana" in semantics and (
            value.get("region") is not None or value.get("district") is not None or value.get("nearby") != []
        ):
            failures.append("outside-Ghana result was not empty")
        results.append(
            {
                "caseId": case["id"],
                "status": "failed" if failures else "passed",
                "protocols": ["rest"],
                **({"message": "; ".join(failures)} if failures else {}),
            }
        )
        evidence.append(
            {
                "caseId": case["id"],
                "protocol": "rest",
                "requestDigest": digest(request),
                "responseDigest": digest(response),
            }
        )
    failed = sum(result["status"] == "failed" for result in results)
    if observed_versions != {tested_dataset_version}:
        message = (
            f"fixture dataset versions {sorted(observed_versions)!r} do not match "
            f"tested_dataset_version {tested_dataset_version!r}"
        )
        results[0]["status"] = "failed"
        results[0]["message"] = f"{results[0].get('message', '')}; {message}".lstrip("; ")
        failed = sum(result["status"] == "failed" for result in results)
    report = {
        "schemaVersion": 2,
        "contract": {"digest": exported["contractDigest"], "runnerVersion": "1.0.0"},
        "sdk": {"language": "python", "name": "ghanageo", "version": __version__, "supportedProtocols": ["rest"]},
        "apiVersion": api_version,
        "datasetVersion": next(iter(observed_versions), "unobserved"),
        "summary": {"passed": len(results) - failed, "failed": failed, "skipped": 0},
        "evidenceDigest": digest(evidence),
        "evidence": evidence,
        "results": results,
    }
    return report


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--base-url", required=True)
    parser.add_argument("--output", type=Path, default=Path("python-conformance-report.json"))
    args = parser.parse_args()
    report = run(args.base_url)
    args.output.write_text(json.dumps(report, indent=2) + "\n")
    if report["summary"]["failed"]:
        raise SystemExit(1)
