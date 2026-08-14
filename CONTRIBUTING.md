# Contributing — Authress Terraform Provider

## Resource Creation Pattern: POST → Adopt-or-Error

Every resource that implements `Create` MUST follow this pattern:

```
1. POST to create the resource
2. If 201/200 → success, set state, return
3. If 409 (conflict / already exists):
   a. GET the existing resource
   b. Compare all configurable fields between planned and existing
   c. If they MATCH → adopt (set state from planned, return success)
   d. If they DIFFER → error with field-by-field mismatch report
4. Any other error → surface to user with HTTP status and body
```

### Why

- **Never blindly PUT over an existing resource.** PUT is destructive — it overwrites whatever is there. If a resource was created outside Terraform (manually, by another pipeline, or by a different TF root), overwriting it silently would cause data loss.
- **Adoption is safe.** If the existing resource already matches what TF wants, there's nothing to do — just record it in state.
- **Mismatch = human decision needed.** If the existing resource differs, the user must decide: update their config to match reality, or delete-and-recreate.

### Implementation checklist for new resources

1. `Create` method calls the POST/create API endpoint
2. On 409: call GET, then `collectXxxMismatches(planned, existing)`
3. If mismatches are nil → adopt (set state, return)
4. If mismatches are non-nil → `formatMismatches(...)` error
5. `Update` method uses PUT — this is safe because TF owns the resource in state
6. `Read` method uses GET — 404 removes from state
7. `Delete` method uses DELETE — 404 is a no-op

### Shared helpers (src/adopt.go)

- `compareField(field, expected, actual) *FieldMismatch`
- `collectMismatches(checks ...*FieldMismatch) []FieldMismatch`
- `formatMismatches(resourceType, resourceId, mismatches) string`

### Inline access records (service_client statement blocks)

The `upsertServiceClientAccessRecord` helper follows the same pattern:
POST → 409 → GET → compare → adopt-or-error. It builds an
`AuthressAccessRecordResource` from the inline statements and reuses
`collectAccessRecordMismatches`.

## Computed Attributes in Update

When reading resource identity (like `client_id`) during an Update operation,
ALWAYS read from `req.State` (current), never `req.Plan` (planned). Computed
attributes are not populated in the plan during Update — they only exist in state.
