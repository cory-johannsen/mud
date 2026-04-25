package gameserver

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/aoe"
	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/cory-johannsen/mud/internal/game/ruleset"
	"github.com/cory-johannsen/mud/internal/game/technology"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// validatorSvc builds a minimal GameServiceServer with the registries needed
// to exercise resolveContentAoeShape and validateUseRequestAoeTemplate.
func validatorSvc(t *testing.T) *GameServiceServer {
	t.Helper()

	techReg := technology.NewRegistry()
	techReg.Register(&technology.TechnologyDef{
		ID:        "tech_cone",
		Name:      "Cone Tech",
		AoeShape:  aoe.AoeShapeCone,
		AoeLength: 30,
	})
	techReg.Register(&technology.TechnologyDef{
		ID:        "tech_burst",
		Name:      "Burst Tech",
		AoeShape:  aoe.AoeShapeBurst,
		AoeRadius: 2,
	})
	techReg.Register(&technology.TechnologyDef{
		ID:   "tech_single",
		Name: "Single Tech",
	})

	featReg := ruleset.NewFeatRegistryFromSlice([]*ruleset.Feat{
		{
			ID:         "feat_line",
			Name:       "Line Feat",
			Category:   "combat",
			Active:     true,
			ActionCost: 1,
			AoeShape:   aoe.AoeShapeLine,
			AoeLength:  20,
		},
		{
			ID:         "feat_burst",
			Name:       "Burst Feat",
			Category:   "combat",
			Active:     true,
			ActionCost: 1,
			AoeShape:   aoe.AoeShapeBurst,
			AoeRadius:  2,
		},
	})

	cfReg := ruleset.NewClassFeatureRegistry([]*ruleset.ClassFeature{
		{
			ID:         "cf_cone",
			Name:       "Cone Feature",
			Active:     true,
			ActionCost: 1,
			AoeShape:   aoe.AoeShapeCone,
			AoeLength:  30,
		},
	})

	invReg := inventory.NewRegistry()
	if err := invReg.RegisterExplosive(&inventory.ExplosiveDef{
		ID:        "exp_line",
		Name:      "Line Explosive",
		AoeShape:  aoe.AoeShapeLine,
		AoeLength: 30,
	}); err != nil {
		t.Fatalf("register exp_line: %v", err)
	}

	return &GameServiceServer{
		featRegistry:         featReg,
		classFeatureRegistry: cfReg,
		techRegistry:         techReg,
		invRegistry:          invReg,
	}
}

func TestValidateUseRequestAoeTemplate_ConeRequiresTemplate(t *testing.T) {
	svc := validatorSvc(t)
	req := &gamev1.UseRequest{FeatId: "tech_cone"}
	err := svc.validateUseRequestAoeTemplate(req)
	if err == nil {
		t.Fatal("expected InvalidArgument error for cone tech without template; got nil")
	}
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument; got code=%s err=%v", status.Code(err), err)
	}
}

func TestValidateUseRequestAoeTemplate_LineRequiresTemplate(t *testing.T) {
	svc := validatorSvc(t)
	req := &gamev1.UseRequest{FeatId: "feat_line", TargetX: 3, TargetY: 4}
	err := svc.validateUseRequestAoeTemplate(req)
	if err == nil {
		t.Fatal("expected InvalidArgument for line content with target_x/y but no template")
	}
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument; got code=%s", status.Code(err))
	}
}

func TestValidateUseRequestAoeTemplate_LineWithTemplateAccepted(t *testing.T) {
	svc := validatorSvc(t)
	req := &gamev1.UseRequest{
		FeatId: "feat_line",
		Template: &gamev1.AoeTemplate{
			Shape:   gamev1.AoeTemplate_SHAPE_LINE,
			AnchorX: 1,
			AnchorY: 2,
			Facing:  gamev1.AoeTemplate_DIR_E,
		},
	}
	if err := svc.validateUseRequestAoeTemplate(req); err != nil {
		t.Fatalf("expected nil error for line with template; got %v", err)
	}
}

func TestValidateUseRequestAoeTemplate_BurstAcceptsLegacyCoords(t *testing.T) {
	svc := validatorSvc(t)
	req := &gamev1.UseRequest{
		FeatId:  "feat_burst",
		TargetX: 3,
		TargetY: 4,
	}
	if err := svc.validateUseRequestAoeTemplate(req); err != nil {
		t.Fatalf("expected nil error for burst with target_x/y back-compat; got %v", err)
	}
}

func TestValidateUseRequestAoeTemplate_BurstWithTemplateAccepted(t *testing.T) {
	svc := validatorSvc(t)
	req := &gamev1.UseRequest{
		FeatId: "feat_burst",
		Template: &gamev1.AoeTemplate{
			Shape:   gamev1.AoeTemplate_SHAPE_BURST,
			AnchorX: 5,
			AnchorY: 5,
		},
	}
	if err := svc.validateUseRequestAoeTemplate(req); err != nil {
		t.Fatalf("expected nil error for burst with template; got %v", err)
	}
}

func TestValidateUseRequestAoeTemplate_BurstNoCoordsNoTemplateRejected(t *testing.T) {
	svc := validatorSvc(t)
	req := &gamev1.UseRequest{
		FeatId:  "feat_burst",
		TargetX: -1,
		TargetY: -1,
	}
	err := svc.validateUseRequestAoeTemplate(req)
	if err == nil {
		t.Fatal("expected InvalidArgument when burst has no template and negative coords")
	}
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument; got %s", status.Code(err))
	}
}

func TestValidateUseRequestAoeTemplate_SingleTargetNoTemplateOk(t *testing.T) {
	svc := validatorSvc(t)
	req := &gamev1.UseRequest{FeatId: "tech_single"}
	if err := svc.validateUseRequestAoeTemplate(req); err != nil {
		t.Fatalf("single-target tech should not require template; got %v", err)
	}
}

func TestValidateUseRequestAoeTemplate_EmptyFeatIdOk(t *testing.T) {
	svc := validatorSvc(t)
	if err := svc.validateUseRequestAoeTemplate(&gamev1.UseRequest{}); err != nil {
		t.Fatalf("empty feat_id (choices list) should not require template; got %v", err)
	}
}

func TestValidateUseRequestAoeTemplate_UnknownContentOk(t *testing.T) {
	svc := validatorSvc(t)
	if err := svc.validateUseRequestAoeTemplate(&gamev1.UseRequest{FeatId: "no_such_id"}); err != nil {
		t.Fatalf("unknown content should not require template; got %v", err)
	}
}

func TestValidateUseRequestAoeTemplate_ClassFeatureConeRequiresTemplate(t *testing.T) {
	svc := validatorSvc(t)
	err := svc.validateUseRequestAoeTemplate(&gamev1.UseRequest{FeatId: "cf_cone"})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument for class-feature cone without template; got %v", err)
	}
}

func TestValidateUseRequestAoeTemplate_ExplosiveLineRequiresTemplate(t *testing.T) {
	svc := validatorSvc(t)
	err := svc.validateUseRequestAoeTemplate(&gamev1.UseRequest{FeatId: "exp_line"})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument for explosive line without template; got %v", err)
	}
}

func TestValidateUseRequestAoeTemplate_NilRequestOk(t *testing.T) {
	svc := validatorSvc(t)
	if err := svc.validateUseRequestAoeTemplate(nil); err != nil {
		t.Fatalf("nil request should be a no-op; got %v", err)
	}
}
