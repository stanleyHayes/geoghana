# RADIUS_OUT_OF_RANGE

<!-- Generated from contracts/errors/catalog.yaml. DO NOT EDIT. -->

| | |
|---|---|
| **HTTP status** | `400` |
| **gRPC status** | `INVALID_ARGUMENT` |

Radius exceeds the maximum.

## When this happens

A nearby search asked for more than the documented maximum radius.

## How to fix it

Reduce radius to 50000 metres or fewer.

## Fields in `error.details`

- `maxRadiusMeters`

## Example

```json
{
  "error": {
    "code": "RADIUS_OUT_OF_RANGE",
    "message": "Radius exceeds the maximum.",
    "requestId": "req_01K3Y…",
    "docs": "/docs/errors/RADIUS_OUT_OF_RANGE"
  }
}
```
