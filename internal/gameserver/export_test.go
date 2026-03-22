package gameserver

import (
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/game/technology"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// ExportedBuildOptions exposes buildOptions for white-box testing.
func ExportedBuildOptions(ids []string, levels []int, reg *technology.Registry) []string {
	return buildOptions(ids, levels, reg)
}

// ExportedParseTechID exposes parseTechID for white-box testing.
func ExportedParseTechID(option string) string {
	return parseTechID(option)
}

// RequireEditor exposes requireEditor for white-box testing.
var RequireEditor = func(sess *session.PlayerSession) *gamev1.ServerEvent {
	return requireEditor(sess)
}

// RequireAdmin exposes requireAdmin for white-box testing.
var RequireAdmin = func(sess *session.PlayerSession) *gamev1.ServerEvent {
	return requireAdmin(sess)
}
