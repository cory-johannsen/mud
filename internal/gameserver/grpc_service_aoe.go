package gameserver

import (
	"github.com/cory-johannsen/mud/internal/game/aoe"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// resolveContentAoeShape returns the AoeShape declared by the content (feat,
// class feature, technology, or explosive item) identified by abilityID, or
// AoeShapeNone if no matching content is found or the content has no shape.
//
// The lookup mirrors the resolution order in handleUse: techs (with
// short-name resolution), then feats, then class features, then explosive
// inventory items. The first matching content's shape is returned.
//
// Precondition: none — empty abilityID is permitted and yields AoeShapeNone.
// Postcondition: returned shape is a valid aoe.AoeShape value (possibly the
// AoeShapeNone empty value).
func (s *GameServiceServer) resolveContentAoeShape(abilityID string) aoe.AoeShape {
	if abilityID == "" {
		return aoe.AoeShapeNone
	}

	// Resolve tech short-name to canonical ID, mirroring handleUse.
	techID := abilityID
	if s.techRegistry != nil {
		if def, ok := s.techRegistry.GetByShortName(abilityID); ok {
			techID = def.ID
		}
	}

	if s.techRegistry != nil {
		if def, ok := s.techRegistry.Get(techID); ok {
			return def.AoeShape
		}
	}
	if s.featRegistry != nil {
		if f, ok := s.featRegistry.Feat(abilityID); ok {
			return f.AoeShape
		}
	}
	if s.classFeatureRegistry != nil {
		if cf, ok := s.classFeatureRegistry.ClassFeature(abilityID); ok {
			return cf.AoeShape
		}
	}
	if s.invRegistry != nil {
		if exp := s.invRegistry.Explosive(abilityID); exp != nil {
			return exp.AoeShape
		}
	}

	return aoe.AoeShapeNone
}

// validateUseRequestAoeTemplate enforces (AOE-13) the proto contract for
// AoeTemplate on inbound UseRequest messages.
//
// Rules:
//   - Content with AoeShape == cone or line: req.Template MUST be non-nil.
//     Missing template returns codes.InvalidArgument "AoE template required".
//   - Content with AoeShape == burst: req.Template OR non-negative
//     target_x/target_y both satisfy the contract (back-compat with the
//     legacy burst path; AOE-16).
//   - Content with AoeShape == none (single-target / non-AoE) or content not
//     found in any registry: no template required; nil is returned.
//
// Precondition: req may be nil; the validator treats nil as a no-op.
// Postcondition: returns nil iff the request satisfies the contract; otherwise
// returns a gRPC status error with codes.InvalidArgument.
func (s *GameServiceServer) validateUseRequestAoeTemplate(req *gamev1.UseRequest) error {
	if req == nil {
		return nil
	}
	abilityID := req.GetFeatId()
	if abilityID == "" {
		// Empty feat_id is the choices-list request — no template required.
		return nil
	}
	shape := s.resolveContentAoeShape(abilityID)
	switch shape {
	case aoe.AoeShapeCone, aoe.AoeShapeLine:
		if req.GetTemplate() == nil {
			return status.Error(codes.InvalidArgument, "AoE template required")
		}
	case aoe.AoeShapeBurst:
		if req.GetTemplate() == nil && (req.GetTargetX() < 0 || req.GetTargetY() < 0) {
			return status.Error(codes.InvalidArgument, "AoE template required")
		}
	}
	return nil
}
