# ORIGIN_NOT_ALLOWED

<!-- Generated from contracts/errors/catalog.yaml. DO NOT EDIT. -->

| | |
|---|---|
| **HTTP status** | `403` |
| **gRPC status** | `PERMISSION_DENIED` |

This origin is not on the key's allow-list.

## When this happens

A browser key was used from an unlisted origin.

## How to fix it

Add the origin to the application's allowed origins.

## Fields in `error.details`

- `origin`

## Example

```json
{
  "error": {
    "code": "ORIGIN_NOT_ALLOWED",
    "message": "This origin is not on the key's allow-list.",
    "requestId": "req_01K3Y…",
    "docs": "/docs/errors/ORIGIN_NOT_ALLOWED"
  }
}
```
