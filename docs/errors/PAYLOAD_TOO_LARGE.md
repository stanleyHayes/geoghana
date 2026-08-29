# PAYLOAD_TOO_LARGE

<!-- Generated from contracts/errors/catalog.yaml. DO NOT EDIT. -->

| | |
|---|---|
| **HTTP status** | `413` |
| **gRPC status** | `RESOURCE_EXHAUSTED` |

The request body is too large.

## When this happens

A request body or gRPC message exceeded the size limit.

## How to fix it

Split the request into smaller batches.

## Example

```json
{
  "error": {
    "code": "PAYLOAD_TOO_LARGE",
    "message": "The request body is too large.",
    "requestId": "req_01K3Y…",
    "docs": "/docs/errors/PAYLOAD_TOO_LARGE"
  }
}
```
