package gomud

// GomudZone is the parsed form of a gomud assets/zones/<name>.yaml file.
// Rooms and Areas are lists of display names, not IDs.
type GomudZone struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Rooms       []string `yaml:"rooms"`
	Areas       []string `yaml:"areas"`
}

// GomudArea is the parsed form of a gomud assets/areas/<name>.yaml file.
// Rooms is a list of display names.
type GomudArea struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Rooms       []string `yaml:"rooms"`
}

// GomudRoom is the parsed form of a gomud assets/rooms/<name>.yaml file.
// The objects field is intentionally omitted (not supported in this project).
type GomudRoom struct {
	Name        string               `yaml:"name"`
	Description string               `yaml:"description"`
	Exits       map[string]GomudExit `yaml:"exits"`
}

// GomudExit is one exit entry in a GomudRoom.Exits map.
// Direction is the capitalized compass direction (e.g. "North", "Southwest").
// Target is the display name of the target room.
type GomudExit struct {
	Direction string `yaml:"direction"`
	Name      string `yaml:"name"`
	Target    string `yaml:"target"`
}
