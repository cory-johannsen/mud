package session

import (
	"fmt"
	"sync"
	"testing"

	"pgregory.net/rapid"
)

// addTestPlayer is a helper that adds a player with minimal valid options.
func addTestPlayer(t *testing.T, mgr *Manager, uid string) *PlayerSession {
	t.Helper()
	sess, err := mgr.AddPlayer(AddPlayerOptions{
		UID:         uid,
		Username:    uid + "-user",
		CharName:    uid + "-char",
		CharacterID: 1,
		RoomID:      "room-1",
		CurrentHP:   10,
		MaxHP:       10,
		Role:        "player",
	})
	if err != nil {
		t.Fatalf("addTestPlayer(%q): %v", uid, err)
	}
	return sess
}

// TestCreateGroup verifies that CreateGroup allocates a UUID ID, sets LeaderUID,
// adds leader to MemberUIDs, stores in manager.groups, and sets leader session's GroupID.
func TestCreateGroup(t *testing.T) {
	mgr := NewManager()
	addTestPlayer(t, mgr, "uid-0")

	g := mgr.CreateGroup("uid-0")

	if g == nil {
		t.Fatal("CreateGroup returned nil")
	}
	if g.ID == "" {
		t.Error("expected non-empty group ID")
	}
	if g.LeaderUID != "uid-0" {
		t.Errorf("expected LeaderUID=uid-0, got %q", g.LeaderUID)
	}
	if len(g.MemberUIDs) != 1 || g.MemberUIDs[0] != "uid-0" {
		t.Errorf("expected MemberUIDs=[uid-0], got %v", g.MemberUIDs)
	}

	found, ok := mgr.GroupByID(g.ID)
	if !ok || found != g {
		t.Error("group not found in manager.groups after CreateGroup")
	}

	sess, _ := mgr.GetPlayer("uid-0")
	if sess.GroupID != g.ID {
		t.Errorf("expected sess.GroupID=%q, got %q", g.ID, sess.GroupID)
	}
}

// TestAddGroupMember verifies normal add, duplicate error, and full-group error.
func TestAddGroupMember(t *testing.T) {
	mgr := NewManager()
	addTestPlayer(t, mgr, "uid-0")
	addTestPlayer(t, mgr, "uid-1")
	g := mgr.CreateGroup("uid-0")

	// Normal add
	if err := mgr.AddGroupMember(g.ID, "uid-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(g.MemberUIDs) != 2 {
		t.Errorf("expected 2 members, got %d", len(g.MemberUIDs))
	}
	sess1, _ := mgr.GetPlayer("uid-1")
	if sess1.GroupID != g.ID {
		t.Errorf("expected uid-1 GroupID=%q, got %q", g.ID, sess1.GroupID)
	}

	// Duplicate
	if err := mgr.AddGroupMember(g.ID, "uid-1"); err == nil {
		t.Error("expected error for duplicate member, got nil")
	}

	// Not found group
	if err := mgr.AddGroupMember("no-such-group", "uid-1"); err == nil {
		t.Error("expected error for missing group, got nil")
	}
}

// TestAddGroupMember_Full verifies "Group is full" error when group has 8 members.
func TestAddGroupMember_Full(t *testing.T) {
	mgr := NewManager()
	for i := 0; i < 9; i++ {
		addTestPlayer(t, mgr, fmt.Sprintf("uid-%d", i))
	}
	g := mgr.CreateGroup("uid-0")
	for i := 1; i <= 7; i++ {
		if err := mgr.AddGroupMember(g.ID, fmt.Sprintf("uid-%d", i)); err != nil {
			t.Fatalf("unexpected error adding uid-%d: %v", i, err)
		}
	}
	if len(g.MemberUIDs) != 8 {
		t.Fatalf("expected 8 members before cap test, got %d", len(g.MemberUIDs))
	}
	err := mgr.AddGroupMember(g.ID, "uid-8")
	if err == nil {
		t.Fatal("expected error adding 9th member, got nil")
	}
	if err.Error() != "Group is full (max 8 members)." {
		t.Errorf("unexpected error message: %q", err.Error())
	}
}

