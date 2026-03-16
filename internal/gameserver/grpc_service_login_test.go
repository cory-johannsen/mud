package gameserver

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/cory-johannsen/mud/internal/game/character"
	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/ruleset"
	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// mockCharSaverFull is a test double for CharacterSaver that allows controlling
// return values and recording whether save methods are called.
//
// It is distinct from the minimal mockCharSaver defined in teleport_handler_test.go
// and adds load-error injection and call-recording for save operations.
type mockCharSaverFull struct {
	// loadWeaponPresetsErr, if non-nil, is returned by LoadWeaponPresets.
	loadWeaponPresetsErr error
	// loadEquipmentErr, if non-nil, is returned by LoadEquipment.
	loadEquipmentErr error

	// saveWeaponPresetsCalled is incremented each time SaveWeaponPresets is called.
	saveWeaponPresetsCalled atomic.Int32
	// saveEquipmentCalled is incremented each time SaveEquipment is called.
	saveEquipmentCalled atomic.Int32
}

// SaveState satisfies CharacterSaver; always succeeds in tests.
func (m *mockCharSaverFull) SaveState(_ context.Context, _ int64, _ string, _ int) error {
	return nil
}

// LoadWeaponPresets returns an error if loadWeaponPresetsErr is set,
// otherwise returns a default-initialized LoadoutSet.
func (m *mockCharSaverFull) LoadWeaponPresets(_ context.Context, _ int64, _ *inventory.Registry) (*inventory.LoadoutSet, error) {
	if m.loadWeaponPresetsErr != nil {
		return nil, m.loadWeaponPresetsErr
	}
	return inventory.NewLoadoutSet(), nil
}

// SaveWeaponPresets records that it was called and always succeeds.
func (m *mockCharSaverFull) SaveWeaponPresets(_ context.Context, _ int64, _ *inventory.LoadoutSet) error {
	m.saveWeaponPresetsCalled.Add(1)
	return nil
}

// LoadEquipment returns an error if loadEquipmentErr is set,
// otherwise returns a default-initialized Equipment.
func (m *mockCharSaverFull) LoadEquipment(_ context.Context, _ int64) (*inventory.Equipment, error) {
	if m.loadEquipmentErr != nil {
		return nil, m.loadEquipmentErr
	}
	return inventory.NewEquipment(), nil
}

// SaveEquipment records that it was called and always succeeds.
func (m *mockCharSaverFull) SaveEquipment(_ context.Context, _ int64, _ *inventory.Equipment) error {
	m.saveEquipmentCalled.Add(1)
	return nil
}

// LoadInventory satisfies CharacterSaver; always returns an empty slice.
func (m *mockCharSaverFull) LoadInventory(_ context.Context, _ int64) ([]inventory.InventoryItem, error) {
	return nil, nil
}

// SaveInventory satisfies CharacterSaver; always succeeds.
func (m *mockCharSaverFull) SaveInventory(_ context.Context, _ int64, _ []inventory.InventoryItem) error {
	return nil
}

// HasReceivedStartingInventory satisfies CharacterSaver; always returns false.
func (m *mockCharSaverFull) HasReceivedStartingInventory(_ context.Context, _ int64) (bool, error) {
	return false, nil
}

// MarkStartingInventoryGranted satisfies CharacterSaver; always succeeds.
func (m *mockCharSaverFull) MarkStartingInventoryGranted(_ context.Context, _ int64) error {
	return nil
}

// GetByID satisfies CharacterSaver; returns an empty Character with zero-value stats.
func (m *mockCharSaverFull) GetByID(_ context.Context, id int64) (*character.Character, error) {
	return &character.Character{ID: id}, nil
}

// SaveAbilities satisfies CharacterSaver; always succeeds.
func (m *mockCharSaverFull) SaveAbilities(_ context.Context, _ int64, _ character.AbilityScores) error {
	return nil
}

// SaveProgress satisfies CharacterSaver; always succeeds.
func (m *mockCharSaverFull) SaveProgress(_ context.Context, _ int64, _, _, _, _ int) error {
	return nil
}

// SaveDefaultCombatAction satisfies CharacterSaver; always succeeds.
func (m *mockCharSaverFull) SaveDefaultCombatAction(_ context.Context, _ int64, _ string) error {
	return nil
}

