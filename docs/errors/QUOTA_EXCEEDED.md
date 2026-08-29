# QUOTA_EXCEEDED

<!-- Generated from contracts/errors/catalog.yaml. DO NOT EDIT. -->

| | |
|---|---|
| **HTTP status** | `429` |
| **gRPC status** | `RESOURCE_EXHAUSTED` |

The monthly quota for this plan is exhausted.

## When this happens

Cumulative cost-class usage passed the plan allowance.

## How to fix it

Upgrade the plan or wait for the quota window to reset.

## Fields in `error.details`

- `resetsAt`

## Example

```json
{
  "error": {
    "code": "QUOTA_EXCEEDED",
    "message": "The monthly quota for this plan is exhausted.",
    "requestId": "req_01K3Y…",
    "docs": "/docs/errors/QUOTA_EXCEEDED"
  }
}
```
