package command_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/command"
)

func TestHandleClassFeatures_ReturnsConstant(t *testing.T) {
	if command.HandleClassFeatures() != command.HandlerClassFeatures {
		t.Error("HandleClassFeatures should return HandlerClassFeatures")
	}
}

func TestClassFeaturesCommandRegistered(t *testing.T) {
	cmds := command.BuiltinCommands()
	for _, c := range cmds {
		if c.Name == command.HandlerClassFeatures {
			return
		}
	}
	t.Errorf("class_features not found in BuiltinCommands")
}

func TestClassFeaturesAlias(t *testing.T) {
	cmds := command.BuiltinCommands()
	for _, c := range cmds {
		if c.Name == command.HandlerClassFeatures {
			for _, a := range c.Aliases {
				if a == "cf" {
					return
				}
			}
			t.Error("class_features missing alias 'cf'")
			return
		}
	}
	t.Error("class_features not found")
}