func (m *mockCharSaverFull) SaveCurrency(_ context.Context, _ int64, _ int) error { return nil }
func (m *mockCharSaverFull) LoadCurrency(_ context.Context, _ int64) (int, error) { return 0, nil }
func (m *mockCharSaverFull) SaveGender(_ context.Context, _ int64, _ string) error { return nil }
func (m *mockCharSaverFull) SaveHeroPoints(_ context.Context, _ int64, _ int) error { return nil }
func (m *mockCharSaverFull) LoadHeroPoints(_ context.Context, _ int64) (int, error) { return 0, nil }

// testGRPCServerWithSaverFull starts an in-process gRPC server using the supplied
// CharacterSaver and returns a connected client and the session manager.
//
// Precondition: t must be non-nil; saver may be nil (no persistence).
// Postcondition: Returns a connected GameServiceClient and the underlying session.Manager.
func testGRPCServerWithSaverFull(t *testing.T, saver CharacterSaver) (gamev1.GameServiceClient, *session.Manager) {
	t.Helper()

	worldMgr, sessMgr := testWorldAndSession(t)
	cmdRegistry := command.DefaultRegistry()
	npcMgr := npc.NewManager()
	worldHandler := NewWorldHandler(worldMgr, sessMgr, npcMgr, nil, nil, nil)
	chatHandler := NewChatHandler(sessMgr)
	logger := zaptest.NewLogger(t)

	svc := NewGameServiceServer(
		worldMgr, sessMgr, cmdRegistry,
		worldHandler, chatHandler, logger,
		saver, nil, nil, npcMgr, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, "",
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
	)

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	grpcServer := grpc.NewServer()
	gamev1.RegisterGameServiceServer(grpcServer, svc)

	go func() { _ = grpcServer.Serve(lis) }()
	t.Cleanup(func() { grpcServer.Stop() })

	conn, err := grpc.NewClient(lis.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })

	return gamev1.NewGameServiceClient(conn), sessMgr
}

// joinWorldWithCharID sends a JoinWorldRequest with a non-zero CharacterId and
// returns the initial RoomView.
//
// Precondition: stream must be an open Session stream; characterID must be > 0
// for charSaver to be invoked on login.
// Postcondition: Returns the first RoomView received from the server.
func joinWorldWithCharID(t *testing.T, stream gamev1.GameService_SessionClient, uid, username string, characterID int64) *gamev1.RoomView {
	t.Helper()
	err := stream.Send(&gamev1.ClientMessage{
		RequestId: "join",
		Payload: &gamev1.ClientMessage_JoinWorld{
			JoinWorld: &gamev1.JoinWorldRequest{
				Uid:         uid,
				Username:    username,
				CharacterId: characterID,
			},
		},
	})
	require.NoError(t, err)

	resp, err := stream.Recv()
	require.NoError(t, err)
	roomView := resp.GetRoomView()
	require.NotNil(t, roomView, "expected RoomView after join")
	return roomView
}

// TestSession_LoadErrorFallback_LoadoutSet verifies that when LoadWeaponPresets
// returns an error the session is assigned a non-nil default LoadoutSet.
func TestSession_LoadErrorFallback_LoadoutSet(t *testing.T) {
	saver := &mockCharSaverFull{
		loadWeaponPresetsErr: errors.New("db unavailable"),
	}
	client, sessMgr := testGRPCServerWithSaverFull(t, saver)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stream, err := client.Session(ctx)
	require.NoError(t, err)

	joinWorldWithCharID(t, stream, "u1", "Alice", 42)

	// Give the server a moment to complete the login path.
	time.Sleep(50 * time.Millisecond)

	sess, ok := sessMgr.GetPlayer("u1")
	require.True(t, ok, "player session must exist")
	assert.NotNil(t, sess.LoadoutSet, "LoadoutSet must be non-nil even when load returns an error")
}

// TestSession_LoadErrorFallback_Equipment verifies that when LoadEquipment
// returns an error the session is assigned a non-nil default Equipment.
func TestSession_LoadErrorFallback_Equipment(t *testing.T) {
	saver := &mockCharSaverFull{
		loadEquipmentErr: errors.New("db unavailable"),
	}
	client, sessMgr := testGRPCServerWithSaverFull(t, saver)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stream, err := client.Session(ctx)
	require.NoError(t, err)

	joinWorldWithCharID(t, stream, "u1", "Alice", 42)

	time.Sleep(50 * time.Millisecond)

	sess, ok := sessMgr.GetPlayer("u1")
	require.True(t, ok, "player session must exist")
	assert.NotNil(t, sess.Equipment, "Equipment must be non-nil even when load returns an error")
}