// TestRemoveGroupMember verifies removal, no-op for absent uid, and clears sess.GroupID.
func TestRemoveGroupMember(t *testing.T) {
	mgr := NewManager()
	addTestPlayer(t, mgr, "uid-0")
	addTestPlayer(t, mgr, "uid-1")
	g := mgr.CreateGroup("uid-0")
	_ = mgr.AddGroupMember(g.ID, "uid-1")

	mgr.RemoveGroupMember(g.ID, "uid-1")

	if len(g.MemberUIDs) != 1 || g.MemberUIDs[0] != "uid-0" {
		t.Errorf("expected [uid-0] after remove, got %v", g.MemberUIDs)
	}
	sess1, _ := mgr.GetPlayer("uid-1")
	if sess1.GroupID != "" {
		t.Errorf("expected sess.GroupID empty after removal, got %q", sess1.GroupID)
	}

	// Solo-leader group persists
	_, ok := mgr.GroupByID(g.ID)
	if !ok {
		t.Error("expected group to persist after member removed (solo-leader)")
	}

	// No-op for absent uid
	mgr.RemoveGroupMember(g.ID, "uid-not-here")
	if len(g.MemberUIDs) != 1 {
		t.Errorf("no-op remove changed MemberUIDs: %v", g.MemberUIDs)
	}

	// No-op for missing group
	mgr.RemoveGroupMember("bogus-group", "uid-0")
}

// TestDisbandGroup verifies deletion from manager and clearing of all member GroupIDs.
func TestDisbandGroup(t *testing.T) {
	mgr := NewManager()
	addTestPlayer(t, mgr, "uid-0")
	addTestPlayer(t, mgr, "uid-1")
	g := mgr.CreateGroup("uid-0")
	_ = mgr.AddGroupMember(g.ID, "uid-1")

	mgr.DisbandGroup(g.ID)

	if _, ok := mgr.GroupByID(g.ID); ok {
		t.Error("expected group to be deleted after DisbandGroup")
	}
	sess0, _ := mgr.GetPlayer("uid-0")
	if sess0.GroupID != "" {
		t.Errorf("expected uid-0 GroupID empty after disband, got %q", sess0.GroupID)
	}
	sess1, _ := mgr.GetPlayer("uid-1")
	if sess1.GroupID != "" {
		t.Errorf("expected uid-1 GroupID empty after disband, got %q", sess1.GroupID)
	}

	// No-op for missing group
	mgr.DisbandGroup("bogus-group")
}

// TestGroupByUID verifies lookup by UID and nil for non-member.
func TestGroupByUID(t *testing.T) {
	mgr := NewManager()
	addTestPlayer(t, mgr, "uid-0")
	addTestPlayer(t, mgr, "uid-1")
	g := mgr.CreateGroup("uid-0")

	found := mgr.GroupByUID("uid-0")
	if found != g {
		t.Errorf("expected group for uid-0, got %v", found)
	}

	if mgr.GroupByUID("uid-1") != nil {
		t.Error("expected nil for non-member uid-1")
	}
}

// TestGroupByID verifies lookup by ID.
func TestGroupByID(t *testing.T) {
	mgr := NewManager()
	addTestPlayer(t, mgr, "uid-0")
	g := mgr.CreateGroup("uid-0")

	found, ok := mgr.GroupByID(g.ID)
	if !ok || found != g {
		t.Error("GroupByID did not return the group")
	}

	_, ok = mgr.GroupByID("no-such-id")
	if ok {
		t.Error("expected false for missing group ID")
	}
}

// TestForEachPlayer verifies fn is called for every connected player.
func TestForEachPlayer(t *testing.T) {
	mgr := NewManager()
	addTestPlayer(t, mgr, "uid-0")
	addTestPlayer(t, mgr, "uid-1")
	addTestPlayer(t, mgr, "uid-2")

	seen := map[string]int{}
	mgr.ForEachPlayer(func(s *PlayerSession) {
		seen[s.UID]++
	})

	for _, uid := range []string{"uid-0", "uid-1", "uid-2"} {
		if seen[uid] != 1 {
			t.Errorf("expected fn called once for %q, got %d", uid, seen[uid])
		}
	}
	if len(seen) != 3 {
		t.Errorf("expected exactly 3 players visited, got %d", len(seen))
	}
}

// TestProperty_AddGroupMember_FullAt8 (REQ-T14): property test that adding a 9th member fails.
func TestProperty_AddGroupMember_FullAt8(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		mgr := NewManager()
		// Generate 9 unique UIDs.
		uids := make([]string, 9)
		for i := 0; i < 9; i++ {
			uids[i] = fmt.Sprintf("prop-uid-%d", i)
			addTestPlayer(t, mgr, uids[i])
		}
		g := mgr.CreateGroup(uids[0])
		for i := 1; i <= 7; i++ {
			if err := mgr.AddGroupMember(g.ID, uids[i]); err != nil {
				rt.Fatalf("unexpected error adding member %d: %v", i, err)
			}
		}
		if len(g.MemberUIDs) != 8 {
			rt.Fatalf("expected 8 members, got %d", len(g.MemberUIDs))
		}
		err := mgr.AddGroupMember(g.ID, uids[8])
		if err == nil {
			rt.Fatal("expected error adding 9th member, got nil")
		}
		if err.Error() != "Group is full (max 8 members)." {
			rt.Fatalf("unexpected error message: %q", err.Error())
		}
	})
}

