from __future__ import annotations

import asyncio
import hashlib
import os
import tempfile
from collections.abc import AsyncIterator, Awaitable, Callable, Iterator, Mapping
from pathlib import Path
from typing import Any
from urllib.parse import quote

import httpx
from pydantic import TypeAdapter

from ._transport import (
    RetryConfig,
    async_request,
    async_stream_response,
    error_from_response,
    read_error_response,
    read_error_response_async,
    sync_request,
    sync_stream_response,
)
from ._version import api_version, tested_dataset_version
from .errors import GhanaGeoError
from .models import (
    BoundaryFeature,
    DatasetPage,
    District,
    DistrictPage,
    DownloadList,
    Page,
    Place,
    PlacePage,
    Region,
    RegionPage,
    ReverseResult,
    SearchPage,
)

DEFAULT_BASE_URL = "https://api.ghanageo.gov.gh/v1"
DEFAULT_MAX_DOWNLOAD_BYTES = 256 * 1024 * 1024
DEFAULT_MAX_ERROR_BODY_BYTES = 1024 * 1024
Telemetry = Callable[[Mapping[str, Any]], None]


def _headers(api_key: str | None) -> dict[str, str]:
    headers = {
        "Accept": "application/json",
        "Accept-Encoding": "identity",
        "User-Agent": "ghanageo-python/2.0.0a1",
    }
    if api_key:
        headers["Authorization"] = f"Bearer {api_key}"
    return headers


def _retry(value: bool | RetryConfig) -> RetryConfig | None:
    return RetryConfig() if value is True else value if isinstance(value, RetryConfig) else None


def _model[ModelT](response: httpx.Response, model: type[ModelT]) -> ModelT:
    if not response.is_success:
        raise error_from_response(response)
    return TypeAdapter(model).validate_python(response.json())


