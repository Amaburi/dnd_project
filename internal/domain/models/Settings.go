package models

// Setting represents the campaign world setting
type Setting struct {
	WorldName    string        `json:"world_name" bson:"world_name"`
	Era          string        `json:"era" bson:"era"`
	MagicLevel   string        `json:"magic_level" bson:"magic_level"`
	TechLevel    string        `json:"technology_level" bson:"technology_level"`
	KeyLocations []KeyLocation `json:"key_locations" bson:"key_locations"`
	Factions     []Faction     `json:"factions" bson:"factions"`
}

// KeyLocation represents an important location in the world
type KeyLocation struct {
	Name        string      `json:"name" bson:"name"`
	Description string      `json:"description" bson:"description"`
	Coordinates Coordinates `json:"coordinates" bson:"coordinates"`
}

// Faction represents a faction in the world
type Faction struct {
	Name        string `json:"name" bson:"name"`
	Description string `json:"description" bson:"description"`
	Alignment   string `json:"alignment" bson:"alignment"`
}
