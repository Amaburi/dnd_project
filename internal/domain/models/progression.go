package models

// xpThresholds is the experience needed to reach each character level.
var xpThresholds = [21]int{
	0,      // index 0 unused
	0,      // 1
	300,    // 2
	900,    // 3
	2700,   // 4
	6500,   // 5
	14000,  // 6
	23000,  // 7
	34000,  // 8
	48000,  // 9
	64000,  // 10
	85000,  // 11
	100000, // 12
	120000, // 13
	140000, // 14
	165000, // 15
	195000, // 16
	225000, // 17
	265000, // 18
	305000, // 19
	355000, // 20
}

// XPForLevel returns the experience required to reach a character level.
func XPForLevel(level int) int {
	if level < 1 {
		return 0
	}
	if level > 20 {
		level = 20
	}
	return xpThresholds[level]
}

// LevelForXP returns the character level an experience total has earned.
func LevelForXP(xp int) int {
	level := 1
	for l := 20; l >= 1; l-- {
		if xp >= xpThresholds[l] {
			level = l
			break
		}
	}
	return level
}

// XPToNextLevel returns how much more experience is needed to level, and
// whether there is a next level at all.
func XPToNextLevel(xp int) (int, bool) {
	level := LevelForXP(xp)
	if level >= 20 {
		return 0, false
	}
	return xpThresholds[level+1] - xp, true
}

// ExhaustionEffects are the penalties that apply at a level of exhaustion.
//
// Exhaustion is cumulative: each level adds its effect to every level below,
// which is why these are reported as a set of flags rather than a single
// penalty.
type ExhaustionEffects struct {
	DisadvantageOnAbilityChecks   bool // level 1+
	SpeedHalved                   bool // level 2+
	DisadvantageOnAttacksAndSaves bool // level 3+
	HitPointMaximumHalved         bool // level 4+
	SpeedZero                     bool // level 5+
	Dead                          bool // level 6
}

// ExhaustionEffectsFor returns the penalties at a level of exhaustion.
func ExhaustionEffectsFor(level int) ExhaustionEffects {
	return ExhaustionEffects{
		DisadvantageOnAbilityChecks:   level >= 1,
		SpeedHalved:                   level >= 2,
		DisadvantageOnAttacksAndSaves: level >= 3,
		HitPointMaximumHalved:         level >= 4,
		SpeedZero:                     level >= 5,
		Dead:                          level >= MaxExhaustion,
	}
}

// AbilityScoreImprovementLevels returns the levels at which a class grants an
// Ability Score Improvement (or a feat in its place).
//
// Most classes improve at 4, 8, 12, 16 and 19; fighters gain two extra and
// rogues one, which is a real difference in how quickly they reach a 20 in
// their primary ability.
func AbilityScoreImprovementLevels(c Class) []int {
	base := []int{4, 8, 12, 16, 19}
	switch c {
	case ClassFighter:
		return []int{4, 6, 8, 12, 14, 16, 19}
	case ClassRogue:
		return []int{4, 8, 10, 12, 16, 19}
	default:
		return base
	}
}

// MaxAbilityScore is the ceiling ordinary advancement can reach. Magic items
// and a few epic effects raise it, which is why validation warns rather than
// clamping.
const MaxAbilityScore = 20
