from __future__ import annotations

from typing import Any


class GhanaGeoError(Exception):
    """A normalized GhanaGeo API or client failure."""

    def __init__(
        self,
        message: str,
        *,
        status: int | None = None,
        code: str | None = None,
        request_id: str | None = None,
        details: dict[str, Any] | None = None,
        docs: str | None = None,
    ) -> None:
        super().__init__(message)
        self.status = status
        self.code = code
        self.request_id = request_id
        self.details = details
        self.docs = docs
