# Game Engine Core

The game engine handles all D&D 5e mechanics, dice rolling, combat resolution, and state management.

---

## 1. Dice System

### Dice Expression Parser

Supports standard D&D dice notation:
- `d20` - single 20-sided die
- `2d6` - two 6-sided dice
- `1d8+3` - one 8-sided die plus modifier
- `4d6dropLowest` - four 6-sided dice, drop lowest
- `2d20kh1` - two 20-sided dice, keep highest
- `3d8dl1` - three 8-sided dice, drop lowest

### Dice Types

```go
package dice

type Die struct {
    Sides    int
    Quantity int
    Modifier int
}

type RollType string

const (
    RollTypeAbilityCheck     RollType = "ability_check"
    RollTypeAttack           RollType = "attack"
    RollTypeDamage           RollType = "damage"
    RollTypeSavingThrow      RollType = "saving_throw"
    RollTypeSkillCheck       RollType = "skill_check"
    RollTypeInitiative       RollType = "initiative"
    RollTypeDeathSave        RollType = "death_save"
    RollTypeRecovery         RollType = "recovery"
)

type DiceExpression struct {
    Raw         string
    Dice        []Die
    Modifier    int
    Advantage   bool
    Disadvantage bool
    KeepHighest int
    KeepLowest  int
    DropLowest  int
    RerollOn    []int  // e.g., reroll 1s
    ExplodeOn   []int  // e.g., explode on 20
}

type RollResult struct {
    RollID          string
    Expression      DiceExpression
    DiceResults     []int
    NaturalRolls    []int
    Modifier        int
    Total           int
    IsCritical      bool
    IsCriticalFail  bool
    Timestamp       time.Time
}
```

### Dice Roller Implementation

```go
package dice

type Roller struct {
    rng *rand.Rand
}

func NewRoller() *Roller {
    return &Roller{
        rng: rand.New(rand.NewSource(time.Now().UnixNano())),
    }
}

func (r *Roller) Roll(expr DiceExpression) *RollResult {
    var diceResults []int
    
    // Handle advantage/disadvantage
    if expr.Advantage || expr.Disadvantage {
        return r.rollWithAdvantage(expr)
    }
    
    // Roll each die
    for _, die := range expr.Dice {
       0; i  for i := < die.Quantity; i++ {
            result := r.rollSingle(die.Sides)
            diceResults = append(diceResults, result)
        }
    }
    
    // Apply modifiers
    total := sum(diceResults) + expr.Modifier
    
    // Apply keep/drop rules
    total = r.applyKeepDrop(diceResults, expr, total)
    
    return &RollResult{
        RollID:       generateUUID(),
        Expression:   expr,
        DiceResults:  diceResults,
        NaturalRolls: diceResults,
        Modifier:     expr.Modifier,
        Total:        total,
        IsCritical:   isNatural20(diceResults) || isCriticalHit(expr, total),
        IsCriticalFail: isNatural1(diceResults),
    }
}

func (r *Roller) rollWithAdvantage(expr DiceExpression) *RollResult {
    // Roll twice, keep best (advantage) or worst (disadvantage)
    first := r.rollStandard(expr)
    second := r.rollStandard(expr)
    
    if expr.Advantage {
        if first.Total >= second.Total {
            first.IsAdvantageRoll = true
            return first
        }
        second.IsAdvantageRoll = true
        return second
    }
    
    if first.Total <= second.Total {
        first.IsAdvantageRoll = true
        return first
    }
    second.IsAdvantageRoll = true
    return second
}
```

### Dice Probability Calculator

```go
package dice

type ProbabilityCalculator struct {
    cache map[string]ProbabilityDistribution
}

type ProbabilityDistribution struct {
    Min      int
    Max      int
    Mean     float64
    Median   int
    StdDev   float64
    Probabilities map[int]float64
}

func (p *ProbabilityCalculator) CalculateDistribution(expr DiceExpression) *ProbabilityDistribution {
    // Use convolution for multiple dice
    // For complex expressions, use Monte Carlo simulation
    
    switch {
    case expr.Dice[0].Quantity == 1 && expr.Dice[0].Sides == 20:
        return p.calculateD20Distribution(expr.Modifier)
    case len(expr.Dice) == 1:
        return p.calculateSingleDieDistribution(expr)
    default:
        return p.calculateMultipleDiceDistribution(expr)
    }
}
```

---

## 2. Rules Engine

### Core Rules Structure

