# Schema migration — 2026-09-04

Two rounds of changes on branch `satu` altered stored document shapes. Nothing here runs
automatically; apply what applies to your data before starting the server against it.

If your database only holds throwaway development data, **dropping the `characters`
collection is faster and safer than migrating it.**

---

## 1. Characters: `campaign_id` held the wrong identifier

Character handlers used to write the campaign's **`_id` hex** into `campaign_id`, while
`campaign_id` everywhere else means the campaign's *business* ID. Queries now use the
business ID, so pre-existing characters will not be found.

```js
// For each campaign, repoint any character linked by its _id hex.
db.campaigns.find({}, { _id: 1, campaign_id: 1 }).forEach(c => {
  db.characters.updateMany(
    { campaign_id: c._id.toString() },
    { $set: { campaign_id: c.campaign_id } }
  );
});
```

Run this **first** — the later steps assume characters are attached to the right campaign.

## 2. Timestamps changed type

`Character.created_at` / `updated_at` moved from `primitive.DateTime` to `time.Time`.
BSON stores both as a date, so **no data migration is needed** — but JSON responses changed
from a millisecond number to an RFC 3339 string. Any client parsing that field breaks.

## 3. `derived_stats` → `combat_stats`, and most of it is gone

Armor class, proficiency bonus, initiative modifier and passive perception are now computed
from ability scores, level and equipment. Only hit points, hit dice, death saves, speed and
explicit magic/feat bonuses are stored.

```js
db.characters.updateMany({}, [
  { $set: {
      combat_stats: {
        hit_points: { $ifNull: ["$derived_stats.hit_points",
                                { current: 0, maximum: 0, temporary: 0 }] },
        hit_dice:   { die: 8, total: { $ifNull: ["$basic_info.level", 1] }, spent: 0 },
        death_saves: { successes: 0, failures: 0 },
        speed: { $ifNull: ["$derived_stats.speed", 30] },
        armor_class_bonus: 0,
        initiative_bonus: 0
      }
  }},
  { $unset: "derived_stats" }
]);
```

**Check the hit dice `die` per character** — the migration guesses d8. It is d6 for
sorcerers and wizards, d10 for fighters, paladins and rangers, d12 for barbarians.

Armor class is no longer stored. If a character's AC came from armor, give the equipped
item an `armor` block (`category`, `base_ac`) so it can be computed; otherwise put the
difference in `combat_stats.armor_class_bonus`.

## 3b. `basic_info` is now class/race/background typed, and multiclass-aware

`class` and `level` became a `classes` array so Fighter 3 / Wizard 2 is representable,
and race, subrace and background became closed sets. Values must match the constants
(lower_snake_case: `half_elf`, `folk_hero`, `eldritch_knight`).

```js
db.characters.updateMany({ "basic_info.classes": { $exists: false } }, [
  { $set: {
      "basic_info.classes": [{
        class:    { $toLower: "$basic_info.class" },
        subclass: "",
        level:    { $ifNull: ["$basic_info.level", 1] }
      }],
      "basic_info.race":       { $toLower: "$basic_info.race" },
      "basic_info.background": { $toLower: "$basic_info.background" }
  }},
  { $unset: ["basic_info.class", "basic_info.level"] }
]);
```

Then fix by hand what a script cannot infer:

- **Subclass.** Every character at or above their class's subclass level needs one
  (`3` for most, `2` for druid and wizard, `1` for cleric, sorcerer and warlock).
  `ValidateSheet` rejects a character missing it.
- **Subrace.** Dwarf, elf, halfling and gnome require one; the others must leave it empty.
- **Multi-word names** need underscores: `"Half Elf"` → `half_elf`, `"Folk Hero"` →
  `folk_hero`.
- **Hit dice** are rebuilt from class by `Character.ExpectedHitDice()`, so the d8 guess in
  step 3 is corrected once `basic_info.classes` is right. Call `ApplyClassDefaults()` or
  re-save through the API.

`CreateCharacter` now runs `ValidateSheet` — new characters must be legal 5e. Updates
deliberately do not, so legacy sheets stay editable.

## 4. Skills and saving throws are no longer booleans

Proficiency is four-valued (`""`, `"half"`, `"proficient"`, `"expertise"`) and the fields
are now maps keyed by skill/ability name.

```js
db.characters.find({ skills: { $type: "object" } }).forEach(ch => {
  const skills = {}, saves = {};
  for (const [k, v] of Object.entries(ch.skills || {})) if (v === true) skills[k] = "proficient";
  for (const [k, v] of Object.entries(ch.saving_throws || {})) if (v === true) saves[k] = "proficient";
  db.characters.updateOne({ _id: ch._id },
    { $set: { skills: skills, saving_throws: saves } });
});
```

Old keys were camelCase-derived (`animal_handling`, `sleight_of_hand`) and already match
the new constants. **Re-flag expertise by hand** — a boolean could not record it, so no
migration can recover it.

## 5. `status_effects` is gone; `conditions` is a closed set

The two overlapping lists became one. Conditions must be one of the fourteen 5e values;
exhaustion moved to its own 0–6 field.

```js
db.characters.updateMany({}, {
  $unset: { status_effects: "" },
  $set:   { exhaustion: 0 }
});
```

Anything in `status_effects` that was not a real condition was narrative colour — move it
into a description. `AddCondition` now rejects unknown values.

## 6. Hostile creatures move to `monsters`

`CharacterType` is `player` or `npc` only. Characters with type `enemy` or `monster` are
no longer valid and will not deserialise into a usable statblock.

```js
db.characters.find({ type: { $in: ["enemy", "monster"] } }).count();
```

There is no automatic conversion: a statblock needs a challenge rating, damage affinities
and actions that the character schema never held. Re-enter them in `monsters`, then delete
the originals. The `monsters` collection and its indexes are created at startup.

## 7. Spells restructured

`spells_known` was `map[string][]string` keyed by `"1st"`, `"2nd"`. It is now `known` — an
array of `{name, level}` — plus `cantrips`, `prepared`, and **`slots`**, which did not
exist before. Without slots a caster has no resource limit at all, so slots must be filled
in per character from the class table.

## 8. Combat encounters

`Combatant` changed shape: `current_hp`/`max_hp`/`temporary_hp` became a nested
`hit_points`; `character_id` became `source_id` alongside a `source_type`; `status` gained
`dying` and `stable`; action counters became booleans. Encounters are transient, so
**dropping `combat_encounters` is the practical fix.**
