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
| Campaign + Character + Monster REST CRUD | Implemented, campaign-scoped, cascading delete |
| AI client, prompt builder, AI service | Implemented, **not wired into the API** |
| 5e rules vocabulary + derived stats | Implemented in `models`, unit-tested |
| Classes, subclasses, races, backgrounds, multiclassing | Implemented as tables in `models`, unit-tested |
| Monster statblocks | Implemented — model, repository, REST CRUD, SRD seed catalogue |
| Session / StoryEvent / CombatEncounter | **Models only** — no repository, no routes |
| Dice roller | Implemented (`internal/domain/dice`), seedable |
| Rules engine (checks, saves, attacks) | Implemented (`internal/domain/rules`) |
| Combat tracker (turn order, rounds) | **Spec only** (`docs/GAME_ENGINE.md`) — no code |
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
go test ./...   # 206 tests, all offline — no provider is ever called
go run ./examples          # one full turn against the stub, free
go run ./examples -live    # the same prompts, sent for real
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
internal/api/middleware/      request id, logging, recovery, errors, CORS, rate limit
internal/domain/dice/         the only source of randomness — seedable for tests; also
                              probability.go, which is exact maths and rolls nothing
internal/domain/rules/        authoritative resolution; returns facts, not prose
internal/domain/models/       entities (Character, Campaign, Session, Monster) plus the
                              typed 5e vocabulary (abilities, skills, conditions, items,
                              spells, dice) and the rules encoded as methods
internal/infrastructure/
  config/                     Viper loader
  database/mongodb/           client, collections, indexes, repositories
  ai/                         OpenAI-compatible client, prompt templates, service