// TestSession_Cleanup_SavesWeaponPresetsAndEquipment verifies that
// SaveWeaponPresets and SaveEquipment are invoked during cleanupPlayer.
func TestSession_Cleanup_SavesWeaponPresetsAndEquipment(t *testing.T) {
	saver := &mockCharSaverFull{}
	client, _ := testGRPCServerWithSaverFull(t, saver)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stream, err := client.Session(ctx)
	require.NoError(t, err)

	joinWorldWithCharID(t, stream, "u1", "Alice", 42)

	// Quit to trigger cleanupPlayer via the deferred call in Session.
	err = stream.Send(&gamev1.ClientMessage{
		RequestId: "quit1",
		Payload:   &gamev1.ClientMessage_Quit{Quit: &gamev1.QuitRequest{}},
	})
	require.NoError(t, err)

	// Consume the Disconnected response.
	resp, err := stream.Recv()
	require.NoError(t, err)
	require.NotNil(t, resp.GetDisconnected())

	// Close the stream so Session() returns and the deferred cleanupPlayer runs.
	_ = stream.CloseSend()

	// Allow time for the deferred cleanup to complete.
	require.Eventually(t, func() bool {
		return saver.saveWeaponPresetsCalled.Load() > 0 &&
			saver.saveEquipmentCalled.Load() > 0
	}, 2*time.Second, 25*time.Millisecond,
		"SaveWeaponPresets and SaveEquipment must each be called during cleanup")
}

// mockCharSaverGrantTracking extends mockCharSaverFull with per-method call counters
// for SaveInventory, SaveEquipment, and SaveWeaponPresets so that grantStartingInventory
// persistence calls can be independently verified.
type mockCharSaverGrantTracking struct {
	mockCharSaverFull
	saveInventoryCalled     atomic.Int32
	saveEquipmentCalledGrant atomic.Int32
	saveWeaponPresetsCalledGrant atomic.Int32
}

func (m *mockCharSaverGrantTracking) SaveInventory(_ context.Context, _ int64, _ []inventory.InventoryItem) error {
	m.saveInventoryCalled.Add(1)
	return nil
}

func (m *mockCharSaverGrantTracking) SaveEquipment(_ context.Context, _ int64, _ *inventory.Equipment) error {
	m.saveEquipmentCalledGrant.Add(1)
	return nil
}

func (m *mockCharSaverGrantTracking) SaveWeaponPresets(_ context.Context, _ int64, _ *inventory.LoadoutSet) error {
	m.saveWeaponPresetsCalledGrant.Add(1)
	return nil
}

// TestGrantStartingInventory_SavesEquipmentAndWeaponPresets verifies that
// grantStartingInventory calls SaveEquipment and SaveWeaponPresets in addition
// to SaveInventory after equipping the starting kit.
// This is a regression test for Bug 3.
func TestGrantStartingInventory_SavesEquipmentAndWeaponPresets(t *testing.T) {
	// Write a minimal loadout YAML to a temp directory.
	loadoutsDir := t.TempDir()
	loadoutYAML := `archetype: test_arch
base:
  weapon: ""
  armor: {}
  consumables: []
  currency: 0
`
	require.NoError(t, os.WriteFile(filepath.Join(loadoutsDir, "test_arch.yaml"), []byte(loadoutYAML), 0600))

	saver := &mockCharSaverGrantTracking{}
	logger := zaptest.NewLogger(t)

	mgr := session.NewManager()
	sess, err := mgr.AddPlayer(session.AddPlayerOptions{
		UID:               "u-grant",
		Username:          "grantuser",
		CharName:          "Grantee",
		CharacterID:       99,
		RoomID:            "room1",
		CurrentHP:         10,
		MaxHP:             10,
		Abilities:         character.AbilityScores{},
		Role:              "player",
		RegionDisplayName: "Test Region",
		Class:             "Aggressor",
		Level:             1,
	})
	require.NoError(t, err)

	svc := &GameServiceServer{
		charSaver:   saver,
		invRegistry: inventory.NewRegistry(),
		loadoutsDir: loadoutsDir,
		logger:      logger,
		sessions:    mgr,
		commands:    command.DefaultRegistry(),
	}

	err = svc.grantStartingInventory(context.Background(), sess, 99, "test_arch", "", nil)
	require.NoError(t, err)

	assert.Equal(t, int32(1), saver.saveInventoryCalled.Load(), "SaveInventory must be called once")
	assert.Equal(t, int32(1), saver.saveEquipmentCalledGrant.Load(), "SaveEquipment must be called once during starting grant")
	assert.Equal(t, int32(1), saver.saveWeaponPresetsCalledGrant.Load(), "SaveWeaponPresets must be called once during starting grant")
}

