from __future__ import annotations

import asyncio
import email.utils
import random
import time
from collections.abc import Awaitable, Callable, Iterable, Mapping
from datetime import UTC, datetime
from typing import Any

import httpx

from .errors import GhanaGeoError

RETRYABLE_STATUS = frozenset({429, 502, 503, 504})


class RetryConfig:
    def __init__(
        self,
        max_retries: int = 2,
        base_delay: float = 0.1,
        max_delay: float = 2.0,
        *,
        sleep: Callable[[float], None] = time.sleep,
        async_sleep: Callable[[float], Awaitable[None]] = asyncio.sleep,
    ) -> None:
        if max_retries < 0 or base_delay < 0 or max_delay < 0:
            raise ValueError("retry values must be non-negative")
        self.max_retries: int = min(max_retries, 5)
        self.max_delay: float = min(max_delay, 60.0)
        self.base_delay: float = min(base_delay, self.max_delay)
        self.sleep = sleep
        self.async_sleep = async_sleep

    def delay(self, attempt: int, retry_after: str | None) -> float:
        parsed = _retry_after_seconds(retry_after)
        if parsed is not None:
            return min(parsed, self.max_delay)
        jitter = float(random.uniform(0.8, 1.2))  # noqa: S311 -- retry jitter is not cryptographic
        return float(min(self.base_delay * (2**attempt) * jitter, self.max_delay))


def _retry_after_seconds(value: str | None) -> float | None:
    if not value:
        return None
    try:
        return max(0.0, float(value))
    except ValueError:
        try:
            date = email.utils.parsedate_to_datetime(value)
            if date.tzinfo is None:
                date = date.replace(tzinfo=UTC)
            return max(0.0, (date - datetime.now(UTC)).total_seconds())
        except (TypeError, ValueError, OverflowError):
            return None


def error_from_response(response: httpx.Response) -> GhanaGeoError:
    try:
        body = response.json()
    except ValueError:
        body = {}
    candidate = body.get("error", {}) if isinstance(body, dict) else {}
    failure = candidate if isinstance(candidate, dict) else {}
    details = failure.get("details") if isinstance(failure.get("details"), dict) else None
    return GhanaGeoError(
        failure.get("message", f"GhanaGeo request failed ({response.status_code})"),
        status=response.status_code,
        code=failure.get("code"),
        request_id=failure.get("requestId") or response.headers.get("x-request-id"),
        details=details,
        docs=failure.get("docs") if isinstance(failure.get("docs"), str) else None,
    )


def emit(
    observer: Callable[[Mapping[str, Any]], None] | None,
    *,
    path: str,
    attempt: int,
    started: float,
    status: int | None = None,
) -> None:
    if observer is None:
        return
    try:
        observer(
            {
                "path": path,
                "method": "GET",
                "attempt": attempt,
                "status": status,
                "duration_ms": (time.monotonic() - started) * 1000,
            }
        )
    except Exception:  # noqa: BLE001 -- telemetry must never affect transport behavior
        pass


def sync_request(
    client: httpx.Client,
    path: str,
    *,
    params: Mapping[str, Any] | None,
    retry: RetryConfig | None,
    telemetry: Callable[[Mapping[str, Any]], None] | None,
    error_body_limit: int,
    sleep: Callable[[float], None] = time.sleep,
) -> httpx.Response:
    attempts = 1 + (retry.max_retries if retry else 0)
    for attempt in range(attempts):
        started = time.monotonic()
        try:
            request = client.build_request("GET", path, params=_clean(params))
            response = client.send(request, stream=True)
            emit(telemetry, path=path, attempt=attempt, started=started, status=response.status_code)
            if not retry or response.status_code not in RETRYABLE_STATUS or attempt == attempts - 1:
                if response.is_success:
                    response.read()
                    return response
                return read_error_response(response, error_body_limit)
            response.close()
            (retry.sleep if retry else sleep)(retry.delay(attempt, response.headers.get("retry-after")))
        except httpx.TransportError:
            emit(telemetry, path=path, attempt=attempt, started=started)
            if not retry or attempt == attempts - 1:
                raise
            (retry.sleep if retry else sleep)(retry.delay(attempt, None))
    raise RuntimeError("retry loop exhausted")


