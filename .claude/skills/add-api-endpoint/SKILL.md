---
name: add-api-endpoint
description: Wire a new REST endpoint or resource end-to-end in this D&D campaign manager — model, repository, handler, route, and main.go dependency injection. Use this skill whenever the user wants to add, expose, or change an HTTP endpoint, add CRUD for a resource (sessions, story events, combat encounters, dice, AI), mentions a new route or path, or asks why a request 404s or returns the wrong status — even if they don't say "endpoint". There is no route registry or DI container here, so every new handler requires edits in four specific files; skipping one silently produces a 404 or a nil-pointer panic at startup.
---

# Adding an API endpoint

Wiring is entirely manual. A new handler touches **four files**, and there is no
framework magic that will pick it up for you.

## The four edits, in order

### 1. Repository — `internal/infrastructure/database/mongodb/<entity>_repository.go`

Only if the resource has no repository yet. Follow `campaign_repository.go`.
See the `mongo-repository` skill for the conventions and the full-struct `$set` trap.

Add the collection constant to `collections.go` and its indexes to `indexes.go` if the
entity is new. `InitializeCollections` already lists all six collections, so a genuinely
new collection needs adding there too.

### 2. Handler — `internal/api/handlers/<entity>_handler.go`

Follow `campaign_handler.go` exactly:

```go
type XHandler struct {
    xRepo *mongodb.XRepository   // concrete type, not an interface
}

func NewXHandler(xRepo *mongodb.XRepository) *XHandler {
    return &XHandler{xRepo: xRepo}
}
```

Handlers hold the concrete repository. **Do not introduce an interface for one new
handler** — it would be the only one in the codebase and buys nothing until a service
layer exists. A handler may hold more than one repository when it genuinely needs it:
`CampaignHandler` also takes the character repository so a delete can cascade, and
`CharacterHandler` takes the campaign repository to resolve its parent.

Per-method shape:

```go
func (h *XHandler) GetX(c *gin.Context) {
    id, err := primitive.ObjectIDFromHex(c.Param("id"))
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid x ID"})
        return
    }

    x, err := h.xRepo.GetXByID(c.Request.Context(), id)
    if err != nil {
        respondRepoError(c, err)   // maps 400 / 404 / opaque 500
        return
    }
    if x == nil {                       // reads return (nil, nil) when absent
        c.JSON(http.StatusNotFound, gin.H{"error": "x not found"})
        return
    }

    c.JSON(http.StatusOK, x)
}
```

Rules that are easy to get wrong:

- **Always pass `c.Request.Context()`**, never `context.Background()`.
- **Always nil-check reads.** Repositories return `(nil, nil)` for "not found". Skipping
  the check emits `null` with a 200.
- Route repository errors through `respondRepoError` (`handlers/errors.go`). It maps
  `models.ErrValidation` → 400, `models.ErrNotFound` → 404, everything else → an opaque
  500 with the real error recorded on the context. Never hand `err.Error()` straight to
  the client for an unexpected failure — that leaks driver internals.
- Errors are `gin.H{"error": string}`. There is no error envelope type — do not invent
  one for a single endpoint.
- Create → 201, delete → 204 with a nil body, everything else → 200.
- Bind with `c.ShouldBindJSON(&entity)`. Binding the full model lets a client set any
  field, so **the path always wins**: overwrite server-owned fields after binding
  (`entity.ID = id`, `character.CampaignID = campaignID`). Repositories additionally clear
  a client-supplied `_id` on create.
- After a PUT, re-read the document before responding. Updates deliberately omit the
  immutable fields, so echoing the bound struct would return them as zero values.

### 3. Route — `internal/api/server.go`, in `setupRoutes`

Add to the `v1` group. Nested resources take the parent's `:id`:

```go
v1.POST("/campaigns/:id/sessions", s.sessionHandler.CreateSession)
```

Gin **cannot register two different parameter names at the same path position**.
`/campaigns/:id/...` is already established, so a sibling route must also use `:id`,
not `:campaign_id`. Mixing them panics at startup with a wildcard conflict.

### 4. Wiring — `internal/api/server.go` struct + `cmd/server/main.go`

Three separate spots, all required:

- add the field to the `Server` struct,
- add the parameter to `NewServer(...)` and assign it in the literal,
- in `main.go`, construct the repository and handler and pass them to `api.NewServer`.

`NewServer` takes handlers as positional parameters. Adding one changes its signature —
that is expected here; update the single call site in `main.go`.

## Checklist

- [ ] Repository method exists and filters by the right field
- [ ] Collection constant + indexes registered if the entity is new
- [ ] Handler nil-checks reads, uses `c.Request.Context()`, routes errors through `respondRepoError`
- [ ] Nested resource resolves and filters on its parent ID
- [ ] PUT re-reads before responding
- [ ] Route added under `v1`, reusing `:id` for the campaign segment
- [ ] `Server` struct field + `NewServer` parameter + assignment
- [ ] `main.go` constructs repo and handler and passes them
- [ ] `go build ./... && go vet ./...` clean
- [ ] README's endpoint list updated if the surface changed

## Nested resources: scope to the parent

The `:id` segment is a campaign **`_id` hex**, but children reference their campaign by
its **business `campaign_id`**. Those are different values, so resolve one to the other
before touching a child — `CharacterHandler.resolveCampaignID` does this, and it doubles
as an existence check on the parent:

```go
campaignID, ok := h.resolveCampaignID(c)   // 400 on bad hex, 404 if no such campaign
if !ok {
    return
}
```

Then filter on the parent ID as well as `_id`, so a child cannot be reached through an
unrelated parent's URL:

```go
bson.M{"_id": charID, "campaign_id": campaignID}
```

Deleting a parent must delete its children first; the reverse order strands orphans that
no URL can reach.
