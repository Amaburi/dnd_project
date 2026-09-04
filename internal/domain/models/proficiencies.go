package models

// Armour and weapon proficiency categories. Weapons may also be named
// individually, which is how classes grant a rapier without granting every
// martial weapon.
const (
	ProfLightArmor  = "light"
	ProfMediumArmor = "medium"
	ProfHeavyArmor  = "heavy"
	ProfShields     = "shields"

	ProfSimpleWeapons  = "simple"
	ProfMartialWeapons = "martial"
)

// Proficiencies are the trained categories a character carries on their sheet.
//
// Class and background tables list these, but until they are copied onto the
// character nothing can answer "is this character proficient with a longsword?"
// -- and without that, an attack bonus cannot be computed.
type Proficiencies struct {
	Armor     []string `json:"armor" bson:"armor"`
	Weapons   []string `json:"weapons" bson:"weapons"`
	Tools     []string `json:"tools" bson:"tools"`
	Languages []string `json:"languages" bson:"languages"`
}

func contains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}

// HasArmor reports proficiency with an armour category.
func (p Proficiencies) HasArmor(category ArmorCategory) bool {
	return contains(p.Armor, string(category))
}

// HasShield reports proficiency with shields.
func (p Proficiencies) HasShield() bool {
	return contains(p.Armor, ProfShields)
}

// HasTool reports proficiency with a tool.
func (p Proficiencies) HasTool(tool string) bool {
	return contains(p.Tools, tool)
}

// SpeaksLanguage reports whether the character knows a language.
func (p Proficiencies) SpeaksLanguage(language string) bool {
	return contains(p.Languages, language)
}

// HasWeapon reports proficiency with a specific weapon.
//
// A weapon is covered either by its category -- "simple" or "martial" -- or by
// being named outright, which is how a rogue is proficient with rapiers and
// shortswords without being proficient with martial weapons generally.
func (p Proficiencies) HasWeapon(item InventoryItem) bool {
	if item.Weapon != nil && contains(p.Weapons, string(item.Weapon.Category)) {
		return true
	}
	return item.Key != "" && contains(p.Weapons, item.Key)
}

// add appends values that are not already present, keeping the first order.
func addUnique(dst []string, values ...string) []string {
	for _, v := range values {
		if v != "" && !contains(dst, v) {
			dst = append(dst, v)
		}
	}
	return dst
}

// Merge folds another set of proficiencies in, skipping duplicates.
func (p *Proficiencies) Merge(other Proficiencies) {
	p.Armor = addUnique(p.Armor, other.Armor...)
	p.Weapons = addUnique(p.Weapons, other.Weapons...)
	p.Tools = addUnique(p.Tools, other.Tools...)
	p.Languages = addUnique(p.Languages, other.Languages...)
}

// IsArmorProficient reports whether the character is trained in everything
// they are currently wearing.
//
// Wearing armour without proficiency imposes disadvantage on every Strength
// and Dexterity check, save and attack, and prevents spellcasting entirely --
// which makes this worth checking rather than assuming.
func (c *Character) IsArmorProficient() bool {
	if c.Equipment.Armor != nil && c.Equipment.Armor.Armor != nil {
		if !c.Proficiencies.HasArmor(c.Equipment.Armor.Armor.Category) {
			return false
		}
	}
	if c.Equipment.Shield != nil && !c.Proficiencies.HasShield() {
		return false
	}
	return true
}

// ClassProficiencies returns what a class grants, honouring the reduced set
// that applies when the class is taken as a later multiclass rather than at
// first level.
func ClassProficiencies(c Class, first bool) Proficiencies {
	def, ok := ClassTable[c]
	if !ok {
		return Proficiencies{}
	}
	if first {
		return Proficiencies{
			Armor:   append([]string(nil), def.ArmorProficiencies...),
			Weapons: append([]string(nil), def.WeaponProficiencies...),
		}
	}
	return Proficiencies{
		Armor:   append([]string(nil), def.MulticlassProficiencies.Armor...),
		Weapons: append([]string(nil), def.MulticlassProficiencies.Weapons...),
		Tools:   append([]string(nil), def.MulticlassProficiencies.Tools...),
	}
}
