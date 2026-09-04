# CLAUDE.md

AI-driven D&D 5e campaign manager. Go + Gin + MongoDB Atlas + any OpenAI-compatible AI
provider (Groq by default).
Module path: `github.com/dnd-campaign/manager`. Go 1.21.

## Current state — read this first

**The design docs describe far more than the code implements.** `docs/` is ~6,200 lines
of specification; `internal/` is ~3,400 lines of Go. Phase 1 (MVP foundation) is done;
Phases 2–5 are not started.

| Area | Status |
|---|---|
| Config loading, MongoDB client, indexes | Implemented, unit-tested |
| Campaign + Character models & repositories | Implemented |
| Campaign + Character REST CRUD | Implemented, campaign-scoped, cascading delete |
| AI client, prompt builder, AI service | Implemented, **not wired into the API** |
| Session / StoryEvent / CombatEncounter | **Models only** — no repository, no routes |
| Dice system, rules engine, combat tracker | **Spec only** (`docs/GAME_ENGINE.md`) — no code |
| Auth, rate limiting, Docker | **Config/docs only** — no code |

Consequences that bite:
- `pkg/`, `internal/application/`, `test/`, `cmd/migrator`, `cmd/seed` **do not exist**,
  though the README, `Makefile` and `docs/PROJECT_STRUCTURE.md` all reference them.
  `make migrate`, `make seed`, `make test-integration` fail.
- `docker-compose up` (README) fails — there is no compose file.
- The README's `export MONGODB_PASSWORD=...` is wrong; the variable is `MONGODB_URI`.

When a doc and the code disagree, **the code is the truth**. Treat `docs/` as the
intended design, not a description of what exists.

## Out of scope — do not reinstate

Cut on 2026-09-04 (rationale in `docs/ARCHITECTURE.md` §0). Propose these again only with
a measured reason, not because a doc still mentions them:

| Cut | Do this instead |
|---|---|
| Event sourcing (replay, snapshots, undo) | Append-only `story_events` ordered by `sequence_number` |
| Vector embeddings / semantic retrieval | Last N story events + a rolling summary, by recency |
| Redis | Nothing; an in-process map behind a mutex if ever needed |
| WebSocket / real-time | REST request/response; SSE for a single streamed narration |

**Still in scope:** the full D&D 5e rules engine specified in `docs/GAME_ENGINE.md`.

## Commands

```bash
make run-dev    # run from source (sources .env first)
make run        # build ./dnd-campaign-manager, then run it
make build
go test ./...   # 34 tests across handlers, ai, config, mongodb
make lint       # golangci-lint (not vendored — install separately)
```

`make run` / `make run-dev` source `.env` into the environment before starting, because
`configs/config.yaml` uses `${VAR}` placeholders. Running the binary directly without
those variables exported leaves the Mongo URI empty and startup fails.

## Configuration

Three layers, later wins: **defaults in `config.go` → `config.yaml` → environment**.

Search path: `.`, `./configs`, `/etc/dnd-campaign/`. A missing file is not an error —
defaults plus env alone are enough to boot.

Two non-obvious mechanics in `internal/infrastructure/config/config.go`, both
deliberate and both load-bearing:

1. **`SetEnvKeyReplacer(".", "_")`** maps nested keys onto flat env names
   (`mongodb.uri` → `MONGODB_URI`). Without it `AutomaticEnv` looks up `MONGODB.URI`,
   which no shell can set.
2. **The config file is read twice.** Viper does not interpolate `${VAR}`, so after
   `ReadInConfig` the located file is re-read through `os.ExpandEnv` and re-parsed.
   An unset variable expands to `""`, never to the literal `"${VAR}"` — a literal
   placeholder reaching the Mongo driver was a real past bug and
   `TestLoadDoesNotLeakUnexpandedPlaceholder` guards it.