```go
package rules

type RulesEngine struct {
    version      string  // 5e, homebrew variants
    settings     RulesSettings
    spellManager *SpellManager
    classManager *ClassManager
    racialTraits map[string][]Feature
}

type RulesSettings struct {
    CriticalHitRule     CriticalHitRule
    CriticalFailRule    CriticalFailRule
    FallingDamageRule   FallingDamageRule
    HealingSurgeRule    HealingSurgeRule
    RestVariant         RestVariant
    GrittyRealism       bool
    AllowHomebrew       bool
}

type CriticalHitRule string

const (
    CriticalHitMaxDamage   CriticalHitRule = "max_damage"
    CriticalHitDoubleDice  CriticalHitRule = "double_dice"
    CriticalHitExtraDamage CriticalHitRule = "extra_damage"
)
```

### Ability Checks

```go
type AbilityCheck struct {
    Ability        Ability
    Skill          *Skill
    Proficiency    Proficiency
    Circumstances  CheckCircumstances
    DC             int
}

type CheckCircumstances struct {
    Advantage bool
    Disadvantage bool
    HalfCover bool
    ThreeQuarterCover bool
    FriendlyTerrain bool
    UnfavorableConditions bool
}

func (e *RulesEngine) ResolveAbilityCheck(
    character *Character,
    check *AbilityCheck,
) *CheckResult {
    
    // Calculate modifier
    modifier := character.GetAbilityModifier(check.Ability)
    
    // Apply proficiency
    if check.Skill != nil && character.HasSkillProficiency(check.Skill) {
        modifier += character.ProficiencyBonus
    }
    
    // Roll dice
    roller := dice.NewRoller()
    var expr dice.DiceExpression
    
    if check.Circumstances.Advantage {
        expr.Advantage = true
    } else if check.Circumstances.Disadvantage {
        expr.Disadvantage = true
    } else {
        expr.Dice = []dice.Die{{Sides: 20, Quantity: 1}}
    }
    
    roll := roller.Roll(expr)
    
    // Apply cover
    modifier = e.applyCover(modifier, check.Circumstances)
    
    total := roll.Total + modifier
    
    // Determine success/failure
    success := total >= check.DC
    
    return &CheckResult{
        Roll:          roll,
        Modifier:      modifier,
        Total:         total,
        DC:            check.DC,
        Success:       success,
        CriticalHit:   roll.IsCritical,
        CriticalFail:  roll.IsCriticalFail,
    }
}
```

### Combat Rules

```go
type CombatEngine struct {
    rules        *RulesEngine
    initiative   *InitiativeTracker
    turnManager  *TurnManager
}

type AttackRoll struct {
    Attacker        *Character
    Weapon          *Weapon
    AttackType      AttackType
    Target          *Character
    Proficiency     bool
    MagicBonus      int
    Circumstances   AttackCircumstances
}

type AttackCircumstances struct {
    Advantage           bool
    Disadvantage        bool
    Flanking            bool
    Hidden              bool
    Charging            bool
    Diving              bool
    AttackingFromAbove  bool
    DifficultTerrain    bool
    TargetProne         bool
    AttackerProne       bool
}

func (e *CombatEngine) ResolveAttack(attack *AttackRoll) *AttackResult {
    
    // Calculate attack modifier
    modifier := attack.Attacker.GetAttackModifier(attack.Weapon)
    
    if attack.Proficiency {
        modifier += attack.Attacker.ProficiencyBonus
    }
    
    modifier += attack.MagicBonus
    
    // Roll attack
    roller := dice.NewRoller()
    var expr dice.DiceExpression
    
    if attack.Circumstances.Advantage {
        expr.Advantage = true
    } else if attack.Circumstances.Disadvantage {
        expr.Disadvantage = true
    } else {
        expr.Dice = []dice.Die{{Sides: 20, Quantity: 1}}
    }
    
    attackRoll := roller.Roll(expr)
    total := attackRoll.Total + modifier
    
    // Check for critical
    criticalHit := false
    if attackRoll.IsCritical {
        criticalHit = e.checkCriticalHit(attack)
    }
    
    // Check defense
    targetAC := attack.Target.GetArmorClass()
    
    // Determine hit/miss
    hit := attackRoll.IsCritical || (!attack.Circumstances.Disadvantage && total >= targetAC)
    
    // Calculate damage
    var damage *DamageResult
    if hit || (attack.Circumstances.Advantage && attackRoll.NaturalRolls[0] >= targetAC) {
        damage = e.calculateDamage(attack, criticalHit)
    }
    
    return &AttackResult{
        AttackRoll:    attackRoll,
        TotalAttack:   total,
        TargetAC:      targetAC,
        Hit:           hit,
        Critical:     criticalHit,
        Damage:        damage,
    }
}

func (e *CombatEngine) calculateDamage(attack *AttackRoll, isCritical bool) *DamageResult {
    weapon := attack.Weapon
    baseDamage := weapon.Damage
    
    // Double damage on critical (per standard rules)
    if isCritical {
        baseDamage = e.applyCriticalHitRules(weapon)
    }
    
    // Add ability modifier
    abilityMod := attack.Attacker.GetAbilityModifier(weapon.DamageAbility)
    
    // Add magic bonus
    magicBonus := attack.MagicBonus
    
    totalDamage := baseDamage + abilityMod + magicBonus
    
    return &DamageResult{
        Amount:      totalDamage,
        Type:        weapon.DamageType,
        BaseDamage:  baseDamage,
        AbilityMod:  abilityMod,
        MagicBonus:  magicBonus,
        IsCritical:  isCritical,
    }
}
```