// mockClassFeaturesRepo is a test double for CharacterClassFeaturesGetter.
//
// It returns a fixed set of feature IDs for any characterID.
type mockClassFeaturesRepo struct {
	// featureIDs is the slice returned by GetAll.
	featureIDs []string
	// err, if non-nil, is returned instead of featureIDs.
	err error
}

// GetAll satisfies CharacterClassFeaturesGetter.
func (m *mockClassFeaturesRepo) GetAll(_ context.Context, _ int64) ([]string, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.featureIDs, nil
}

// testGRPCServerWithClassFeatures starts an in-process gRPC server configured
// with the supplied class feature registry and character class features repo.
//
// Precondition: t must be non-nil; cfRegistry and cfRepo may be nil.
// Postcondition: Returns a connected GameServiceClient and the underlying session.Manager.
func testGRPCServerWithClassFeatures(
	t *testing.T,
	cfRegistry *ruleset.ClassFeatureRegistry,
	cfRepo CharacterClassFeaturesGetter,
) (gamev1.GameServiceClient, *session.Manager) {
	t.Helper()

	worldMgr, sessMgr := testWorldAndSession(t)
	cmdRegistry := command.DefaultRegistry()
	npcMgr := npc.NewManager()
	worldHandler := NewWorldHandler(worldMgr, sessMgr, npcMgr, nil, nil, nil)
	chatHandler := NewChatHandler(sessMgr)
	logger := zaptest.NewLogger(t)

	svc := NewGameServiceServer(
		worldMgr, sessMgr, cmdRegistry,
		worldHandler, chatHandler, logger,
		nil, nil, nil, npcMgr, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, "",
		nil, nil, nil, nil, nil, nil, nil, cfRegistry, cfRepo, nil, nil, nil, nil,
		nil, nil,
	)

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	grpcServer := grpc.NewServer()
	gamev1.RegisterGameServiceServer(grpcServer, svc)

	go func() { _ = grpcServer.Serve(lis) }()
	t.Cleanup(func() { grpcServer.Stop() })

	conn, err := grpc.NewClient(lis.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })

	return gamev1.NewGameServiceClient(conn), sessMgr
}

// TestSession_PassiveFeatsPopulatedAtLogin verifies that PassiveFeats is populated
// with only passive (Active==false) class features assigned to the character at login.
//
// Precondition: characterID must be > 0 so the class feature loading path executes.
// Postcondition: sess.PassiveFeats contains exactly the passive feature IDs, not active ones.
func TestSession_PassiveFeatsPopulatedAtLogin(t *testing.T) {
	sucker := &ruleset.ClassFeature{ID: "sucker_punch", Name: "Sucker Punch", Active: false}
	zone := &ruleset.ClassFeature{ID: "zone_awareness", Name: "Zone Awareness", Active: false}
	berserk := &ruleset.ClassFeature{ID: "berserk_rush", Name: "Berserk Rush", Active: true}

	cfRegistry := ruleset.NewClassFeatureRegistry([]*ruleset.ClassFeature{sucker, zone, berserk})
	cfRepo := &mockClassFeaturesRepo{
		featureIDs: []string{"sucker_punch", "zone_awareness", "berserk_rush"},
	}

	client, sessMgr := testGRPCServerWithClassFeatures(t, cfRegistry, cfRepo)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stream, err := client.Session(ctx)
	require.NoError(t, err)

	joinWorldWithCharID(t, stream, "u1", "Alice", 1)

	// Allow server goroutine to complete session initialization.
	time.Sleep(50 * time.Millisecond)

	sess, ok := sessMgr.GetPlayer("u1")
	require.True(t, ok, "player session must exist after login")

	require.NotNil(t, sess.PassiveFeats, "PassiveFeats must be non-nil after login")
	assert.True(t, sess.PassiveFeats["sucker_punch"], "sucker_punch must be in PassiveFeats")
	assert.True(t, sess.PassiveFeats["zone_awareness"], "zone_awareness must be in PassiveFeats")
	assert.False(t, sess.PassiveFeats["berserk_rush"], "active feature berserk_rush must not be in PassiveFeats")
}

