# NOT_FOUND

<!-- Generated from contracts/errors/catalog.yaml. DO NOT EDIT. -->

| | |
|---|---|
| **HTTP status** | `404` |
| **gRPC status** | `NOT_FOUND` |

No record matches that identifier.

## When this happens

The id does not exist in the current dataset version.

## How to fix it

Check the id, or search for the place by name.

## Fields in `error.details`

- `id`

## Example

```json
{
  "error": {
    "code": "NOT_FOUND",
    "message": "No record matches that identifier.",
    "requestId": "req_01K3Y…",
    "docs": "/docs/errors/NOT_FOUND"
  }
}
```