examples/ai_usage.go          runnable AI demo (package main, separate binary)
```

Intended flow is `handler → service → repository`. **The service layer does not exist**:
handlers hold `*mongodb.CampaignRepository` / `*mongodb.CharacterRepository` concretely,
so validation lives in repositories and HTTP handlers import the Mongo package directly.
Follow the existing pattern when extending — do not introduce a lone interface for one
new type. If you add a service layer, do it deliberately and convert all handlers at once.

Dependencies are constructed in `main.go` and passed to `api.NewServer`. Adding a handler
means touching `NewServer`'s signature and the `Server` struct — there is no registry.

## HTTP middleware

`internal/api/middleware` is the whole chain; `server.setupMiddleware` installs it and
nothing else. Gin's own `gin.Logger()` and `gin.Recovery()` are deliberately *not* used —
they write plain text to stdout and answer a panic with an empty body, neither of which a
JSON API or a browser client can work with.

**Order is load-bearing** and the comment in `setupMiddleware` says why:

```
RequestID → Logger → Recovery → ErrorHandler → CORS → RateLimit → handlers
```

- `RequestID` first, so everything downstream has one to log. It reuses an inbound
  `X-Request-ID` when the caller supplies one (so a UI can correlate) and always echoes
  the value back on the response. Read it with `middleware.RequestIDFrom(c)`.
- `Recovery` answers a panic with `500` JSON carrying the request id. It logs the stack
  and **never sends it** — a stack trace in a response body is an information leak.
- `ErrorHandler` turns a handler that called `c.Error(err)` and then wrote nothing into a
  `500` JSON body. If the handler already wrote a response it leaves it alone; this is a
  safety net, not a replacement for a handler answering properly itself
  (`handlers.respondRepoError` for repository errors, `badRequest` for bad input).
- `CORS` before `RateLimit`, so a browser preflight is answered rather than spending a
  token. It echoes the requesting origin rather than `*` (required once credentials are
  in play), sets `Vary: Origin`, and answers `OPTIONS` with `204`. A disallowed origin is
  still *served* — it simply gets no `Access-Control-Allow-Origin` header, because the
  browser is what enforces CORS, not the server. Empty `allowed_origins` disables it.
- `RateLimit` is a hand-rolled token bucket keyed on `c.ClientIP()` — no new dependency.
  `requests_per_minute: 0` disables it; `burst` defaults to a quarter of the rate.
  A refusal is `429` with a `Retry-After` computed from when a token will actually exist,
  not a guess. Idle buckets are swept after ten minutes so a long-lived process does not
  accumulate one entry per address ever seen.

The limiter is tested through `rateLimitWithClock`, an unexported constructor taking a
`now func() time.Time`. **Do not test it against wall time** — that is how you get a suite
that fails on a slow machine.

`rate_limit.ai_requests_per_hour` exists in config but is not wired to anything yet. It
belongs on the AI client (a budget on provider calls), not in HTTP middleware.

## Domain conventions

**Dual identity.** Every persisted entity carries both a Mongo `_id` (`primitive.ObjectID`)
and a separate string business ID (`campaign_id`, `character_id`, `monster_id`,
`session_id`). The string ID is generated in the repository via
`primitive.NewObjectID().Hex()` when blank, and carries a **unique index**. Cross-document
references always use the string ID, never `_id`. HTTP routes address entities by `_id` hex.

**Timestamps are `time.Time` everywhere, always UTC.** Set them with `time.Now().UTC()`.

**Campaign scoping.** Characters belong to a campaign via the `campaign_id` string field,
and every character read, update and delete filters on `_id` **and** `campaign_id`, so a
character is only reachable through its own campaign's URL. The `:id` path segment is a
campaign `_id`; handlers resolve it to `campaign.CampaignID` before touching characters,
which also validates that the campaign exists. Deleting a campaign deletes its characters
first — the reverse order would strand unreachable orphans.

### The 5e vocabulary is typed — use it

`abilities.go`, `skills.go`, `conditions.go`, `items.go`, `spells.go` and `dice.go` hold the
shared game vocabulary. **Never reintroduce a bare `string` for one of these.**

| Type | Notes |
|---|---|
| `Ability` | six constants; `AbilityScores.Modifier(a)` |
| `Skill` | eighteen constants; `SkillAbility` is the only skill→ability table |
| `Proficiency` | `none` / `half` / `proficient` / `expertise` — **not a bool**, Expertise doubles and Jack of All Trades halves |
| `Condition` | closed set of fourteen flags; exhaustion is `Character.Exhaustion` (0–6) because it has degrees |
| `DamageType` | thirteen constants; `DamageAffinity` applies resist/immune/vulnerable |
| `RollMode` | `Combine` never stacks — advantage plus disadvantage is a normal roll |
| `CharacterType` | `player` / `npc` only. Hostiles are `Monster`, not characters |
| `Class` / `Race` / `Background` | closed sets backed by `ClassTable`, `RaceTable`, `BackgroundTable` |

### Character sheets are table-driven

`class.go`, `race.go` and `background.go` are the **single source of truth** for what
each determines. Never hand-enter a value one of these tables can derive.

`BasicInfo.Classes` is a `[]ClassLevel`, because 5e has three different notions of level
that one integer conflated:

| Notion | Method | Used for |
|---|---|---|
| Total level | `BasicInfo.TotalLevel()` | proficiency bonus, level gates |
| Caster level | `SpellSlotsForClasses` | spell slots |
| Per-class levels | `HitDiceForClasses` | hit dice pools (3d10 + 2d6) |

Multiclass rules that are easy to get wrong, all tested:

- **Half and third casters round *up* single-classed but *down* multiclassed.** A paladin
  gets slots at level 2 from their own table, but contributes `floor(level/2)` to a
  combined caster level. That asymmetry is real 5e, not a bug.
- **Pact magic never merges.** Warlock slots are returned separately by
  `SpellSlotsForClasses` and come back on a *short* rest.
- **Only the first class grants saving throw proficiencies** (`GrantedSaveProficiencies`).
- **Subclass casting starts at the subclass level** — an Eldritch Knight 2 casts nothing.
- **Multiclass prerequisites are OR-of-ANDs**: fighter is "STR 13 or DEX 13", monk is
  "DEX 13 and WIS 13". Use `MeetsMulticlassPrerequisites`.
- **Multiclassing into bard, ranger or rogue grants one extra skill**
  (`MulticlassSkillChoices`); no other class does.

**Subclasses carry mechanics, not just names.** `SubclassDefinition` holds a key, display
name, `Source` (currently all `SourcePHB`), optional `Casting`, and `CritRangeAt`. Look
them up with `Class.Subclass(key)`.

**`ResolveAttack(roll, targetAC, critRange)` takes a crit range** — pass
`Character.CritRange()`, not `NaturalCrit`, or a Champion silently loses the one thing
their archetype does (19–20 from level 3, 18–20 from 15).

**Budgets are enforced.** `SkillBudget()` = granted (race + background) + first class's
choices + later classes' `MulticlassSkillChoices` + racial picks. `ExpertiseBudget()` is
2 per grant (rogue at 1 and 6, bard at 3 and 10). `ValidateSheet` rejects exceeding either
— checking only that each skill had *a* source let a rogue claim all eleven on their list.

**Races are table-driven too**, and carry mechanics rather than trait names:

- `RacialProficiencies(subrace)` grants real training — Dwarven Combat Training's four
  weapons, a mountain dwarf's light/medium armour, elf and drow weapon training, a rock
  gnome's tinker's tools. `ApplyClassDefaults` merges them, so a mountain dwarf wizard is
  genuinely proficient in the chain shirt they wear.
- `BonusHitPointsPerLevel` carries Dwarven Toughness, counted by
  `ExpectedMaxHitPoints()` (full hit die at first level, class average after, plus CON and
  the racial bonus each level).
- **Draconic ancestry is a subrace**, not a trait name — all ten dragons, each with damage
  type, breath shape, save ability and matching resistance. `Character.BreathWeapon()`
  returns the weapon, dice for the level, and DC (8 + CON + proficiency).
- **Humans require a subrace**: `standard` (+1 to all six) or `variant` (+1 to two, one
  skill, one feat). The ability spread moved off the race onto the subraces, so a bare
  `Race: RaceHuman` with no subrace now fails validation.
- `AttackRollMode(item)` returns disadvantage for a **Small creature wielding a Heavy
  weapon** and for exhaustion 3+. Carried on every `AttackProfile` as `Mode`.
- Non-PHB races (Aasimar, Genasi, Goliath, Tabaxi, Firbolg) are included and tagged;
  `RacesFromSource(SourcePHB)` filters to core.

### Monsters and combat

**Monsters are an open catalogue, not a closed table.** Unlike classes and races they are
stored, not enumerated — `SRDMonsters()` is seed data a campaign copies and may edit, never
a source of truth. `POST /campaigns/:id/monsters/seed` stamps copies into a campaign
(idempotent by name).

**`Combatant` is the single combat representation** for both sources. Build one with
`Monster.ToCombatant(id)` or `Character.ToCombatant(id)` — never construct it by hand, or
affinities and the death-save flag get missed.

**`Combatant.TakeDamage(amount, damageType, critical)` takes a damage type.** It used to
take a bare amount, so resistances and immunities were unreachable on the only path that
matters and a fire-immune creature took full fire damage in every encounter. Affinities and
condition immunities are copied onto the combatant at conversion time.

**`MakesDeathSaves` separates the two kinds of creature**: characters drop to *dying* and
roll to stabilise, monsters die at 0. `LegendaryResistanceRemaining` tracks the boss
resource.

Statblocks derive `ProficiencyBonus()`, `XP()` and `PassivePerception()` from CR and
ability scores rather than storing them. `Monster.Validate()` is the statblock counterpart
of `ValidateSheet()` and runs on create only; it checks damage types, conditions, that a
multiattack names actions the monster actually has, and that printed hit points match what
`HitDice` averages to (`ParseHitDiceFormula`).

### The two-call turn

One player sentence becomes three steps, and the split is the whole design:

```
"I stab the goblin"
  → ExtractIntent   temperature 0, JSON mode, closed option lists → models.Intent
  → rules.Engine    decides everything; returns Facts()
  → NarrateAction   describes those facts; decides nothing
