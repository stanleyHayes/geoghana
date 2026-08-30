from __future__ import annotations

import io
import json
from pathlib import Path
from typing import Any


def read_pandas(source: bytes | str | Path, *, format: str = "csv") -> Any:
    """Load an artifact into pandas. Install ``ghanageo[pandas]`` first."""
    try:
        import pandas as pd  # type: ignore[import-untyped]
    except ImportError as error:
        raise ImportError("read_pandas requires the 'pandas' extra: pip install ghanageo[pandas]") from error
    raw = _read(source)
    if format == "csv":
        return pd.read_csv(io.BytesIO(raw))
    if format == "json":
        return pd.read_json(io.BytesIO(raw))
    raise ValueError("pandas format must be 'csv' or 'json'")


def read_geopandas(source: bytes | str | Path) -> Any:
    """Load a GeoJSON artifact into GeoPandas. Install ``ghanageo[geopandas]`` first."""
    try:
        import geopandas as gpd  # type: ignore[import-untyped]
    except ImportError as error:
        raise ImportError("read_geopandas requires the 'geopandas' extra: pip install ghanageo[geopandas]") from error
    return gpd.read_file(io.BytesIO(_read(source)), driver="GeoJSON")


def read_json(source: bytes | str | Path) -> Any:
    return json.loads(_read(source))


def _read(source: bytes | str | Path) -> bytes:
    return source if isinstance(source, bytes) else Path(source).read_bytes()