// TestProperty_DisbandGroup_ClearsAllGroupIDs (REQ-T15): after disband, zero sessions retain that GroupID.
func TestProperty_DisbandGroup_ClearsAllGroupIDs(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Generate a group size between 1 and 8.
		size := rapid.IntRange(1, 8).Draw(rt, "size")
		mgr := NewManager()
		uids := make([]string, size)
		for i := 0; i < size; i++ {
			uids[i] = fmt.Sprintf("prop-uid-%d", i)
			addTestPlayer(t, mgr, uids[i])
		}
		g := mgr.CreateGroup(uids[0])
		for i := 1; i < size; i++ {
			if err := mgr.AddGroupMember(g.ID, uids[i]); err != nil {
				rt.Fatalf("unexpected error: %v", err)
			}
		}
		groupID := g.ID
		mgr.DisbandGroup(groupID)

		count := 0
		mgr.ForEachPlayer(func(s *PlayerSession) {
			if s.GroupID == groupID {
				count++
			}
		})
		if count != 0 {
			rt.Fatalf("expected 0 sessions with disbanded GroupID, got %d", count)
		}
	})
}

// TestProperty_NoDuplicateMemberUIDs (REQ-T16): MemberUIDs never contains duplicates.
func TestProperty_NoDuplicateMemberUIDs(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		mgr := NewManager()
		const poolSize = 8
		for i := 0; i < poolSize; i++ {
			addTestPlayer(t, mgr, fmt.Sprintf("prop-uid-%d", i))
		}
		g := mgr.CreateGroup("prop-uid-0")

		// Perform random create/add/remove operations.
		ops := rapid.IntRange(0, 20).Draw(rt, "ops")
		for op := 0; op < ops; op++ {
			action := rapid.IntRange(0, 2).Draw(rt, fmt.Sprintf("action-%d", op))
			switch action {
			case 0: // add
				idx := rapid.IntRange(1, poolSize-1).Draw(rt, fmt.Sprintf("add-idx-%d", op))
				_ = mgr.AddGroupMember(g.ID, fmt.Sprintf("prop-uid-%d", idx))
			case 1: // remove
				if len(g.MemberUIDs) > 0 {
					idx := rapid.IntRange(0, len(g.MemberUIDs)-1).Draw(rt, fmt.Sprintf("rm-idx-%d", op))
					uid := g.MemberUIDs[idx]
					mgr.RemoveGroupMember(g.ID, uid)
				}
			case 2: // no-op: re-add leader (duplicate should be rejected)
				_ = mgr.AddGroupMember(g.ID, "prop-uid-0")
			}
		}

		// Check for duplicates.
		seen := map[string]int{}
		for _, uid := range g.MemberUIDs {
			seen[uid]++
		}
		for uid, cnt := range seen {
			if cnt > 1 {
				rt.Fatalf("duplicate UID %q appears %d times in MemberUIDs", uid, cnt)
			}
		}
	})
}

// TestProperty_ConcurrentGroupAccess_NoRace (REQ-T17): concurrent reads/writes must not race.
func TestProperty_ConcurrentGroupAccess_NoRace(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		numPlayers := rapid.IntRange(3, 8).Draw(rt, "numPlayers")
		leaderIndices := make([]int, 20)
		for i := range leaderIndices {
			leaderIndices[i] = rapid.IntRange(0, numPlayers-1).Draw(rt, fmt.Sprintf("leader-%d", i))
		}

		mgr := NewManager()
		for i := 0; i < numPlayers; i++ {
			addTestPlayer(t, mgr, fmt.Sprintf("uid-%d", i))
		}
		g := mgr.CreateGroup("uid-0")

		var wg sync.WaitGroup
		const goroutines = 20
		wg.Add(goroutines)
		for i := 0; i < goroutines; i++ {
			i := i
			go func() {
				defer wg.Done()
				switch i % 3 {
				case 0:
					_ = mgr.GroupByUID("uid-0")
				case 1:
					_, _ = mgr.GroupByID(g.ID)
				default:
					ng := mgr.CreateGroup(fmt.Sprintf("uid-%d", leaderIndices[i]))
					mgr.DisbandGroup(ng.ID)
				}
			}()
		}
		wg.Wait()
	})
}