class GhanaGeo:
    """Synchronous GhanaGeo client. Construction and public reads are anonymous by default."""

    api_version = api_version
    tested_dataset_version = tested_dataset_version

    def __init__(
        self,
        *,
        base_url: str = DEFAULT_BASE_URL,
        api_key: str | None = None,
        retry: bool | RetryConfig = True,
        telemetry: Telemetry | bool | None = None,
        timeout: float | httpx.Timeout = 10.0,
        transport: httpx.BaseTransport | None = None,
        max_download_bytes: int = DEFAULT_MAX_DOWNLOAD_BYTES,
        max_error_body_bytes: int = DEFAULT_MAX_ERROR_BODY_BYTES,
    ) -> None:
        self._retry = _retry(retry)
        self._telemetry = telemetry if callable(telemetry) else None
        self._max_download_bytes = _positive_size(max_download_bytes)
        self._max_error_body_bytes = _positive_size(max_error_body_bytes)
        self._client = httpx.Client(
            base_url=base_url.rstrip("/"),
            headers=_headers(api_key),
            timeout=timeout,
            transport=transport,
        )

    def __enter__(self) -> GhanaGeo:
        return self

    def __exit__(self, *_: object) -> None:
        self.close()

    def close(self) -> None:
        self._client.close()

    def _get[ModelT](self, path: str, model: type[ModelT], params: Mapping[str, Any] | None = None) -> ModelT:
        return _model(
            sync_request(
                self._client,
                path,
                params=params,
                retry=self._retry,
                telemetry=self._telemetry,
                error_body_limit=self._max_error_body_bytes,
            ),
            model,
        )

    def regions(self, *, cursor: str | None = None, limit: int | None = None) -> RegionPage:
        return self._get("/regions", RegionPage, {"cursor": cursor, "limit": limit})

    def region(self, region_id: str) -> Region:
        return self._get(f"/regions/{_segment(region_id)}", Region)

    def region_districts(self, region_id: str, *, cursor: str | None = None, limit: int | None = None) -> DistrictPage:
        return self._get(f"/regions/{_segment(region_id)}/districts", DistrictPage, {"cursor": cursor, "limit": limit})

    def districts(
        self, *, region_id: str | None = None, q: str | None = None, cursor: str | None = None, limit: int | None = None
    ) -> DistrictPage:
        return self._get("/districts", DistrictPage, {"regionId": region_id, "q": q, "cursor": cursor, "limit": limit})

    def district(self, district_id: str) -> District:
        return self._get(f"/districts/{_segment(district_id)}", District)

    def district_places(
        self, district_id: str, *, cursor: str | None = None, limit: int | None = None, type: str | None = None
    ) -> PlacePage:
        return self._get(
            f"/districts/{_segment(district_id)}/places", PlacePage, {"cursor": cursor, "limit": limit, "type": type}
        )

    def places(
        self,
        *,
        region_id: str | None = None,
        district_id: str | None = None,
        type: str | None = None,
        q: str | None = None,
        cursor: str | None = None,
        limit: int | None = None,
    ) -> PlacePage:
        return self._get(
            "/places",
            PlacePage,
            {"regionId": region_id, "districtId": district_id, "type": type, "q": q, "cursor": cursor, "limit": limit},
        )

    def place(self, place_id: str) -> Place:
        return self._get(f"/places/{_segment(place_id)}", Place)

    def search(
        self,
        q: str,
        *,
        region_id: str | None = None,
        district_id: str | None = None,
        type: str | None = None,
        limit: int | None = None,
    ) -> SearchPage:
        return self._get(
            "/search",
            SearchPage,
            {"q": q, "regionId": region_id, "districtId": district_id, "type": type, "limit": limit},
        )

    def autocomplete(self, q: str, *, limit: int = 10) -> SearchPage:
        return self._get("/autocomplete", SearchPage, {"q": q, "limit": limit})

    def geocode(self, q: str, *, limit: int = 10) -> SearchPage:
        return self._get("/geocode", SearchPage, {"q": q, "limit": limit})

    def reverse_geocode(self, latitude: float, longitude: float) -> ReverseResult:
        return self._get("/reverse", ReverseResult, {"lat": latitude, "lng": longitude})

    def nearby(self, latitude: float, longitude: float, *, radius: int = 5000, limit: int = 20) -> PlacePage:
        return self._get("/nearby", PlacePage, {"lat": latitude, "lng": longitude, "radius": radius, "limit": limit})

    def boundary(self, location_id: str) -> BoundaryFeature:
        return self._get(f"/boundaries/{_segment(location_id)}", BoundaryFeature)

    def datasets(self) -> DatasetPage:
        return self._get("/datasets", DatasetPage)

    def dataset_downloads(self, version: str) -> DownloadList:
        return self._get(f"/datasets/{_segment(version)}/downloads", DownloadList)

    def roads(self) -> dict[str, Any]:
        return self._json("/roads")

    def points_of_interest(self) -> dict[str, Any]:
        return self._json("/pois")

    def _json(self, path: str) -> dict[str, Any]:
        response = sync_request(
            self._client,
            path,
            params=None,
            retry=self._retry,
            telemetry=self._telemetry,
            error_body_limit=self._max_error_body_bytes,
        )
        if not response.is_success:
            raise error_from_response(response)
        body = response.json()
        if not isinstance(body, dict):
            raise GhanaGeoError(
                "GhanaGeo returned a non-object response", status=response.status_code, code="INVALID_RESPONSE"
            )
        return body

    def download_dataset_artifact(
        self,
        version: str,
        entity: str,
        format: str,
        *,
        destination: str | Path | None = None,
        checksum: str | None = None,
    ) -> bytes:
        path = f"/datasets/{_segment(version)}/downloads/{_segment(entity)}.{_segment(format)}"
        response = sync_stream_response(self._client, path, retry=self._retry, telemetry=self._telemetry)
        try:
            if not response.is_success:
                response = read_error_response(response, self._max_error_body_bytes)
                raise error_from_response(response)
            _check_content_length(response, self._max_download_bytes)
            content = _bounded_bytes(response.iter_bytes(), self._max_download_bytes)
        finally:
            response.close()
        _verify_checksum(content, checksum)
        if destination is not None:
            _atomic_write(Path(destination), content)
        return content

    def pages[PageT](self, fetch_page: Callable[[str | None], Page[PageT]]) -> Iterator[Page[PageT]]:
        cursor: str | None = None
        seen: set[str] = set()
        while True:
            page = fetch_page(cursor)
            yield page
            if not page.next_cursor:
                return
            if page.next_cursor in seen:
                raise GhanaGeoError("GhanaGeo returned a repeated pagination cursor", status=500, code="INVALID_CURSOR")
            seen.add(page.next_cursor)
            cursor = page.next_cursor

    def region_pages(self, *, limit: int | None = None) -> Iterator[RegionPage]:
        return self.pages(lambda cursor: self.regions(cursor=cursor, limit=limit))

    def district_pages(
        self, *, region_id: str | None = None, q: str | None = None, limit: int | None = None
    ) -> Iterator[DistrictPage]:
        return self.pages(lambda cursor: self.districts(region_id=region_id, q=q, cursor=cursor, limit=limit))

    def place_pages(
        self,
        *,
        region_id: str | None = None,
        district_id: str | None = None,
        type: str | None = None,
        q: str | None = None,
        limit: int | None = None,
    ) -> Iterator[PlacePage]:
        return self.pages(
            lambda cursor: self.places(
                region_id=region_id, district_id=district_id, type=type, q=q, cursor=cursor, limit=limit
            )
        )


