package gameserver

import (
	"github.com/cory-johannsen/mud/internal/game/combat"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// ResolveContext carries the per-resolution context used by future
// PostFilterAffectedCells implementations (e.g. visibility / line-of-sight in
// #267). v1 has no fields; the type exists so callers and the seam are stable.
//
// Precondition: none.
// Postcondition: zero-value is a valid context.
type ResolveContext struct {
	// Combat is the active combat. May be nil for non-combat resolutions
	// (e.g. preview / validation paths). Future filters MAY use it to
	// derive line-of-sight from the source combatant.
	Combat *combat.Combat
	// SourceID is the id of the actor placing the template, when known.
	SourceID string
}

// dirFromProto converts a wire-format AoeTemplate.Direction enum into the
// internal combat.Direction octant.
//
// Precondition: none — DIR_UNSPECIFIED maps to combat.DirN as a stable default
// to avoid panics on incomplete client requests.
// Postcondition: returned Direction is in [combat.DirN, combat.DirNW].
func dirFromProto(d gamev1.AoeTemplate_Direction) combat.Direction {
	switch d {
	case gamev1.AoeTemplate_DIR_N:
		return combat.DirN
	case gamev1.AoeTemplate_DIR_NE:
		return combat.DirNE
	case gamev1.AoeTemplate_DIR_E:
		return combat.DirE
	case gamev1.AoeTemplate_DIR_SE:
		return combat.DirSE
	case gamev1.AoeTemplate_DIR_S:
		return combat.DirS
	case gamev1.AoeTemplate_DIR_SW:
		return combat.DirSW
	case gamev1.AoeTemplate_DIR_W:
		return combat.DirW
	case gamev1.AoeTemplate_DIR_NW:
		return combat.DirNW
	default:
		return combat.DirN
	}
}

// CellsForTemplate returns the set of grid cells covered by the AoE template
// for the given content dimensions. Burst templates use radiusFt; cone
// templates use lengthFt; line templates use lengthFt and widthFt.
//
// The function delegates to the geometry helpers in the combat package and
// dispatches on t.Shape. Unknown / unspecified shapes return nil.
//
// Precondition: t must be non-nil. radiusFt, lengthFt, widthFt must be >= 0.
// Postcondition: returned slice contains no duplicates (when geometry helpers
// guarantee that); origin / apex behaviour mirrors the helpers — burst
// includes the centre, cone and line exclude the apex/origin per AOE-10.
func CellsForTemplate(t *gamev1.AoeTemplate, radiusFt, lengthFt, widthFt int) []combat.Cell {
	if t == nil {
		return nil
	}
	anchor := combat.Cell{X: int(t.GetAnchorX()), Y: int(t.GetAnchorY())}
	switch t.GetShape() {
	case gamev1.AoeTemplate_SHAPE_BURST:
		return combat.BurstCells(anchor, radiusFt)
	case gamev1.AoeTemplate_SHAPE_CONE:
		return combat.ConeCells(anchor, dirFromProto(t.GetFacing()), lengthFt)
	case gamev1.AoeTemplate_SHAPE_LINE:
		return combat.LineCells(anchor, dirFromProto(t.GetFacing()), lengthFt, widthFt)
	}
	return nil
}

// PostFilterAffectedCells is the extension seam (AOE-21) where future
// visibility / line-of-sight filtering will plug in (issue #267). The v1
// implementation is the identity function — it returns its input unchanged.
//
// Precondition: cells may be nil; ctx is passed through verbatim.
// Postcondition: returns the same slice header (or nil) — no copy is made in
// the v1 implementation.
func PostFilterAffectedCells(cells []combat.Cell, _ ResolveContext) []combat.Cell {
	return cells
}

// resolveAoeCells is the convenience wrapper used by the inline AoE paths in
// grpc_service.go: it builds the template's cell set, runs the post-filter,
// optionally writes the resolved cells back to the proto template (AOE-12),
// and returns the combatants whose grid square intersects.
//
// When template is nil this is a back-compat path (AOE-16): the caller has
// supplied targetX / targetY for a legacy burst content. A burst template is
// synthesised on the fly so the same code path applies.
//
// Precondition: cbt must be non-nil.
// Postcondition: returned slice is the affected combatants (possibly empty);
// when template is non-nil, template.Cells is populated with the resolved
// grid cells in geometry-helper order.
func resolveAoeCells(template *gamev1.AoeTemplate, radiusFt, lengthFt, widthFt int, targetX, targetY int32, cbt *combat.Combat) ([]*combat.Combatant, []combat.Cell) {
	if template == nil {
		// AOE-16: synthesise a burst template at runtime when only the
		// legacy targetX/targetY were supplied.
		template = &gamev1.AoeTemplate{
			Shape:   gamev1.AoeTemplate_SHAPE_BURST,
			AnchorX: targetX,
			AnchorY: targetY,
		}
	}
	cells := CellsForTemplate(template, radiusFt, lengthFt, widthFt)
	cells = PostFilterAffectedCells(cells, ResolveContext{Combat: cbt})

	// AOE-12: populate the wire-format cells list on the inbound template so
	// downstream emitters can echo it on the outbound combat event.
	if len(cells) > 0 {
		out := make([]*gamev1.AoeTemplate_Cell, 0, len(cells))
		for _, c := range cells {
			out = append(out, &gamev1.AoeTemplate_Cell{X: int32(c.X), Y: int32(c.Y)})
		}
		template.Cells = out
	}

	return combat.CombatantsInCells(cbt, cells), cells
}
