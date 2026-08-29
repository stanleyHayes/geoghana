# QUERY_TOO_SHORT

<!-- Generated from contracts/errors/catalog.yaml. DO NOT EDIT. -->

| | |
|---|---|
| **HTTP status** | `400` |
| **gRPC status** | `INVALID_ARGUMENT` |

The search query is too short.

## When this happens

A search or autocomplete query was below the minimum length.

## How to fix it

Send at least 2 characters.

## Fields in `error.details`

- `minLength`

## Example

```json
{
  "error": {
    "code": "QUERY_TOO_SHORT",
    "message": "The search query is too short.",
    "requestId": "req_01K3Y…",
    "docs": "/docs/errors/QUERY_TOO_SHORT"
  }
}
```