class AsyncGhanaGeo:
    """Async GhanaGeo client; task cancellation propagates through httpx and retry sleeps."""

    api_version = api_version
    tested_dataset_version = tested_dataset_version

    def __init__(
        self,
        *,
        base_url: str = DEFAULT_BASE_URL,
        api_key: str | None = None,
        retry: bool | RetryConfig = True,
        telemetry: Telemetry | bool | None = None,
        timeout: float | httpx.Timeout = 10.0,
        transport: httpx.AsyncBaseTransport | None = None,
        max_download_bytes: int = DEFAULT_MAX_DOWNLOAD_BYTES,
        max_error_body_bytes: int = DEFAULT_MAX_ERROR_BODY_BYTES,
    ) -> None:
        self._retry = _retry(retry)
        self._telemetry = telemetry if callable(telemetry) else None
        self._max_download_bytes = _positive_size(max_download_bytes)
        self._max_error_body_bytes = _positive_size(max_error_body_bytes)
        self._client = httpx.AsyncClient(
            base_url=base_url.rstrip("/"), headers=_headers(api_key), timeout=timeout, transport=transport
        )

    async def __aenter__(self) -> AsyncGhanaGeo:
        return self

    async def __aexit__(self, *_: object) -> None:
        await self.aclose()

    async def aclose(self) -> None:
        await self._client.aclose()

    async def _get[ModelT](self, path: str, model: type[ModelT], params: Mapping[str, Any] | None = None) -> ModelT:
        return _model(
            await async_request(
                self._client,
                path,
                params=params,
                retry=self._retry,
                telemetry=self._telemetry,
                error_body_limit=self._max_error_body_bytes,
            ),
            model,
        )

    async def regions(self, *, cursor: str | None = None, limit: int | None = None) -> RegionPage:
        return await self._get("/regions", RegionPage, {"cursor": cursor, "limit": limit})

    async def region(self, region_id: str) -> Region:
        return await self._get(f"/regions/{_segment(region_id)}", Region)

    async def region_districts(
        self, region_id: str, *, cursor: str | None = None, limit: int | None = None
    ) -> DistrictPage:
        return await self._get(
            f"/regions/{_segment(region_id)}/districts", DistrictPage, {"cursor": cursor, "limit": limit}
        )

    async def districts(
        self, *, region_id: str | None = None, q: str | None = None, cursor: str | None = None, limit: int | None = None
    ) -> DistrictPage:
        return await self._get(
            "/districts", DistrictPage, {"regionId": region_id, "q": q, "cursor": cursor, "limit": limit}
        )

    async def district(self, district_id: str) -> District:
        return await self._get(f"/districts/{_segment(district_id)}", District)

    async def district_places(
        self, district_id: str, *, cursor: str | None = None, limit: int | None = None, type: str | None = None
    ) -> PlacePage:
        return await self._get(
            f"/districts/{_segment(district_id)}/places", PlacePage, {"cursor": cursor, "limit": limit, "type": type}
        )

    async def places(
        self,
        *,
        region_id: str | None = None,
        district_id: str | None = None,
        type: str | None = None,
        q: str | None = None,
        cursor: str | None = None,
        limit: int | None = None,
    ) -> PlacePage:
        return await self._get(
            "/places",
            PlacePage,
            {"regionId": region_id, "districtId": district_id, "type": type, "q": q, "cursor": cursor, "limit": limit},
        )

    async def place(self, place_id: str) -> Place:
        return await self._get(f"/places/{_segment(place_id)}", Place)

    async def search(
        self,
        q: str,
        *,
        region_id: str | None = None,
        district_id: str | None = None,
        type: str | None = None,
        limit: int | None = None,
    ) -> SearchPage:
        return await self._get(
            "/search",
            SearchPage,
            {"q": q, "regionId": region_id, "districtId": district_id, "type": type, "limit": limit},
        )

    async def autocomplete(self, q: str, *, limit: int = 10) -> SearchPage:
        return await self._get("/autocomplete", SearchPage, {"q": q, "limit": limit})

    async def geocode(self, q: str, *, limit: int = 10) -> SearchPage:
        return await self._get("/geocode", SearchPage, {"q": q, "limit": limit})

    async def reverse_geocode(self, latitude: float, longitude: float) -> ReverseResult:
        return await self._get("/reverse", ReverseResult, {"lat": latitude, "lng": longitude})

    async def nearby(self, latitude: float, longitude: float, *, radius: int = 5000, limit: int = 20) -> PlacePage:
        return await self._get(
            "/nearby", PlacePage, {"lat": latitude, "lng": longitude, "radius": radius, "limit": limit}
        )

    async def boundary(self, location_id: str) -> BoundaryFeature:
        return await self._get(f"/boundaries/{_segment(location_id)}", BoundaryFeature)

    async def datasets(self) -> DatasetPage:
        return await self._get("/datasets", DatasetPage)

    async def dataset_downloads(self, version: str) -> DownloadList:
        return await self._get(f"/datasets/{_segment(version)}/downloads", DownloadList)

    async def roads(self) -> dict[str, Any]:
        return await self._json("/roads")

    async def points_of_interest(self) -> dict[str, Any]:
        return await self._json("/pois")

    async def _json(self, path: str) -> dict[str, Any]:
        response = await async_request(
            self._client,
            path,
            params=None,
            retry=self._retry,
            telemetry=self._telemetry,
            error_body_limit=self._max_error_body_bytes,
        )
        if not response.is_success:
            raise error_from_response(response)
        body = response.json()
        if not isinstance(body, dict):
            raise GhanaGeoError(
                "GhanaGeo returned a non-object response", status=response.status_code, code="INVALID_RESPONSE"
            )
        return body

    async def download_dataset_artifact(
        self,
        version: str,
        entity: str,
        format: str,
        *,
        destination: str | Path | None = None,
        checksum: str | None = None,
    ) -> bytes:
        path = f"/datasets/{_segment(version)}/downloads/{_segment(entity)}.{_segment(format)}"
        response = await async_stream_response(self._client, path, retry=self._retry, telemetry=self._telemetry)
        chunks: list[bytes] = []
        size = 0
        try:
            if not response.is_success:
                response = await read_error_response_async(response, self._max_error_body_bytes)
                raise error_from_response(response)
            _check_content_length(response, self._max_download_bytes)
            async for chunk in response.aiter_bytes():
                size += len(chunk)
                if size > self._max_download_bytes:
                    raise _download_too_large(self._max_download_bytes)
                chunks.append(chunk)
        finally:
            await response.aclose()
        content = b"".join(chunks)
        _verify_checksum(content, checksum)
        if destination is not None:
            await asyncio.to_thread(_atomic_write, Path(destination), content)
        return content

    async def pages[PageT](
        self, fetch_page: Callable[[str | None], Awaitable[Page[PageT]]]
    ) -> AsyncIterator[Page[PageT]]:
        cursor: str | None = None
        seen: set[str] = set()
        while True:
            page = await fetch_page(cursor)
            yield page
            if not page.next_cursor:
                return
            if page.next_cursor in seen:
                raise GhanaGeoError("GhanaGeo returned a repeated pagination cursor", status=500, code="INVALID_CURSOR")
            seen.add(page.next_cursor)
            cursor = page.next_cursor

    def region_pages(self, *, limit: int | None = None) -> AsyncIterator[RegionPage]:
        return self.pages(lambda cursor: self.regions(cursor=cursor, limit=limit))

    def district_pages(
        self, *, region_id: str | None = None, q: str | None = None, limit: int | None = None
    ) -> AsyncIterator[DistrictPage]:
        return self.pages(lambda cursor: self.districts(region_id=region_id, q=q, cursor=cursor, limit=limit))

    def place_pages(
        self,
        *,
        region_id: str | None = None,
        district_id: str | None = None,
        type: str | None = None,
        q: str | None = None,
        limit: int | None = None,
    ) -> AsyncIterator[PlacePage]:
        return self.pages(
            lambda cursor: self.places(
                region_id=region_id, district_id=district_id, type=type, q=q, cursor=cursor, limit=limit
            )
        )


