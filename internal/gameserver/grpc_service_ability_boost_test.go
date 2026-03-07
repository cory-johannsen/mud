package gameserver

import (
	"context"
	"net"
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
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/ruleset"
	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// mockAbilityBoostsRepo is a test double for postgres.CharacterAbilityBoostsRepository.
//
// GetAll returns the configured initial map. Add records calls and appends to an
// in-memory store so subsequent calls to GetAll (if any) would reflect additions.
type mockAbilityBoostsRepo struct {
	// initialBoosts is returned by GetAll on first call; keyed by source → abilities.
	initialBoosts map[string][]string
	// getAllErr, if non-nil, is returned by GetAll instead of initialBoosts.
	getAllErr error
	// addCalls records every Add invocation.
	addCalls []struct{ source, ability string }
	// addCallCount is an atomic counter for safe reads from the test goroutine.
	addCallCount atomic.Int32
}

// GetAll satisfies CharacterAbilityBoostsRepository.
//
// Precondition: characterID must be > 0 (not enforced by mock).
// Postcondition: Returns initialBoosts or getAllErr.
func (m *mockAbilityBoostsRepo) GetAll(_ context.Context, _ int64) (map[string][]string, error) {
	if m.getAllErr != nil {
		return nil, m.getAllErr
	}
	if m.initialBoosts != nil {
		return m.initialBoosts, nil
	}
	return map[string][]string{}, nil
}

// Add satisfies CharacterAbilityBoostsRepository; records the call.
//
// Precondition: characterID > 0; source and ability non-empty (not enforced by mock).
// Postcondition: The (source, ability) pair is appended to addCalls.
func (m *mockAbilityBoostsRepo) Add(_ context.Context, _ int64, source, ability string) error {
	m.addCalls = append(m.addCalls, struct{ source, ability string }{source, ability})
	m.addCallCount.Add(1)
	return nil
}

// mockCharSaverAbilityBoost extends mockCharSaverFull with ability score recording.
//
// It returns a character with the given region and class from GetByID so that
// the session setup and boost prompting logic have a coherent DB character.
type mockCharSaverAbilityBoost struct {
	mockCharSaverFull
	region          string
	class           string
	saveAbilitiesCount atomic.Int32
	savedAbilities  character.AbilityScores
}

// GetByID satisfies CharacterSaver; returns a character with region and class set.
func (m *mockCharSaverAbilityBoost) GetByID(_ context.Context, id int64) (*character.Character, error) {
	return &character.Character{
		ID:     id,
		Region: m.region,
		Class:  m.class,
	}, nil
}

// SaveAbilities satisfies CharacterSaver; records the call and the saved scores.
func (m *mockCharSaverAbilityBoost) SaveAbilities(_ context.Context, _ int64, abilities character.AbilityScores) error {
	m.saveAbilitiesCount.Add(1)
	m.savedAbilities = abilities
	return nil
}

// testGRPCServerWithAbilityBoosts starts an in-process gRPC server configured with
// the supplied char saver, ability boosts repo, archetypes, and regions, then returns
// a connected client and the underlying session manager.
//
// Precondition: t must be non-nil.
// Postcondition: Returns a connected GameServiceClient and the underlying session.Manager.
func testGRPCServerWithAbilityBoosts(
	t *testing.T,
	saver CharacterSaver,
	boostsRepo *mockAbilityBoostsRepo,
	jobReg *ruleset.JobRegistry,
	archetypes map[string]*ruleset.Archetype,
	regions map[string]*ruleset.Region,
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
		saver, nil, nil, npcMgr, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, jobReg, nil, "",
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, boostsRepo, archetypes, regions,
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

// TestSession_AbilityBoostsPromptedAndApplied verifies that when a character has no
// stored ability boost choices, the server prompts for each missing free boost,
// persists them via the repo, and updates sess.Abilities with non-zero scores.
//
// Setup: archetype has Free=2, region has Free=1 → expect 3 prompts total.
//
// Precondition: CharacterID must be > 0 so the ability boost path executes.
// Postcondition: repo.Add is called 3 times; sess.Abilities is non-zero.
func TestSession_AbilityBoostsPromptedAndApplied(t *testing.T) {
	const archetypeID = "test_arch"
	const regionID = "test_region"
	const jobID = "test_job"

	archetype := &ruleset.Archetype{
		ID:   archetypeID,
		Name: "Test Archetype",
		AbilityBoosts: &ruleset.AbilityBoostGrant{
			Fixed: []string{"brutality"},
			Free:  2,
		},
	}
	region := &ruleset.Region{
		ID:        regionID,
		Name:      "Test Region",
		Modifiers: map[string]int{"grit": 2},
		AbilityBoosts: &ruleset.AbilityBoostGrant{
			Fixed: nil,
			Free:  1,
		},
	}
	job := &ruleset.Job{
		ID:         jobID,
		Name:       "Test Job",
		Archetype:  archetypeID,
		KeyAbility: "quickness",
	}

	jobReg := ruleset.NewJobRegistry()
	jobReg.Register(job)
	archetypes := map[string]*ruleset.Archetype{archetypeID: archetype}
	regions := map[string]*ruleset.Region{regionID: region}

	boostsRepo := &mockAbilityBoostsRepo{}
	saver := &mockCharSaverAbilityBoost{
		region: regionID,
		class:  jobID,
	}

	client, sessMgr := testGRPCServerWithAbilityBoosts(t, saver, boostsRepo, jobReg, archetypes, regions)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	stream, err := client.Session(ctx)
	require.NoError(t, err)

	// Send the join request with class set so job lookup works.
	err = stream.Send(&gamev1.ClientMessage{
		RequestId: "join",
		Payload: &gamev1.ClientMessage_JoinWorld{
			JoinWorld: &gamev1.JoinWorldRequest{
				Uid:         "u_boost",
				Username:    "Booster",
				CharacterId: 1,
				Class:       jobID,
			},
		},
	})
	require.NoError(t, err)

	// Receive initial room view.
	resp, recvErr := stream.Recv()
	require.NoError(t, recvErr)
	require.NotNil(t, resp.GetRoomView(), "expected RoomView as first server event")

	// Helper: receive a prompt message and respond with choice "1".
	answerPrompt := func(index int) {
		t.Helper()
		promptResp, pErr := stream.Recv()
		require.NoError(t, pErr, "expected prompt message %d", index)
		require.NotNil(t, promptResp.GetMessage(), "expected MessageEvent for prompt %d", index)

		sendErr := stream.Send(&gamev1.ClientMessage{
			Payload: &gamev1.ClientMessage_Say{
				Say: &gamev1.SayRequest{Message: "1"},
			},
		})
		require.NoError(t, sendErr, "sending response %d", index)

		// Receive confirmation.
		confirmResp, cErr := stream.Recv()
		require.NoError(t, cErr, "expected confirmation message %d", index)
		require.NotNil(t, confirmResp.GetMessage(), "expected confirmation MessageEvent %d", index)
	}

	// Archetype has Free=2 → 2 prompts.
	answerPrompt(1)
	answerPrompt(2)
	// Region has Free=1 → 1 prompt.
	answerPrompt(3)

	// Allow server to complete session initialization.
	time.Sleep(100 * time.Millisecond)

	sess, ok := sessMgr.GetPlayer("u_boost")
	require.True(t, ok, "player session must exist after login")

	// Three Add calls must have been made: 2 archetype + 1 region.
	assert.Equal(t, int32(3), boostsRepo.addCallCount.Load(),
		"repo.Add must be called exactly 3 times (2 archetype + 1 region)")

	archetypeAdds := 0
	regionAdds := 0
	for _, c := range boostsRepo.addCalls {
		switch c.source {
		case "archetype":
			archetypeAdds++
		case "region":
			regionAdds++
		}
	}
	assert.Equal(t, 2, archetypeAdds, "2 archetype boosts must be persisted")
	assert.Equal(t, 1, regionAdds, "1 region boost must be persisted")

	// sess.Abilities must be non-zero: at minimum grit=12 (base 10 + region modifier +2).
	assert.True(t,
		sess.Abilities.Brutality > 0 ||
			sess.Abilities.Grit > 0 ||
			sess.Abilities.Quickness > 0 ||
			sess.Abilities.Reasoning > 0 ||
			sess.Abilities.Savvy > 0 ||
			sess.Abilities.Flair > 0,
		"sess.Abilities must be non-zero after boost application")

	// Grit must be 12: base 10 + region modifier +2 (no boost chosen for grit in this test
	// since pool ordering picks first available, but at minimum the region modifier applied).
	assert.GreaterOrEqual(t, sess.Abilities.Grit, 12,
		"Grit must be at least 12 (base 10 + region +2 modifier)")

	// SaveAbilities must have been called.
	assert.Equal(t, int32(1), saver.saveAbilitiesCount.Load(),
		"SaveAbilities must be called exactly once after boost computation")
}

// TestSession_AbilityBoostsSkippedWhenAlreadyStored verifies that when a character
// already has all boost choices stored, no prompts are sent and no Add calls are made.
//
// Precondition: CharacterID must be > 0; all boost slots already filled.
// Postcondition: repo.Add is never called; sess.Abilities reflects the stored boosts.
func TestSession_AbilityBoostsSkippedWhenAlreadyStored(t *testing.T) {
	const archetypeID = "test_arch2"
	const regionID = "test_region2"
	const jobID = "test_job2"

	archetype := &ruleset.Archetype{
		ID:   archetypeID,
		Name: "Test Archetype 2",
		AbilityBoosts: &ruleset.AbilityBoostGrant{
			Fixed: nil,
			Free:  1,
		},
	}
	region := &ruleset.Region{
		ID:        regionID,
		Name:      "Test Region 2",
		Modifiers: map[string]int{},
		AbilityBoosts: &ruleset.AbilityBoostGrant{
			Fixed: nil,
			Free:  1,
		},
	}
	job := &ruleset.Job{
		ID:        jobID,
		Name:      "Test Job 2",
		Archetype: archetypeID,
	}

	jobReg := ruleset.NewJobRegistry()
	jobReg.Register(job)
	archetypes := map[string]*ruleset.Archetype{archetypeID: archetype}
	regions := map[string]*ruleset.Region{regionID: region}

	// Both boost slots already stored → no prompts expected.
	boostsRepo := &mockAbilityBoostsRepo{
		initialBoosts: map[string][]string{
			"archetype": {"grit"},
			"region":    {"savvy"},
		},
	}
	saver := &mockCharSaverAbilityBoost{
		region: regionID,
		class:  jobID,
	}

	client, sessMgr := testGRPCServerWithAbilityBoosts(t, saver, boostsRepo, jobReg, archetypes, regions)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stream, err := client.Session(ctx)
	require.NoError(t, err)

	err = stream.Send(&gamev1.ClientMessage{
		RequestId: "join",
		Payload: &gamev1.ClientMessage_JoinWorld{
			JoinWorld: &gamev1.JoinWorldRequest{
				Uid:         "u_boost_stored",
				Username:    "Stored",
				CharacterId: 2,
				Class:       jobID,
			},
		},
	})
	require.NoError(t, err)

	// Receive initial room view.
	resp, recvErr := stream.Recv()
	require.NoError(t, recvErr)
	require.NotNil(t, resp.GetRoomView(), "expected RoomView as first server event")

	// Allow server to complete session initialization without prompts.
	time.Sleep(100 * time.Millisecond)

	sess, ok := sessMgr.GetPlayer("u_boost_stored")
	require.True(t, ok, "player session must exist after login")

	assert.Equal(t, int32(0), boostsRepo.addCallCount.Load(),
		"repo.Add must not be called when all boost slots are already stored")

	// Abilities must be recomputed with the stored boosts applied.
	// grit boost (+2) and savvy boost (+2) must be reflected.
	assert.Equal(t, 12, sess.Abilities.Grit, "Grit must be 12 (base 10 + archetype free boost +2)")
	assert.Equal(t, 12, sess.Abilities.Savvy, "Savvy must be 12 (base 10 + region free boost +2)")
	assert.Equal(t, int32(1), saver.saveAbilitiesCount.Load(),
		"SaveAbilities must be called once even when no prompts are needed")
}