// mockFeatureChoicesRepo is a test double for CharacterFeatureChoicesRepository.
//
// It records all Set calls and returns a configured map from GetAll.
type mockFeatureChoicesRepo struct {
	// getAllVal is the map returned by GetAll.
	getAllVal map[string]map[string]string
	// getAllErr, if non-nil, is returned by GetAll instead of getAllVal.
	getAllErr error

	// setCalls records every Set invocation in order.
	setCalls []struct{ featureID, choiceKey, value string }
	// setErr, if non-nil, is returned by Set.
	setErr error
}

// GetAll satisfies CharacterFeatureChoicesRepository.
func (m *mockFeatureChoicesRepo) GetAll(_ context.Context, _ int64) (map[string]map[string]string, error) {
	if m.getAllErr != nil {
		return nil, m.getAllErr
	}
	if m.getAllVal != nil {
		return m.getAllVal, nil
	}
	return make(map[string]map[string]string), nil
}

// Set satisfies CharacterFeatureChoicesRepository; records all calls.
func (m *mockFeatureChoicesRepo) Set(_ context.Context, _ int64, featureID, choiceKey, value string) error {
	m.setCalls = append(m.setCalls, struct{ featureID, choiceKey, value string }{featureID, choiceKey, value})
	return m.setErr
}

// testGRPCServerWithFeatureChoices starts an in-process gRPC server configured
// with the supplied class feature registry, class features repo, and feature choices repo.
//
// Precondition: t must be non-nil; all repo/registry args may be nil.
// Postcondition: Returns a connected GameServiceClient and the underlying session.Manager.
func testGRPCServerWithFeatureChoices(
	t *testing.T,
	cfRegistry *ruleset.ClassFeatureRegistry,
	cfRepo CharacterClassFeaturesGetter,
	fcRepo CharacterFeatureChoicesRepository,
) (gamev1.GameServiceClient, *session.Manager) {
	t.Helper()

	worldMgr, sessMgr := testWorldAndSession(t)
	cmdRegistry := command.DefaultRegistry()
	npcMgr := npc.NewManager()
	worldHandler := NewWorldHandler(worldMgr, sessMgr, npcMgr, nil, nil, nil)
	chatHandler := NewChatHandler(sessMgr)
	logger := zaptest.NewLogger(t)

	svc := NewGameServiceServer(
		worldMgr, sessMgr, cmdRegistry,
		worldHandler, chatHandler, logger,
		nil, nil, nil, npcMgr, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, "",
		nil, nil, nil, nil, nil, nil, nil, cfRegistry, cfRepo, fcRepo, nil, nil, nil,
		nil, nil,
	)

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	grpcServer := grpc.NewServer()
	gamev1.RegisterGameServiceServer(grpcServer, svc)

	go func() { _ = grpcServer.Serve(lis) }()
	t.Cleanup(func() { grpcServer.Stop() })

	conn, err := grpc.NewClient(lis.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })

	return gamev1.NewGameServiceClient(conn), sessMgr
}

// TestSession_FavoredTargetLoadedAtLogin verifies that FavoredTarget is populated
// from the feature choices repo when a character logs in with a stored choice.
//
// Precondition: characterID must be > 0 so the feature choices loading path executes.
// Postcondition: sess.FavoredTarget equals the value stored in the repo.
func TestSession_FavoredTargetLoadedAtLogin(t *testing.T) {
	fcRepo := &mockFeatureChoicesRepo{
		getAllVal: map[string]map[string]string{
			"predators_eye": {"favored_target": "robot"},
		},
	}

	client, sessMgr := testGRPCServerWithFeatureChoices(t, nil, nil, fcRepo)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stream, err := client.Session(ctx)
	require.NoError(t, err)

	joinWorldWithCharID(t, stream, "u1", "Alice", 1)

	// Allow server goroutine to complete session initialization.
	time.Sleep(50 * time.Millisecond)

	sess, ok := sessMgr.GetPlayer("u1")
	require.True(t, ok, "player session must exist after login")
	assert.Equal(t, "robot", sess.FavoredTarget,
		"FavoredTarget must be populated from feature choices repo at login")
}