### Spell Casting

```go
type SpellResolver struct {
    spellLibrary *SpellLibrary
    concentration *ConcentrationTracker
}

type Spell struct {
    Name           string
    Level          int
    School         School
    CastingTime    string
    Range          string
    Components     []Component
    Duration       Duration
    AttackType     AttackType
    SaveAbility    Ability
    Damage         *SpellDamage
    Effects        []SpellEffect
}

type SpellEffect string

const (
    SpellEffectDamage      SpellEffect = "damage"
    SpellEffectHealing     SpellEffect = "healing"
    SpellEffectBuff        SpellEffect = "buff"
    SpellEffectDebuff      SpellEffect = "debuff"
    SpellEffectSummon      SpellEffect = "summon"
    SpellEffectTeleport    SpellEffect = "teleport"
    SpellEffectControl     SpellEffect = "control"
    SpellEffectArea        SpellEffect = "area_of_effect"
)

func (r *SpellResolver) ResolveSpell(
    caster *Character,
    spell *Spell,
    target *Target,
    context SpellContext,
) *SpellResult {
    
    // Check spell slots
    if !caster.HasSpellSlot(spell.Level) {
        return &SpellResult{
            Success: false,
            Error:   "No spell slots available",
        }
    }
    
    // Consume spell slot
    caster.UseSpellSlot(spell.Level)
    
    // Handle concentration spells
    if spell.Duration.Concentration {
        r.concentration.EndConcentration(caster)
    }
    
    // Roll attack if needed
    var attackResult *AttackResult
    if spell.AttackType == AttackTypeRanged || spell.AttackType == AttackTypeMelee {
        attack := &AttackRoll{
            Attacker:    caster,
            AttackType:  spell.AttackType,
            Proficiency: true,
        }
        attackResult = resolveAttack(attack)
    }
    
    // Apply effects
    effects := r.applySpellEffects(spell, target, context)
    
    return &SpellResult{
        Success:       true,
        Spell:         spell,
        AttackResult:  attackResult,
        Effects:       effects,
        SlotRemaining: caster.GetRemainingSlots(spell.Level),
    }
}
```

---

## 3. State Management

### Campaign State

```go
type CampaignState struct {
    ID              string
    CurrentSession  *SessionState
    Characters      map[string]*CharacterState
    NPCs            map[string]*NPCState
    StoryProgress   *StoryState
    CombatState     *CombatState
    WorldState      *WorldState
    EventHistory    []GameEvent
    Metadata        CampaignMetadata
}

type SessionState struct {
    ID              string
    Status          SessionStatus
    StartTime       time.Time
    Location        Location
    TimeOfDay       TimeOfDay
    ActiveEffects   []ActiveEffect
    Participants    []Participant
}

type CharacterState struct {
    ID              string
    HP              HitPoints
    Conditions      []Condition
    ExhaustionLevel int
    Initiative      *InitiativeScore
    TempEffects     []TemporaryEffect
    Resources       map[string]int  // e.g., "ki": 4, "sneak_dice": 3
}
```

### State Transitions

