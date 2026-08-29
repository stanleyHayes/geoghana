# QUERY_TOO_COMPLEX

<!-- Generated from contracts/errors/catalog.yaml. DO NOT EDIT. -->

| | |
|---|---|
| **HTTP status** | `400` |
| **gRPC status** | `INVALID_ARGUMENT` |

The GraphQL query exceeds the complexity budget.

## When this happens

Query depth or weighted field cost passed the allowed budget.

## How to fix it

Request fewer nested fields, or paginate with a smaller page size.

## Fields in `error.details`

- `cost`
- `maxCost`
- `depth`
- `maxDepth`

## Example

```json
{
  "error": {
    "code": "QUERY_TOO_COMPLEX",
    "message": "The GraphQL query exceeds the complexity budget.",
    "requestId": "req_01K3Y…",
    "docs": "/docs/errors/QUERY_TOO_COMPLEX"
  }
}
```