def _segment(value: str) -> str:
    if not isinstance(value, str) or not value:
        raise ValueError("resource identifiers must be non-empty strings")
    return quote(value, safe="")


def _positive_size(value: int) -> int:
    if not isinstance(value, int) or isinstance(value, bool) or value <= 0:
        raise ValueError("max_download_bytes must be a positive integer")
    return value


def _download_too_large(limit: int) -> GhanaGeoError:
    return GhanaGeoError(
        f"Dataset artifact exceeds the configured {limit}-byte limit",
        code="DOWNLOAD_TOO_LARGE",
        details={"maxBytes": limit},
    )


def _check_content_length(response: httpx.Response, limit: int) -> None:
    value = response.headers.get("content-length")
    if value and value.isdigit() and int(value) > limit:
        raise _download_too_large(limit)


def _bounded_bytes(chunks: Iterator[bytes], limit: int) -> bytes:
    collected: list[bytes] = []
    size = 0
    for chunk in chunks:
        size += len(chunk)
        if size > limit:
            raise _download_too_large(limit)
        collected.append(chunk)
    return b"".join(collected)


def _verify_checksum(content: bytes, expected: str | None) -> None:
    if expected is None:
        return
    normalized = expected.removeprefix("sha256:").lower()
    if len(normalized) != 64 or any(character not in "0123456789abcdef" for character in normalized):
        raise ValueError("checksum must be a SHA-256 hex digest, optionally prefixed with sha256:")
    actual = hashlib.sha256(content).hexdigest()
    if actual != normalized:
        raise GhanaGeoError(
            "Dataset artifact checksum does not match",
            code="CHECKSUM_MISMATCH",
            details={"expected": normalized, "actual": actual},
        )


def _atomic_write(destination: Path, content: bytes) -> None:
    destination.parent.mkdir(parents=True, exist_ok=True)
    temporary: str | None = None
    try:
        with tempfile.NamedTemporaryFile(
            dir=destination.parent, prefix=f".{destination.name}.", delete=False
        ) as handle:
            temporary = handle.name
            handle.write(content)
            handle.flush()
            os.fsync(handle.fileno())
        os.replace(temporary, destination)
    finally:
        if temporary is not None and os.path.exists(temporary):
            os.unlink(temporary)