```

**`ExtractIntent` is a parser, not a DM.** It runs at temperature 0 in JSON mode and is
handed `models.ActionOptionsFor(character, targets)` — the exact weapons, spells, items and
creatures available — so it chooses from closed lists rather than inventing. The returned
`Intent` is a *proposal*: `Intent.Validate(options)` rejects a weapon the character is not
carrying, and `ExtractIntent` converts a failure into a clarifying question rather than
letting a hallucination reach the engine. `ParseIntent` strips markdown fences and
surrounding prose, and normalises casing (`"Sleight of Hand"` → `sleight_of_hand`).

**`narrationContract` in `prompts.go` is the load-bearing paragraph of the AI layer.**
Every template that describes an outcome prepends it: never change a number, never decide
an outcome, never roll, never apply a condition. `NarrateAction` and `NarrateCheck` take
the engine's `Facts()` map verbatim — facts are copied over style values last, so a style
field can never overwrite one — and refuse outright when handed no facts.

**`dm_base` no longer claims rules authority.** It used to say "Enforce: apply D&D 5e
rules" and emit "Game State Changes", which contradicted `docs/ARCHITECTURE.md` §3.1. Tests
now fail if either phrase returns.

**Every template has a Service method**, enforced by a test: an uncallable prompt drifts
out of date unnoticed. Optional fields fall back to placeholders because an empty value in
a prompt reads as an invitation to invent something.

**Test the AI offline.** `ai.NewStubService(replies...)` returns a Service backed by
`StubClient`, which records every request. `stub.LastPrompt()` is what assertions about
wording use. Calling a real model to check prompt assembly is slow, costs money and tells
you nothing about your own code — only prose quality needs `-live`.

**Progression tables** live in `class_progression.go`: `CantripsKnown()`, `SpellsKnown()`
(for classes that know a fixed list), `PreparedSpellLimit()` (ability mod + level, half
level for paladins, minimum 1), and `ClassFeatures` / `FeaturesAtLevel` /
`FeaturesThroughLevel`. `StartingEquipmentTable` covers creation kit.

Two caveats on that data: `ClassFeatures` is a **display and planning reference**, not
something resolution reads — resolution uses `ClassTable` and `SubclassDefinition`. And
the Eldritch Knight / Arcane Trickster spells-known numbers are the least certain entries
in the file; the code says so where they are defined.

### Attacks, rests and encumbrance

**Proficiencies live on the character** (`Character.Proficiencies`), not just in the class
table — an attack bonus cannot be computed without knowing whether the wielder is trained.
`ApplyClassDefaults()` populates them: the **first** class grants its full list, later
classes only `MulticlassProficiencies` (a fighter taken second brings medium armour, never
heavy). Backgrounds add tools; races add languages.

`Character.AttackWith(item)` turns a weapon into an `AttackProfile` — the bridge to the
dice roller:

- proficiency bonus is added **only when proficient**; the ability modifier always applies
- damage bonus is ability + magic, **never** proficiency, and is added once even on a crit
- finesse picks the better of STR/DEX, ranged uses DEX, reach adds 5 feet
- proficiency matches by weapon **category** *or* by `InventoryItem.Key`, which is how a
  rogue gets rapiers without getting martial weapons

**Two rests, and they do different things.** `ShortRest()` returns warlock pact slots and
`RechargeShortRest` features. `LongRest()` does everything a short rest does plus hit
points, all spell slots, half the hit dice and one level of exhaustion. Hit dice are spent
one at a time via `SpendHitDie(die, rolled)` — the model owns the rules, the caller owns
the randomness.

**Pact slots are stored separately** (`Spells.PactSlots`) because they never merge with
ordinary slots and come back on a *short* rest.

**Penalties that were previously dead data are now applied.** `Speed()` subtracts 10 for
heavy armour worn below its Strength requirement and applies exhaustion (halved at 2, zero
at 5). `SkillRollMode()` returns disadvantage for Stealth in noisy armour and for any check
at exhaustion 1+. `EffectiveHitPointMaximum()` halves at exhaustion 4.

Also on the sheet: `Currency` (coins are fungible — `Spend` makes change, and 50 coins
weigh a pound), `Inspiration`, attunement (`Attune`/`EndAttunement`, max 3), and
encumbrance (`CarriedWeight` counts inventory *and* the purse).

`ValidateSheet()` checks a sheet is legal 5e — subclass timing, subrace, prerequisites,
skills drawn from a granted or class source, total level ≤ 20, no ability score above 20. **`CreateCharacter` runs it;
`UpdateCharacter` deliberately does not**, so sheets predating a rules change stay
editable. `ApplyClassDefaults()` fills in what the tables determine; `ReconcileSpellSlots()`
adds slots on level up without refunding spent ones.

**Derived stats are methods, never fields.** `ProficiencyBonus()`, `SkillModifier()`,
`SavingThrowModifier()`, `ArmorClass()`, `InitiativeModifier()`, `PassivePerception()`,
`SpellSaveDC()`, `SpellAttackModifier()` are all pure functions of stored data. They used
to be stored and drifted the moment a character levelled or changed armor. `CombatStats`
keeps only what nothing can infer: hit points, hit dice, death saves, speed, and explicit
`ArmorClassBonus` / `InitiativeBonus` for magic items and feats.

**Rules encoded in the model** (so the engine cannot get them wrong later):
`AbilityModifier` floors toward negative infinity; `HitPoints.ApplyDamage` spends temporary
hit points first, clamps at 0 and **returns the overflow** so `IsMassiveDamage` can kill
outright; `AddTemporary` takes the higher rather than stacking; `Heal` never restores
temporary hit points; `Combatant.TakeDamage` handles dying/stable/dead and adds death-save
failures for damage taken while down (two on a critical); `ResolveAttack` honours natural
20/1 while `ResolveCheck` deliberately does not — automatic success on ability checks is a
house rule, not RAW.

**Items carry their mechanics.** `InventoryItem` has optional `Weapon` and `Armor` blocks.
Armor class is computed from equipment (`light` adds full DEX, `medium` caps it at +2,
`heavy` ignores it, shield +2), so **never store an AC someone typed in**.

**Spell slots and hit dice are the resource economy.** `Spells.ExpendSlot(level)` takes the
level actually cast at, so upcasting works. `HitDice.RegainOnLongRest` returns half the
total rounded down, minimum one.

**Enums that are still bare strings:** campaign `status`, event types, `Action.ActionType`,
`Relationship.RelationType`, alignment, monster `Type`. Prefer adding constants over
inventing a literal.

## Dice and the rules engine

**All randomness lives in `internal/domain/dice`.** `models` stays a pure function of its
inputs — that split is what lets the rules be tested exactly rather than statistically.
`dice.NewSeeded(n)` replays a sequence exactly; `dice.New()` seeds from the OS. Never call
`math/rand` anywhere else.

`Roller.D20(modifier, mode)` keeps **both** dice under advantage/disadvantage.
`RollDamage(expr, critical)` doubles the dice and adds the modifier once — the most
commonly misplayed part of a critical. `DeathSave` encodes nat 20 = regain 1 HP and
nat 1 = two failures.

**Probability is exact, never sampled** (`probability.go`). `Distribute` convolves one die
at a time to give every total's chance, plus `AtLeast` / `AtMost` — a Monte Carlo estimate
of 3d6 would be wrong in the fourth decimal for no reason. It refuses anything past
`MaxDistributionWork` (5000 dice-faces) rather than hanging a request; the mean and standard
deviation stay available in closed form regardless. `TestDistributionMatchesBruteForce`
checks the convolution against full enumeration, because that code is clever enough to be
wrong quietly.

`faceProbabilities` is the one table both `OddsOfCheck` and `OddsOfAttackWithMode` read, so
the two can never disagree about the same d20. Advantage is not a bonus to the total — it
reshapes which face survives: `P(max=f) = (2f-1)/400`, `P(min=f) = (41-2f)/400`. Attack odds
apply the two rules checks do not (a natural 1 always misses, the crit range always hits)
and report `ExpectedDamage`, which is the number encounter balance actually asks for.

**`internal/domain/rules` is authoritative.** `Engine.SkillCheck`, `AbilityCheck`,
`SavingThrow`, `WeaponAttack` and `MonsterAttack` decide what happened; the attack methods
apply damage to the target, so the result reports what was actually lost after resistance,
not what the dice showed. Character state (exhaustion, noisy armour, Small + Heavy) folds
into the roll mode automatically — callers pass only the *situational* mode.

**`CheckResult.Facts()` / `AttackResult.Facts()` are the contract with the AI.** They are
the complete set of values a narration prompt may reference, every one non-empty even on a
miss. `Summary()` is the sentence the narration must not contradict. **The AI describes
these facts; it never decides them** — see `docs/ARCHITECTURE.md` §3.1.

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
- **Sampling parameters are `*float64`.** `Temperature`, `TopP`, `FrequencyPenalty` and
  `PresencePenalty` are pointers because 0 is meaningful for all four; as plain floats with
  `omitempty`, `temperature: 0` was dropped from the request and the provider silently
  applied its own default. Use `ai.Float(0)`; nil means "provider default".
  `ai.JSONObjectFormat()` requests structured output for intent extraction.
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
`PROJECT_STRUCTURE.md` aspirational tree ·
**`MIGRATION_2026-09-04.md` breaking schema changes — read before pointing the server at
existing data.**

## Working agreements

- **Never run `git commit` or `git push`.** The user owns every commit.
- Secrets live in `.env` (gitignored). `.env.example` documents the shape. Never read
  `.env` values into output or commit them.
- Skills for recurring workflows live in `.claude/skills/`.
