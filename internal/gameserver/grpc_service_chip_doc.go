package gameserver

import (
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

func (s *GameServiceServer) handleUncurse(uid string, req *gamev1.UncurseRequest) (*gamev1.ServerEvent, error) {
	return messageEvent("not implemented"), nil
}
