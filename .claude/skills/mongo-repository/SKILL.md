---
name: mongo-repository
description: Write or modify MongoDB repository code in this D&D campaign manager — queries, updates, indexes, collections, validation, and the dual ObjectID/string-ID identity scheme. Use this skill whenever touching anything under internal/infrastructure/database/mongodb/, writing a bson filter or update, adding a collection or index, persisting a new model, or debugging a duplicate-key error, a wiped field after a PUT, or a query returning nothing — even if the user just says "save this to the database". A full-struct $set here silently destroys data, so read this before writing any update.
---

# MongoDB repositories

All persistence lives in `internal/infrastructure/database/mongodb/`. Repositories hold a
`*Client` and reach collections through `r.client.Database().Collection(string(Name))`.

## Identity: every entity has two IDs

| | Type | Purpose |
|---|---|---|
| `ID` | `primitive.ObjectID`, `bson:"_id,omitempty"` | Mongo primary key. HTTP routes address entities by its hex. |
| `<Entity>ID` | `string`, e.g. `bson:"campaign_id"` | Business ID. **Uniquely indexed.** All cross-document references use this. |

The string ID is generated in the repository's Create method when blank:

```go
if campaign.CampaignID == "" {
    campaign.CampaignID = primitive.NewObjectID().Hex()
}
```

Never reference another document by `_id`. `Character.CampaignID`,
`Session.CombatEncounters`, `StoryEvent.SessionID` are all string business IDs.

## Never `$set` a whole struct — read before writing any update

```go
collection.UpdateOne(ctx, bson.M{"_id": entity.ID}, bson.M{"$set": entity})  // DON'T
```

Only `_id` carries `omitempty`. Every other field marshals even when zero, and the struct
came straight from `ShouldBindJSON`, so **any field the client omitted is written as its
zero value**. This was a live bug in both update methods; it blanked `campaign_id` (which
is uniquely indexed, so the *second* such update died with a duplicate key on `""`) and
zeroed `created_at`.

Build an explicit update document, as `UpdateCampaign` and `UpdateCharacter` now do:

```go
update := bson.M{
    "title":      campaign.Title,
    "status":     campaign.Status,
    "updated_at": time.Now().UTC(),
}
collection.UpdateOne(ctx, bson.M{"_id": campaign.ID}, bson.M{"$set": update})
```

**Immutable after creation:** `_id`, the string business ID, `campaign_id`, `created_at`.
Leave them out of every update document. Because they are omitted, a PUT handler re-reads
the document afterwards so the response still carries them.

Normalise optional slices with `emptyIfNil` so a field is `[]` rather than alternating
between `null` and missing.

Targeted helpers like `UpdateCharacterHP`, `AddStatusEffect` and `RemoveStatusEffect` use
dotted paths (`derived_stats.hit_points.current`) with `$addToSet` / `$pull`. Prefer that
shape for a narrow change.

## Return conventions — match them exactly

```go
// Reads: absent is (nil, nil), NOT an error
err := collection.FindOne(ctx, filter).Decode(&x)
if err != nil {
    if err == mongo.ErrNoDocuments {
        return nil, nil
    }
    return nil, fmt.Errorf("failed to find x: %w", err)
}

// Writes: absent IS an error, and a typed one
if result.MatchedCount == 0 {
    return models.NotFound("character")
}
```

Reads returning `(nil, nil)` means every caller must nil-check. Lists normalise `nil` to
an empty slice so JSON carries `[]`, not `null`.

Unexpected failures wrap with `%w` and read `failed to <verb> <entity>`.

## Validation lives here, not in handlers

There is no service layer. Create/Update methods validate required fields up front and
return typed errors from `internal/domain/models/errors.go`:

```go
if character.Name == "" {
    return models.Invalid("character name is required")
}
```

`handlers.respondRepoError` maps `models.ErrValidation` → **400**, `models.ErrNotFound` →
**404**, and anything else → an opaque **500** with the real error recorded on the gin
context. Returning a bare `fmt.Errorf` for a caller mistake sends a 500 — always use
`models.Invalid` / `models.NotFound`.

## Timestamps

Every model uses `time.Time`, always UTC:

```go
now := time.Now().UTC()
```

Create sets both `CreatedAt` and `UpdatedAt`; Update sets only `UpdatedAt` and leaves
`CreatedAt` out of the update document entirely. (`Character` once used
`primitive.DateTime`, which serialised as a JSON number while every other model emitted
RFC 3339 — do not reintroduce that.)

## Adding a collection

1. Constant in `collections.go` (`CollectionName` string type).
2. Append it to the slice in `InitializeCollections` — it is not derived from the
   constants, so a constant alone does nothing.
3. Add a `create<Entity>Indexes` function in `indexes.go` and call it from `CreateIndexes`.
4. Unique index on the string business ID; plain indexes on foreign keys and anything
   sorted or filtered.

Both run at startup from `main.go`. Index creation is idempotent, but **changing an
existing index's options requires dropping it manually** — `CreateMany` will not alter one
in place, it errors on conflict.

## Query safety and scoping

`SearchCharacters` runs its input through `regexp.QuoteMeta` before it reaches `$regex` —
raw input there is a regex-injection and ReDoS hole. For richer search, use the text index
already defined on `campaigns.title` + `campaigns.description`.

**Scope child reads to their parent.** Character get/update/delete filter on `_id` *and*
`campaign_id`; filtering on `_id` alone let a character be reached through any campaign's
URL. Note that `campaign_id` holds the campaign's **business ID**, not its `_id` hex —
handlers resolve one to the other.

Repository methods that return slices decode with `cursor.All`; always
`defer cursor.Close(ctx)`.

## Checklist

- [ ] Update builds an explicit `$set`, never the whole struct
- [ ] Immutable fields (`_id`, business ID, `campaign_id`, `created_at`) left out of it
- [ ] Read returns `(nil, nil)` when absent; write returns `models.NotFound`
- [ ] Caller mistakes return `models.Invalid`, not a bare `fmt.Errorf`
- [ ] Child entities filter on the parent ID as well as `_id`
- [ ] Lists normalise `nil` to `[]`
- [ ] String business ID generated when blank on create
- [ ] Timestamps set with `time.Now().UTC()`
- [ ] New collection registered in `InitializeCollections` *and* `CreateIndexes`
- [ ] `defer cursor.Close(ctx)` on every cursor
- [ ] User input never reaches `$regex` unescaped
- [ ] `go test ./internal/infrastructure/database/mongodb/...` passes
