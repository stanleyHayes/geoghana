# INVALID_COORDINATES

<!-- Generated from contracts/errors/catalog.yaml. DO NOT EDIT. -->

| | |
|---|---|
| **HTTP status** | `400` |
| **gRPC status** | `INVALID_ARGUMENT` |

Coordinates are out of range.

## When this happens

Latitude outside -90..90 or longitude outside -180..180.

## How to fix it

Send latitude and longitude as decimal degrees, in that order.

## Fields in `error.details`

- `latitude`
- `longitude`

## Example

```json
{
  "error": {
    "code": "INVALID_COORDINATES",
    "message": "Coordinates are out of range.",
    "requestId": "req_01K3Y…",
    "docs": "/docs/errors/INVALID_COORDINATES"
  }
}
```
