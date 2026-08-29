# RESOURCE_GONE

<!-- Generated from contracts/errors/catalog.yaml. DO NOT EDIT. -->

| | |
|---|---|
| **HTTP status** | `410` |
| **gRPC status** | `NOT_FOUND` |

This record was merged into another.

## When this happens

The id was deprecated or merged during reconciliation.

## How to fix it

Follow `details.mergedInto` and update your stored identifier.

## Fields in `error.details`

- `mergedInto`
- `reason`

## Example

```json
{
  "error": {
    "code": "RESOURCE_GONE",
    "message": "This record was merged into another.",
    "requestId": "req_01K3Y…",
    "docs": "/docs/errors/RESOURCE_GONE"
  }
}
```