```go
type StateManager struct {
    store         StateStore
    eventBus      *EventBus
    validator     *StateValidator
    history       *ChangeHistory
}

type StateTransition struct {
    FromState     interface{}
    ToState       interface{}
    Trigger       Event
    Conditions    []TransitionCondition
    SideEffects   []SideEffect
}

func (m *StateManager) ApplyTransition(
    entityID string,
    transition StateTransition,
) (*StateChange, error) {
    
    // Get current state
    current, err := m.store.Get(entityID)
    if err != nil {
        return nil, err
    }
    
    // Validate transition
    valid, err := m.validator.Validate(transition, current)
    if !valid {
        return nil, err
    }
    
    // Apply side effects
    for _, effect := range transition.SideEffects {
        if err := effect.Execute(current); err != nil {
            return nil, err
        }
    }
    
    // Calculate new state
    newState := transition.ToState
    
    // Save state
    if err := m.store.Save(entityID, newState); err != nil {
        return nil, err
    }
    
    // Record history
    m.history.Record(&StateChange{
        EntityID:   entityID,
        FromState:  current,
        ToState:    newState,
        Transition: transition,
        Timestamp:  time.Now(),
    })
    
    // Publish event
    m.eventBus.Publish(StateChangedEvent{
        EntityID: entityID,
        Change:   newState,
    })
    
    return &StateChange{
        FromState: current,
        ToState:   newState,
    }, nil
}
```

### Combat State Machine

```go
type CombatStateMachine struct {
    currentState  CombatPhase
    combat        *Combat
    rules         *RulesEngine
}

type CombatPhase string

const (
    CombatPhaseNotStarted CombatPhase = "not_started"
    CombatPhaseInitiative  CombatPhase = "initiative"
    CombatPhaseCombat      CombatPhase = "combat"
    CombatPhaseVictory     CombatPhase = "victory"
    CombatPhaseDefeat      CombatPhase = "defeat"
    CombatPhaseDraw        CombatPhase = "draw"
)

func (m *CombatStateMachine) Transition(phase CombatPhase) error {
    switch phase {
    case CombatPhaseInitiative:
        return m.startInitiative()
    case CombatPhaseCombat:
        return m.startCombat()
    case CombatPhaseVictory:
        return m.resolveVictory()
    case CombatPhaseDefeat:
        return m.resolveDefeat()
    }
    return nil
}

func (m *CombatStateMachine) startInitiative() error {
    // Roll initiative for all combatants
    for _, combatant := range m.combat.Combatants {
        roller := dice.NewRoller()
        roll := roller.Roll(dice.DiceExpression{
            Dice:     []dice.Die{{Sides: 20, Quantity: 1}},
        })
        
        initiative := &InitiativeScore{
            Roll:    roll.Total,
            Value:   roll.Total + combatant.GetAbilityModifier("dexterity"),
            TieBreaker: combatant.Dexterity,
        }
        
        combatant.Initiative = initiative
    }
    
    // Sort by initiative
    m.combat.SortByInitiative()
    
    m.currentState = CombatPhaseInitiative
    return nil
}
```

### Event Sourcing (OUT OF SCOPE)

> **Cut on 2026-09-04.** Replay, snapshots and undo impose a cost on every write path to
> buy a feature nobody has asked for; see ARCHITECTURE.md §0. Append to `story_events`
> ordered by `sequence_number` instead — that already gives the AI its history and the
> player a readable log. The design below is kept in case undo becomes a real
> requirement.

```go
type EventStore struct {
    events    []GameEvent
    snapshots map[string]*Snapshot
}

type GameEvent struct {
    EventID     string
    EventType   EventType
    EntityID    string
    Payload     interface{}
    Timestamp   time.Time
    SequenceNum int64
    Metadata    EventMetadata
}

const (
    EventTypeCharacterCreated    EventType = "character.created"
    EventTypeCharacterUpdated    EventType = "character.updated"
    EventTypeHPChanged            EventType = "character.hp_changed"
    EventTypeCombatStarted        EventType = "combat.started"
    EventTypeCombatantAdded      EventType = "combat.combatant_added"
    EventTypeTurnTaken           EventType = "combat.turn_taken"
    EventTypeSpellCast           EventType = "spell.cast"
    EventTypeItemUsed            EventType = "item.used"
)

func (s *EventStore) Append(event GameEvent) error {
    event.SequenceNum = s.nextSequence(event.EntityID)
    event.Timestamp = time.Now()
    
    s.events = append(s.events, event)
    
    // Create snapshot every N events
    if len(s.events)%100 == 0 {
        s.createSnapshot(event.EntityID)
    }
    
    return nil
}

func (s *EventStore) ReplayEvents(entityID string, fromSeq int64) (interface{}, error) {
    var state interface{}
    
    // Load from snapshot if available
    if snapshot := s.getLatestSnapshot(entityID); snapshot != nil {
        state = snapshot.State
    }
    
    // Replay events
    for _, event := range s.events {
        if event.EntityID != entityID {
            continue
        }
        if event.SequenceNum <= fromSeq {
            continue
        }
        
        state = applyEvent(state, event)
    }
    
    return state, nil
}
```

