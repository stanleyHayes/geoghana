from __future__ import annotations

import argparse
import hashlib
import json
import shutil
import subprocess
import tomllib
from pathlib import Path
from runpy import run_path
from typing import Any

SDK = Path(__file__).resolve().parents[1]
ROOT = SDK.parents[1]
LOCK = SDK / "contract.lock.json"
RUNNER_VERSION = "1.0.0"
TOOL_NAMES = ("httpx", "build", "hatchling", "mypy", "ruff")


def sha256(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


def locked_versions() -> dict[str, str]:
    document = tomllib.loads((SDK / "uv.lock").read_text())
    return {package["name"]: package["version"] for package in document["package"]}


def generated_lock() -> dict[str, Any]:
    ruby = shutil.which("ruby")
    if ruby is None:
        raise RuntimeError("Ruby is required to export the canonical conformance digest")
    conformance = json.loads(
        subprocess.check_output(  # noqa: S603 -- resolved executable and repository-owned script
            [ruby, "tools/conformance/export_cases.rb"], cwd=ROOT, text=True
        )
    )
    versions = locked_versions()
    sdk_versions = run_path(str(SDK / "src/ghanageo/_version.py"))
    return {
        "schemaVersion": 1,
        "apiVersion": sdk_versions["api_version"],
        "testedDatasetVersion": sdk_versions["tested_dataset_version"],
        "contracts": {
            "openapi": {
                "path": "contracts/openapi/v1.yaml",
                "sha256": sha256(ROOT / "contracts/openapi/v1.yaml"),
            },
            "protobuf": {
                "path": "proto/ghanageo/v1/geography.proto",
                "sha256": sha256(ROOT / "proto/ghanageo/v1/geography.proto"),
            },
            "conformance": {
                "digest": conformance["contractDigest"],
                "runnerVersion": RUNNER_VERSION,
            },
        },
        "generator": {
            "name": "ghanageo-hand-mapped-openapi-3.1",
            "version": "1",
            "pydantic": versions["pydantic"],
        },
        "tools": {
            "python": ">=3.12,<3.15",
            **{name: versions[name] for name in TOOL_NAMES},
        },
    }


def rendered(value: dict[str, Any]) -> str:
    return json.dumps(value, indent=2, ensure_ascii=False) + "\n"


def main() -> int:
    parser = argparse.ArgumentParser(description="Regenerate or verify Python SDK contract.lock.json")
    parser.add_argument("--write", action="store_true", help="write the canonical lock instead of checking it")
    args = parser.parse_args()
    expected = rendered(generated_lock())
    if args.write:
        LOCK.write_text(expected)
        return 0
    actual = LOCK.read_text() if LOCK.exists() else ""
    if actual != expected:
        print("contract.lock.json is stale; run: python scripts/check_contract_lock.py --write")
        return 1
    print("contract.lock.json matches canonical contracts and locked tools")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
