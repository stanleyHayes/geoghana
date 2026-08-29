# KEY_REVOKED

<!-- Generated from contracts/errors/catalog.yaml. DO NOT EDIT. -->

| | |
|---|---|
| **HTTP status** | `401` |
| **gRPC status** | `UNAUTHENTICATED` |

This API key is no longer valid.

## When this happens

The key was revoked or has expired.

## How to fix it

Rotate the key in the developer portal.

## Example

```json
{
  "error": {
    "code": "KEY_REVOKED",
    "message": "This API key is no longer valid.",
    "requestId": "req_01K3Y…",
    "docs": "/docs/errors/KEY_REVOKED"
  }
}
```
