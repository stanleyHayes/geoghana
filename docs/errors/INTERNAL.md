# INTERNAL

<!-- Generated from contracts/errors/catalog.yaml. DO NOT EDIT. -->

| | |
|---|---|
| **HTTP status** | `500` |
| **gRPC status** | `INTERNAL` |

An unexpected error occurred.

## When this happens

An unhandled failure. Always logged with the request id.

## How to fix it

Retry. If it persists, contact support quoting the requestId.

## Example

```json
{
  "error": {
    "code": "INTERNAL",
    "message": "An unexpected error occurred.",
    "requestId": "req_01K3Y…",
    "docs": "/docs/errors/INTERNAL"
  }
}
```