`AutomaticEnv` only resolves keys Viper already knows, so secrets are additionally bound
explicitly via `BindEnv`. **Adding a new secret requires adding it to that map** — a
default or a YAML key alone will not pick up the environment variable. `ai.api_key` is
bound to several names in priority order (`GROQ_API_KEY`, `DEEPSEEK_API_KEY`,
`OPENAI_API_KEY`, `AI_API_KEY`) so switching providers does not mean renaming the variable
in every shell profile.

**There is no default `ai.base_url` or `ai.model`.** Guessing a provider would silently
send traffic somewhere nobody chose, so an incomplete config fails at startup via
`ai.ClientConfig.Validate` instead. Timeout and retries do have defaults.

URI resolution lives in `mongodb.buildConnectionURI`, which passes any `mongodb://` /
`mongodb+srv://` URI through verbatim and only assembles one from parts for a bare
`host:port`. A `GetMongoURI` helper on `MongoDBConfig` used to duplicate this incorrectly
(it re-wrapped an already-complete Atlas URI) and has been removed — do not reintroduce it.

## Layout and layering

```
cmd/server/main.go            composition root — all wiring lives here
internal/api/server.go        Gin engine, route table, graceful shutdown
internal/api/handlers/        HTTP handlers
internal/domain/models/       structs + Session behaviour methods
internal/infrastructure/
  config/                     Viper loader
  database/mongodb/           client, collections, indexes, repositories
  ai/                         DeepSeek client, prompt templates, service
examples/ai_usage.go          runnable AI demo (package main, separate binary)
```

Intended flow is `handler → service → repository`. **The service layer does not exist**:
handlers hold `*mongodb.CampaignRepository` / `*mongodb.CharacterRepository` concretely,
so validation lives in repositories and HTTP handlers import the Mongo package directly.
Follow the existing pattern when extending — do not introduce a lone interface for one
new type. If you add a service layer, do it deliberately and convert all handlers at once.

Dependencies are constructed in `main.go` and passed to `api.NewServer`. Adding a handler
means touching `NewServer`'s signature and the `Server` struct — there is no registry.

## Domain conventions

**Dual identity.** Every persisted entity carries both a Mongo `_id` (`primitive.ObjectID`)
and a separate string business ID (`campaign_id`, `character_id`, `session_id`, `event_id`).
The string ID is generated in the repository via `primitive.NewObjectID().Hex()` when blank,
and carries a **unique index**. Cross-document references always use the string ID, never
`_id`. HTTP routes address entities by `_id` hex.

**Timestamps are `time.Time` everywhere, always UTC.** `Character` previously used
`primitive.DateTime`, which serialised as a JSON number while every other model emitted
RFC 3339. Set them with `time.Now().UTC()`.

**Campaign scoping.** Characters belong to a campaign via the `campaign_id` string field,
and every character read, update and delete filters on `_id` **and** `campaign_id`, so a
character is only reachable through its own campaign's URL. The `:id` path segment is a
campaign `_id`; handlers resolve it to `campaign.CampaignID` before touching characters,
which also validates that the campaign exists. Deleting a campaign deletes its characters
first — the reverse order would strand unreachable orphans.

**Enums are bare strings.** Only session status and attendance have constants
(`models.SessionStatus*`, `models.Attendance*`). Character `type`
(`player` / `npc` / `enemy` / `monster`), campaign `status`, event types, damage types and
the rest are unconstrained strings validated nowhere. Prefer adding constants over
inventing a new literal.

## Repository conventions

- Repositories own validation and return errors wrapping the sentinels in
  `internal/domain/models/errors.go`: `models.Invalid(...)` for bad input,
  `models.NotFound(...)` for a missing document. `handlers.respondRepoError` maps those to
  **400** and **404**; anything else becomes an opaque **500** with the real error recorded
  on the gin context rather than sent to the client.
- "Not found" on reads is `(nil, nil)`, not an error. Handlers must nil-check before
  responding, or they will emit `null` with a 200.
- List methods normalise an empty result to `[]`, never `null`.

