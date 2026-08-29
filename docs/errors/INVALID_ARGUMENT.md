# INVALID_ARGUMENT

<!-- Generated from contracts/errors/catalog.yaml. DO NOT EDIT. -->

| | |
|---|---|
| **HTTP status** | `400` |
| **gRPC status** | `INVALID_ARGUMENT` |

A request parameter was malformed or missing.

## When this happens

A parameter failed wire-format validation.

## How to fix it

Check the parameter names and types against the OpenAPI contract.

## Example

```json
{
  "error": {
    "code": "INVALID_ARGUMENT",
    "message": "A request parameter was malformed or missing.",
    "requestId": "req_01K3Y…",
    "docs": "/docs/errors/INVALID_ARGUMENT"
  }
}
```