// TestSession_FavoredTargetPromptedWhenMissing verifies that when a character
// holds predators_eye (with a Choices block) but has no favored_target stored,
// the server prompts them to choose, persists via the repo, and sets it on
// the session.
//
// Precondition: characterID must be > 0; predators_eye ClassFeature must have Choices.
// Postcondition: sess.FavoredTarget equals the chosen value; repo.Set was called.
func TestSession_FavoredTargetPromptedWhenMissing(t *testing.T) {
	// predators_eye is a passive class feature with a favored_target choice.
	predEye := &ruleset.ClassFeature{
		ID:     "predators_eye",
		Name:   "Predator's Eye",
		Active: false,
		Choices: &ruleset.FeatureChoices{
			Key:     "favored_target",
			Prompt:  "Choose your favored target type:",
			Options: []string{"human", "robot", "animal", "mutant"},
		},
	}
	cfRegistry := ruleset.NewClassFeatureRegistry([]*ruleset.ClassFeature{predEye})
	cfRepo := &mockClassFeaturesRepo{featureIDs: []string{"predators_eye"}}

	// Repo returns empty map so the prompt path is entered.
	fcRepo := &mockFeatureChoicesRepo{}

	client, sessMgr := testGRPCServerWithFeatureChoices(t, cfRegistry, cfRepo, fcRepo)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stream, err := client.Session(ctx)
	require.NoError(t, err)

	// Send the join request.
	err = stream.Send(&gamev1.ClientMessage{
		RequestId: "join",
		Payload: &gamev1.ClientMessage_JoinWorld{
			JoinWorld: &gamev1.JoinWorldRequest{
				Uid:         "u2",
				Username:    "Bob",
				CharacterId: 1,
			},
		},
	})
	require.NoError(t, err)

	// The server sends the room view first (to initialize split-screen), then the prompt.
	// Receive the room view.
	roomResp, recvErr := stream.Recv()
	require.NoError(t, recvErr)
	require.NotNil(t, roomResp.GetRoomView(), "expected RoomView before prompt")

	// Receive the prompt message.
	promptResp, recvErr2 := stream.Recv()
	require.NoError(t, recvErr2)
	require.NotNil(t, promptResp.GetMessage(), "expected prompt MessageEvent after room view")

	// Respond with "2" to select "robot".
	err = stream.Send(&gamev1.ClientMessage{
		Payload: &gamev1.ClientMessage_Say{
			Say: &gamev1.SayRequest{Message: "2"},
		},
	})
	require.NoError(t, err)

	// Receive confirmation message.
	confirmResp, recvErr3 := stream.Recv()
	require.NoError(t, recvErr3)
	require.NotNil(t, confirmResp.GetMessage(), "expected confirmation MessageEvent after selection")

	// Allow server goroutine to complete.
	time.Sleep(50 * time.Millisecond)

	sess, ok := sessMgr.GetPlayer("u2")
	require.True(t, ok, "player session must exist after login")
	assert.Equal(t, "robot", sess.FavoredTarget,
		"FavoredTarget must be set to the chosen value after prompt")
	require.Len(t, fcRepo.setCalls, 1, "repo.Set must be called exactly once")
	assert.Equal(t, "predators_eye", fcRepo.setCalls[0].featureID,
		"repo.Set must be called with feature ID predators_eye")
	assert.Equal(t, "favored_target", fcRepo.setCalls[0].choiceKey,
		"repo.Set must be called with choice key favored_target")
	assert.Equal(t, "robot", fcRepo.setCalls[0].value,
		"repo.Set must be called with the chosen favored target value")
}

