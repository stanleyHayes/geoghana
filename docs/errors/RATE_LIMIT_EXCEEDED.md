# RATE_LIMIT_EXCEEDED

<!-- Generated from contracts/errors/catalog.yaml. DO NOT EDIT. -->

| | |
|---|---|
| **HTTP status** | `429` |
| **gRPC status** | `RESOURCE_EXHAUSTED` |

Request rate exceeded.

## When this happens

The burst limit for this key, application or IP was hit.

## How to fix it

Retry after the interval in the Retry-After header.

## Fields in `error.details`

- `retryAfterSeconds`
- `limit`

## Example

```json
{
  "error": {
    "code": "RATE_LIMIT_EXCEEDED",
    "message": "Request rate exceeded.",
    "requestId": "req_01K3Y…",
    "docs": "/docs/errors/RATE_LIMIT_EXCEEDED"
  }
}
```
