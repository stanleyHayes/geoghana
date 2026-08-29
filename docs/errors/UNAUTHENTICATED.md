# UNAUTHENTICATED

<!-- Generated from contracts/errors/catalog.yaml. DO NOT EDIT. -->

| | |
|---|---|
| **HTTP status** | `401` |
| **gRPC status** | `UNAUTHENTICATED` |

No valid credential was supplied.

## When this happens

The API key was missing or malformed.

## How to fix it

Send an API key as `Authorization: Bearer gh_live_...`.

## Example

```json
{
  "error": {
    "code": "UNAUTHENTICATED",
    "message": "No valid credential was supplied.",
    "requestId": "req_01K3Y…",
    "docs": "/docs/errors/UNAUTHENTICATED"
  }
}
```
