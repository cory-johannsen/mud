package importer

// ZoneData is the common intermediate format produced by all Source
// implementations. Its YAML tags match the project's zone file schema exactly,
// so it can be marshalled directly and validated by world.LoadZoneFromBytes.
type ZoneData struct {
	Zone ZoneSpec `yaml:"zone"`
}

// ZoneSpec holds zone-level metadata and its rooms.
type ZoneSpec struct {
	ID          string     `yaml:"id"`
	Name        string     `yaml:"name"`
	Description string     `yaml:"description"`
	StartRoom   string     `yaml:"start_room"`
	Rooms       []RoomSpec `yaml:"rooms"`
}

// RoomSpec holds a single room's data.
type RoomSpec struct {
	ID          string            `yaml:"id"`
	Title       string            `yaml:"title"`
	Description string            `yaml:"description"`
	Exits       []ExitSpec        `yaml:"exits,omitempty"`
	Properties  map[string]string `yaml:"properties,omitempty"`
}

// ExitSpec holds a single exit's data.
type ExitSpec struct {
	Direction string `yaml:"direction"`
	Target    string `yaml:"target"`
	Locked    bool   `yaml:"locked,omitempty"`
	Hidden    bool   `yaml:"hidden,omitempty"`
}

// Source loads content from a format-specific source directory and produces
// ZoneData ready to be written as zone YAML files.
//
// Precondition: sourceDir must exist and contain the expected layout for the format.
// startRoom is an optional display-name override for the zone's start room;
// empty string means "use format default".
// Postcondition: returns at least one ZoneData, or a non-nil error.
type Source interface {
	Load(sourceDir, startRoom string) ([]*ZoneData, error)
}
