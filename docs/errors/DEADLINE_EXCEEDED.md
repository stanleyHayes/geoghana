# DEADLINE_EXCEEDED

<!-- Generated from contracts/errors/catalog.yaml. DO NOT EDIT. -->

| | |
|---|---|
| **HTTP status** | `504` |
| **gRPC status** | `DEADLINE_EXCEEDED` |

The request took too long and was cancelled.

## When this happens

A query or database statement passed its timeout.

## How to fix it

Narrow the query, or retry with a smaller radius or page size.

## Example

```json
{
  "error": {
    "code": "DEADLINE_EXCEEDED",
    "message": "The request took too long and was cancelled.",
    "requestId": "req_01K3Y…",
    "docs": "/docs/errors/DEADLINE_EXCEEDED"
  }
}
```
