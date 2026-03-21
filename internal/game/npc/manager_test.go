package npc

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestManager_TemplateByID_ReturnsTemplate(t *testing.T) {
	mgr := NewManager()
	tmpl := &Template{
		ID: "test_tmpl", Name: "Test", NPCType: "combat",
		MaxHP: 10, AC: 10, Level: 1,
	}
	_, err := mgr.Spawn(tmpl, "room-1")
	require.NoError(t, err)
	got := mgr.TemplateByID("test_tmpl")
	require.NotNil(t, got)
	assert.Equal(t, "test_tmpl", got.ID)
}

func TestManager_TemplateByID_MissingReturnsNil(t *testing.T) {
	mgr := NewManager()
	assert.Nil(t, mgr.TemplateByID("nonexistent"))
}

func TestManager_InstanceByID_ReturnsInstance(t *testing.T) {
	mgr := NewManager()
	tmpl := &Template{ID: "bandit", Name: "Bandit", NPCType: "combat", MaxHP: 10, AC: 10, Level: 1}
	inst, err := mgr.Spawn(tmpl, "room-1")
	require.NoError(t, err)
	got := mgr.InstanceByID(inst.ID)
	require.NotNil(t, got)
	assert.Equal(t, inst.ID, got.ID)
}

func TestManager_InstanceByID_MissingReturnsNil(t *testing.T) {
	mgr := NewManager()
	assert.Nil(t, mgr.InstanceByID("ghost"))
}