// TestSession_FeatureChoicesGetAllError_FallsBackToEmptyMap verifies that when
// the feature choices repo returns an error on GetAll the login still succeeds,
// the session is created, and FeatureChoices is an empty (non-nil) map.
//
// Precondition: characterID must be > 0 so the feature-choices loading path runs.
// Postcondition: Login completes; sess.FeatureChoices is non-nil and empty.
func TestSession_FeatureChoicesGetAllError_FallsBackToEmptyMap(t *testing.T) {
	fcRepo := &mockFeatureChoicesRepo{
		getAllErr: errors.New("db unavailable"),
	}

	client, sessMgr := testGRPCServerWithFeatureChoices(t, nil, nil, fcRepo)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stream, err := client.Session(ctx)
	require.NoError(t, err)

	joinWorldWithCharID(t, stream, "u-fc-err", "Carol", 1)

	time.Sleep(50 * time.Millisecond)

	sess, ok := sessMgr.GetPlayer("u-fc-err")
	require.True(t, ok, "player session must exist after login even when feature choices repo errors")
	require.NotNil(t, sess.FeatureChoices, "FeatureChoices must be non-nil after repo error")
	assert.Empty(t, sess.FeatureChoices, "FeatureChoices must be empty when repo returns an error")
}

// TestSession_FavoredTargetPromptedWhenMissing_InvalidInput verifies that when the
// player responds to a choice prompt with non-numeric or out-of-range input,
// promptFeatureChoice returns an empty string (no choice stored, no crash).
//
// Precondition: predators_eye has Choices; stored choices are empty.
// Postcondition: sess.FavoredTarget is empty; no repo.Set call is recorded.
func TestSession_FavoredTargetPromptedWhenMissing_InvalidInput(t *testing.T) {
	predEye := &ruleset.ClassFeature{
		ID:     "predators_eye",
		Name:   "Predator's Eye",
		Active: false,
		Choices: &ruleset.FeatureChoices{
			Key:     "favored_target",
			Prompt:  "Choose your favored target type:",
			Options: []string{"human", "robot", "animal", "mutant"},
		},
	}
	cfRegistry := ruleset.NewClassFeatureRegistry([]*ruleset.ClassFeature{predEye})
	cfRepo := &mockClassFeaturesRepo{featureIDs: []string{"predators_eye"}}
	fcRepo := &mockFeatureChoicesRepo{}

	client, sessMgr := testGRPCServerWithFeatureChoices(t, cfRegistry, cfRepo, fcRepo)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stream, err := client.Session(ctx)
	require.NoError(t, err)

	err = stream.Send(&gamev1.ClientMessage{
		RequestId: "join",
		Payload: &gamev1.ClientMessage_JoinWorld{
			JoinWorld: &gamev1.JoinWorldRequest{
				Uid:         "u-invalid",
				Username:    "Dave",
				CharacterId: 1,
			},
		},
	})
	require.NoError(t, err)

	// The server sends the room view first (to initialize split-screen), then the prompt.
	// Receive the room view.
	roomResp, recvErr := stream.Recv()
	require.NoError(t, recvErr)
	require.NotNil(t, roomResp.GetRoomView(), "expected RoomView before prompt")

	// Receive the prompt message.
	promptResp, recvErr2 := stream.Recv()
	require.NoError(t, recvErr2)
	require.NotNil(t, promptResp.GetMessage(), "expected prompt MessageEvent after room view")

	// Respond with an out-of-range number.
	err = stream.Send(&gamev1.ClientMessage{
		Payload: &gamev1.ClientMessage_Say{
			Say: &gamev1.SayRequest{Message: "99"},
		},
	})
	require.NoError(t, err)

	// Receive the invalid-selection feedback message.
	feedbackResp, recvErr3 := stream.Recv()
	require.NoError(t, recvErr3)
	require.NotNil(t, feedbackResp.GetMessage(), "expected invalid-selection feedback MessageEvent")

	// Receive the room view (login proceeds despite invalid input).
	// Note: the test context times out here since no further room view is sent; that is expected.
	_ = roomResp // already verified above

	time.Sleep(50 * time.Millisecond)

	sess, ok := sessMgr.GetPlayer("u-invalid")
	require.True(t, ok, "player session must exist after login")
	assert.Empty(t, sess.FavoredTarget,
		"FavoredTarget must be empty string when invalid input was provided")
	assert.Empty(t, fcRepo.setCalls,
		"repo.Set must not be called when input was invalid")
}
