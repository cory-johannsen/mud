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
	worldHandler := NewWorldHandler(worldMgr, sessMgr, npcMgr, nil)
	chatHandler := NewChatHandler(sessMgr)
	logger := zaptest.NewLogger(t)

	svc := NewGameServiceServer(
		worldMgr, sessMgr, cmdRegistry,
		worldHandler, chatHandler, logger,
		saver, nil, nil, npcMgr, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, "",
		nil, nil, nil, nil, nil, nil, nil, nil, nil,
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
	worldHandler := NewWorldHandler(worldMgr, sessMgr, npcMgr, nil)
	chatHandler := NewChatHandler(sessMgr)
	logger := zaptest.NewLogger(t)

	svc := NewGameServiceServer(
		worldMgr, sessMgr, cmdRegistry,
		worldHandler, chatHandler, logger,
		nil, nil, nil, npcMgr, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, "",
		nil, nil, nil, nil, nil, nil, cfRegistry, cfRepo, nil,
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

// mockFavoredTargetRepo is a test double for FavoredTargetRepository.
//
// It records Set calls and returns a configured value from Get.
type mockFavoredTargetRepo struct {
	// getVal is the value returned by Get.
	getVal string
	// getErr, if non-nil, is returned by Get instead of getVal.
	getErr error

	// setCalled records the argument passed to Set.
	setCalled string
	// setErr, if non-nil, is returned by Set.
	setErr error
}

// Get satisfies FavoredTargetRepository.
func (m *mockFavoredTargetRepo) Get(_ context.Context, _ int64) (string, error) {
	if m.getErr != nil {
		return "", m.getErr
	}
	return m.getVal, nil
}

// Set satisfies FavoredTargetRepository; records the call.
func (m *mockFavoredTargetRepo) Set(_ context.Context, _ int64, targetType string) error {
	m.setCalled = targetType
	return m.setErr
}

// testGRPCServerWithFavoredTarget starts an in-process gRPC server configured
// with the supplied class feature registry, class features repo, and favored target repo.
//
// Precondition: t must be non-nil; all repo/registry args may be nil.
// Postcondition: Returns a connected GameServiceClient and the underlying session.Manager.
func testGRPCServerWithFavoredTarget(
	t *testing.T,
	cfRegistry *ruleset.ClassFeatureRegistry,
	cfRepo CharacterClassFeaturesGetter,
	ftRepo FavoredTargetRepository,
) (gamev1.GameServiceClient, *session.Manager) {
	t.Helper()

	worldMgr, sessMgr := testWorldAndSession(t)
	cmdRegistry := command.DefaultRegistry()
	npcMgr := npc.NewManager()
	worldHandler := NewWorldHandler(worldMgr, sessMgr, npcMgr, nil)
	chatHandler := NewChatHandler(sessMgr)
	logger := zaptest.NewLogger(t)

	svc := NewGameServiceServer(
		worldMgr, sessMgr, cmdRegistry,
		worldHandler, chatHandler, logger,
		nil, nil, nil, npcMgr, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, "",
		nil, nil, nil, nil, nil, nil, cfRegistry, cfRepo, ftRepo,
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
// from the repo when a character logs in.
//
// Precondition: characterID must be > 0 so the favored target loading path executes.
// Postcondition: sess.FavoredTarget equals the value returned by the repo.
func TestSession_FavoredTargetLoadedAtLogin(t *testing.T) {
	ftRepo := &mockFavoredTargetRepo{getVal: "robot"}

	client, sessMgr := testGRPCServerWithFavoredTarget(t, nil, nil, ftRepo)

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
		"FavoredTarget must be populated from repo at login")
}

// TestSession_FavoredTargetPromptedWhenMissing verifies that when a character
// holds predators_eye but has no favored target set, the server prompts them
// to choose, reads their selection, persists it via the repo, and sets it on
// the session.
//
// Precondition: characterID must be > 0; PassiveFeats["predators_eye"] must be true.
// Postcondition: sess.FavoredTarget equals the chosen value; repo.Set was called.
func TestSession_FavoredTargetPromptedWhenMissing(t *testing.T) {
	// predators_eye is a passive class feature.
	predEye := &ruleset.ClassFeature{ID: "predators_eye", Name: "Predator's Eye", Active: false}
	cfRegistry := ruleset.NewClassFeatureRegistry([]*ruleset.ClassFeature{predEye})
	cfRepo := &mockClassFeaturesRepo{featureIDs: []string{"predators_eye"}}

	// Repo returns "" so the prompt path is entered.
	ftRepo := &mockFavoredTargetRepo{getVal: ""}

	client, sessMgr := testGRPCServerWithFavoredTarget(t, cfRegistry, cfRepo, ftRepo)

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

	// The server sends a prompt message before the room view because predators_eye is set.
	// Receive the prompt.
	promptResp, recvErr := stream.Recv()
	require.NoError(t, recvErr)
	require.NotNil(t, promptResp.GetMessage(), "expected prompt MessageEvent before room view")

	// Respond with "2" to select "robot".
	err = stream.Send(&gamev1.ClientMessage{
		Payload: &gamev1.ClientMessage_Say{
			Say: &gamev1.SayRequest{Message: "2"},
		},
	})
	require.NoError(t, err)

	// Receive confirmation message.
	confirmResp, recvErr2 := stream.Recv()
	require.NoError(t, recvErr2)
	require.NotNil(t, confirmResp.GetMessage(), "expected confirmation MessageEvent after selection")

	// Receive the room view.
	roomResp, recvErr3 := stream.Recv()
	require.NoError(t, recvErr3)
	require.NotNil(t, roomResp.GetRoomView(), "expected RoomView after favored target selection")

	// Allow server goroutine to complete.
	time.Sleep(50 * time.Millisecond)

	sess, ok := sessMgr.GetPlayer("u2")
	require.True(t, ok, "player session must exist after login")
	assert.Equal(t, "robot", sess.FavoredTarget,
		"FavoredTarget must be set to the chosen value after prompt")
	assert.Equal(t, "robot", ftRepo.setCalled,
		"repo.Set must be called with the chosen favored target type")
}
