# PERMISSION_DENIED

<!-- Generated from contracts/errors/catalog.yaml. DO NOT EDIT. -->

| | |
|---|---|
| **HTTP status** | `403` |
| **gRPC status** | `PERMISSION_DENIED` |

This key lacks the required scope.

## When this happens

The operation needs a scope the key was not granted.

## How to fix it

Grant the required scope in the developer portal.

## Fields in `error.details`

- `requiredScope`

## Example

```json
{
  "error": {
    "code": "PERMISSION_DENIED",
    "message": "This key lacks the required scope.",
    "requestId": "req_01K3Y…",
    "docs": "/docs/errors/PERMISSION_DENIED"
  }
}
```