**Never `$set` a whole struct.** Build the update document field by field, as
`UpdateCampaign` and `UpdateCharacter` do. Only `_id` carries `omitempty`, so `$set`-ing a
struct decoded from a request body writes every omitted field as its zero value — that
blanked the uniquely indexed `campaign_id` (making the *second* such update fail with a
duplicate key on `""`) and zeroed `created_at`. `_id`, the business ID, `campaign_id` and
`created_at` are immutable after creation; a PUT handler re-reads the document afterwards
so the response still carries them.

`SearchCharacters` runs the caller's query through `regexp.QuoteMeta` before it reaches
`$regex`. Never interpolate user input into a query operator unescaped.

## AI layer

**Provider-agnostic.** `OpenAICompatibleClient` speaks the OpenAI `/chat/completions`
shape, which Groq, DeepSeek, OpenAI and local servers all serve. The vendor is
`ai.base_url` + `ai.model` in config — **no Go code names a provider**, and none should.
Swapping providers must stay a config change.

Pricing is config too (`ai.pricing`, USD per million tokens), because rates differ per
provider and model and change often. Both zero disables cost estimation. Model IDs also
churn — Groq retires them regularly — so confirm against the provider's `/models`
endpoint rather than trusting a name written down anywhere, including here.

`ai.Service` is the entry point; `ai.Client` is the provider interface and
`OpenAICompatibleClient` its only implementation. Templates live in `defaultPrompts()` in
`prompts.go` as `{System, User}` pairs; `PromptBuilder.BuildPrompt` substitutes
`{{var}}` by plain `strings.ReplaceAll`.

- Substitution is **not** a template engine, but it is now single-pass and strict:
  a `{{key}}` inside a substituted *value* is left alone (it used to expand, and Go's
  random map order made that nondeterministic), and a missing variable is an error rather
  than a brace left in the prompt. Pass every variable a template names.
- Templates must not contain Handlebars conditionals — nothing evaluates them. A test
  enforces this. Pre-format optional lines in Go and pass them as a plain variable
  (`combat_narration` takes `damage_line`).
- Player-controlled text still reaches the prompt unescaped. Keep it out of the **system**
  prompt, which is where the DM's rules live.
- Temperature is not per-call; it comes from the `TemperatureSettings` registry keyed by
  task type via `GetTemperature`. Add a key when adding a task.
- Retry lives in `ChatCompletion` and in opening a stream, driven by `Error.Retriable`
  (429/500/502/503/504 and network errors). Backoff is exponential, 1s→2s→4s capped at 30s
  — but a provider's own `Retry-After` header wins when present, capped the same way.
  This matters on Groq's free tier, where guessing burns quota that the header would have
  spent correctly.
- `StreamChatCompletion` returns a `*ChatStream`: range over `Chunks`, **then check
  `Err()`**. A stream that dies mid-flight closes its channel exactly like a clean finish,
  so without the `Err()` check a truncated narrative looks complete.
- `Service.calculateCost` applies the configured `Pricing`, charging prompt and
  completion tokens at their separate rates.

Five templates (`narrative_generation`, `combat_narration`, `story_adaptation`,
`character_backstory`, `quest_generation`) have no corresponding `Service` method yet, so
nothing can reach them.

## Docs map

`ARCHITECTURE.md` hexagonal design and rationale · `GAME_ENGINE.md` full dice/rules/combat
spec (unimplemented, this is the blueprint for Phase 2) · `DATA_MODELS.md` schema reference ·
`API_DESIGN.md` planned endpoint surface · `AI_INTEGRATION.md` + `AI_CONTEXT_PROMPTS.md`
prompt strategy · `MONGODB_SETUP.md` Atlas setup · `IMPLEMENTATION_ROADMAP.md` phase plan ·
`PROJECT_STRUCTURE.md` aspirational tree.

## Working agreements

- **Never run `git commit` or `git push`.** The user owns every commit.
- Secrets live in `.env` (gitignored). `.env.example` documents the shape. Never read
  `.env` values into output or commit them.
- Skills for recurring workflows live in `.claude/skills/`.
