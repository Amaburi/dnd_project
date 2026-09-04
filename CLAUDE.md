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
| 5e rules vocabulary + derived stats | Implemented in `models`, unit-tested |
| Classes, subclasses, races, backgrounds, multiclassing | Implemented as tables in `models`, unit-tested |
| Monster statblocks | **Model only** — no repository, no routes |
| Session / StoryEvent / CombatEncounter | **Models only** — no repository, no routes |
| Dice roller, rules engine, combat tracker | **Spec only** (`docs/GAME_ENGINE.md`) — no code |
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
go test ./...   # 140 tests across models, handlers, ai, config, mongodb
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
internal/domain/models/       entities (Character, Campaign, Session, Monster) plus the
                              typed 5e vocabulary (abilities, skills, conditions, items,
                              spells, dice) and the rules encoded as methods
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
`PROJECT_STRUCTURE.md` aspirational tree ·
**`MIGRATION_2026-09-04.md` breaking schema changes — read before pointing the server at
existing data.**

## Working agreements

- **Never run `git commit` or `git push`.** The user owns every commit.
- Secrets live in `.env` (gitignored). `.env.example` documents the shape. Never read
  `.env` values into output or commit them.
- Skills for recurring workflows live in `.claude/skills/`.
