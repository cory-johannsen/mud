// internal/client/render/render.go
package render

import "time"

// ColorToken identifies the semantic color category for a feed entry or UI element.
// Each client maps tokens to its native representation (CSS class, color.RGBA, ANSI escape).
type ColorToken int

const (
	ColorDefault    ColorToken = iota
	ColorCombat              // CombatEvent, RoundStartEvent, RoundEndEvent
	ColorSpeech              // MessageEvent (say/emote)
	ColorRoomEvent           // arrival/departure events
	ColorSystem              // system messages and unclassified events
	ColorError               // ErrorEvent
	ColorStructured          // CharacterInfo, InventoryView, CharacterSheetView
)

// FeedEntry is a single message in the feed panel.
// It mirrors feed.Entry without creating an import cycle.
type FeedEntry struct {
	Timestamp time.Time
	Token     ColorToken
	Text      string // pre-extracted narrative text; clients render this directly
}

// CharacterSnapshot is a point-in-time view of the character panel state.
// It mirrors session.CharacterState without creating an import cycle.
type CharacterSnapshot struct {
	Name       string
	Level      int
	CurrentHP  int
	MaxHP      int
	Conditions []string
	HeroPoints int
	AP         int
}

// FeedRenderer renders a slice of feed entries into the client's native output.
type FeedRenderer interface {
	RenderFeed(entries []FeedEntry) error
}

// CharacterRenderer renders the character panel state into the client's native output.
type CharacterRenderer interface {
	RenderCharacter(snap CharacterSnapshot) error
}

// ColorMapper maps a ColorToken to a client-native value T.
// T is typically color.RGBA (Ebiten), string CSS class (web), or ANSI code (telnet).
type ColorMapper[T any] interface {
	Map(token ColorToken) T
}