async def async_request(
    client: httpx.AsyncClient,
    path: str,
    *,
    params: Mapping[str, Any] | None,
    retry: RetryConfig | None,
    telemetry: Callable[[Mapping[str, Any]], None] | None,
    error_body_limit: int,
) -> httpx.Response:
    attempts = 1 + (retry.max_retries if retry else 0)
    for attempt in range(attempts):
        started = time.monotonic()
        try:
            request = client.build_request("GET", path, params=_clean(params))
            response = await client.send(request, stream=True)
            emit(telemetry, path=path, attempt=attempt, started=started, status=response.status_code)
            if not retry or response.status_code not in RETRYABLE_STATUS or attempt == attempts - 1:
                if response.is_success:
                    await response.aread()
                    return response
                return await read_error_response_async(response, error_body_limit)
            await response.aclose()
            await retry.async_sleep(retry.delay(attempt, response.headers.get("retry-after")))
        except asyncio.CancelledError:
            raise
        except httpx.TransportError:
            emit(telemetry, path=path, attempt=attempt, started=started)
            if not retry or attempt == attempts - 1:
                raise
            await retry.async_sleep(retry.delay(attempt, None))
    raise RuntimeError("retry loop exhausted")


def sync_stream_response(
    client: httpx.Client,
    path: str,
    *,
    retry: RetryConfig | None,
    telemetry: Callable[[Mapping[str, Any]], None] | None,
    sleep: Callable[[float], None] = time.sleep,
) -> httpx.Response:
    """Return an unread streaming response; the caller must close it."""
    attempts = 1 + (retry.max_retries if retry else 0)
    for attempt in range(attempts):
        started = time.monotonic()
        try:
            response = client.send(client.build_request("GET", path), stream=True)
            emit(telemetry, path=path, attempt=attempt, started=started, status=response.status_code)
            if not retry or response.status_code not in RETRYABLE_STATUS or attempt == attempts - 1:
                return response
            retry_after = response.headers.get("retry-after")
            response.close()
            (retry.sleep if retry else sleep)(retry.delay(attempt, retry_after))
        except httpx.TransportError:
            emit(telemetry, path=path, attempt=attempt, started=started)
            if not retry or attempt == attempts - 1:
                raise
            (retry.sleep if retry else sleep)(retry.delay(attempt, None))
    raise RuntimeError("retry loop exhausted")


async def async_stream_response(
    client: httpx.AsyncClient,
    path: str,
    *,
    retry: RetryConfig | None,
    telemetry: Callable[[Mapping[str, Any]], None] | None,
) -> httpx.Response:
    """Return an unread async streaming response; the caller must close it."""
    attempts = 1 + (retry.max_retries if retry else 0)
    for attempt in range(attempts):
        started = time.monotonic()
        try:
            response = await client.send(client.build_request("GET", path), stream=True)
            emit(telemetry, path=path, attempt=attempt, started=started, status=response.status_code)
            if not retry or response.status_code not in RETRYABLE_STATUS or attempt == attempts - 1:
                return response
            retry_after = response.headers.get("retry-after")
            await response.aclose()
            await retry.async_sleep(retry.delay(attempt, retry_after))
        except asyncio.CancelledError:
            raise
        except httpx.TransportError:
            emit(telemetry, path=path, attempt=attempt, started=started)
            if not retry or attempt == attempts - 1:
                raise
            await retry.async_sleep(retry.delay(attempt, None))
    raise RuntimeError("retry loop exhausted")


def read_error_response(response: httpx.Response, limit: int) -> httpx.Response:
    if response.is_stream_consumed:
        if len(response.content) > limit:
            raise _error_too_large(response.status_code, limit)
        return response
    try:
        content = _bounded_error_chunks(response.iter_raw(), limit, response.status_code)
        request = response.request
    finally:
        response.close()
    return httpx.Response(response.status_code, headers=response.headers, content=content, request=request)


async def read_error_response_async(response: httpx.Response, limit: int) -> httpx.Response:
    if response.is_stream_consumed:
        if len(response.content) > limit:
            raise _error_too_large(response.status_code, limit)
        return response
    chunks: list[bytes] = []
    size = 0
    try:
        async for chunk in response.aiter_raw():
            size += len(chunk)
            if size > limit:
                raise _error_too_large(response.status_code, limit)
            chunks.append(chunk)
    finally:
        await response.aclose()
    return httpx.Response(
        response.status_code, headers=response.headers, content=b"".join(chunks), request=response.request
    )


def _bounded_error_chunks(chunks: Iterable[bytes], limit: int, status: int | None) -> bytes:
    collected: list[bytes] = []
    size = 0
    for chunk in chunks:
        size += len(chunk)
        if size > limit:
            raise _error_too_large(status, limit)
        collected.append(chunk)
    return b"".join(collected)


def _error_too_large(status: int | None, limit: int) -> GhanaGeoError:
    return GhanaGeoError(
        f"GhanaGeo error response exceeds the configured {limit}-byte limit",
        status=status,
        code="ERROR_RESPONSE_TOO_LARGE",
        details={"maxBytes": limit},
    )


def _clean(params: Mapping[str, Any] | None) -> dict[str, Any]:
    return {key: value for key, value in (params or {}).items() if value is not None and value != ""}