---

## 4. Character Management

### Character Builder

```go
type CharacterBuilder struct {
    character *Character
}

func NewCharacterBuilder() *CharacterBuilder {
    return &CharacterBuilder{
        character: &Character{
            ID:        generateUUID(),
            CreatedAt: time.Now(),
        },
    }
}

func (b *CharacterBuilder) SetBasicInfo(race, class, background string) *CharacterBuilder {
    b.character.BasicInfo.Race = race
    b.character.BasicInfo.Class = class
    b.character.BasicInfo.Background = background
    return b
}

func (b *CharacterBuilder) SetAbilityScores(scores AbilityScores) *CharacterBuilder {
    b.character.AbilityScores = scores
    b.character.DerivedStats = CalculateDerivedStats(scores)
    return b
}

func (b *CharacterBuilder) AddClassFeatures(features []Feature) *CharacterBuilder {
    b.character.ClassFeatures = append(b.character.ClassFeatures, features...)
    return b
}

func (b *CharacterBuilder) AddRacialTraits(traits []Feature) *CharacterBuilder {
    b.character.RacialTraits = append(b.character.RacialTraits, traits...)
    return b
}

func (b *CharacterBuilder) SetEquipment(equipment []Item) *CharacterBuilder {
    b.character.Inventory = equipment
    b.character.Equipment = ExtractEquipment(equipment)
    return b
}

func (b *CharacterBuilder) Build() (*Character, error) {
    // Validate character
    if err := b.validate(); err != nil {
        return nil, err
    }
    
    // Calculate final stats
    b.character.MaxHP = b.calculateMaxHP()
    b.character.CurrentHP = b.character.MaxHP
    b.character.ArmorClass = b.calculateArmorClass()
    
    return b.character, nil
}
```

### Hit Points Calculator

```go
func (c *Character) calculateMaxHP() int {
    constitutionMod := c.GetAbilityModifier("constitution")
    
    // First level: max die + constitution
    firstLevelHP := c.BasicInfo.Class.HitDie + constitutionMod
    
    // Subsequent levels: average (round up) + constitution
    levels := c.BasicInfo.Level - 1
    subsequentHP := levels * ((c.BasicInfo.Class.HitDie / 2) + 1 + constitutionMod)
    
    // Add any bonus HP from feats or features
    bonusHP := c.calculateBonusHP()
    
    return firstLevelHP + subsequentHP + bonusHP
}
```

---

## 5. Combat Tracker

### Initiative Tracker

```go
type InitiativeTracker struct {
    combatants []*Combatant
    round      int
    turnIndex  int
}

type Combatant struct {
    CharacterID    string
    Name           string
    Initiative     int
    InitiativeRoll int
    Dexterity      int
    IsPlayer       bool
    IsActive       bool
}

func (t *InitiativeTracker) Sort() {
    sort.Slice(t.combatants, func(i, j int) bool {
        if t.combatants[i].InitiativeRoll != t.combatants[j].InitiativeRoll {
            return t.combatants[i].InitiativeRoll > t.combatants[j].InitiativeRoll
        }
        return t.combatants[i].Dexterity > t.combatants[j].Dexterity
    })
}

func (t *InitiativeTracker) NextTurn() *Combatant {
    if t.turnIndex >= len(t.combatants) {
        t.round++
        t.turnIndex = 0
    }
    
    combatant := t.combatants[t.turnIndex]
    t.turnIndex++
    
    return combatant
}
```

### Turn Actions

```go
type TurnActions struct {
    Actions    int     // 1 per turn
    BonusActions int   // 1 per turn
    Movement   int     // feet
    Reactions  int     // 1 per round
}

func (c *CombatEngine) ExecuteTurn(
    combatantID string,
    actions TurnActions,
) (*TurnResult, error) {
    
    combatant, err := c.getCombatant(combatantID)
    if err != nil {
        return nil, err
    }
    
    // Check if turn is valid
    if !combatant.IsActive {
        return nil, errors.New("combatant is not active")
    }
    
    // Track actions taken
    actionTracker := &ActionTracker{
        CombatantID: combatantID,
        Round:       c.currentRound,
    }
    
    // Execute action
    if action, ok := actions.MainAction.(*AttackAction); ok {
        result := c.executeAttack(combatant, action)
        actionTracker.AddAction(result)
    }
    
    return &TurnResult{
        Combatant: combatant,
        Actions:   actionTracker,
        Duration: 6 * time.Second, // ~1 minute per turn
    }, nil
}
```
