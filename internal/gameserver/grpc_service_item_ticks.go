package gameserver

import "context"

// StartItemTickHook subscribes to the calendar and drives item charge recharge triggers.
// Fires "daily" at Hour==0, "midnight" and "dawn" on period transitions.
//
// Precondition: MUST be called after GameServiceServer is fully initialized.
// Precondition: s.calendar MUST NOT be nil.
// Postcondition: returns a stop function; call it to unsubscribe and stop the goroutine.
func (s *GameServiceServer) StartItemTickHook() func() {
	if s.calendar == nil {
		return func() {}
	}
	ch := make(chan GameDateTime, 4)
	s.calendar.Subscribe(ch)
	stop := make(chan struct{})

	// REQ-ACT-21: initialize lastTimePeriod to avoid spurious triggers on first tick.
	s.itemTickMu.Lock()
	s.lastTimePeriod = s.calendar.CurrentDateTime().Hour.Period()
	s.itemTickMu.Unlock()

	go func() {
		for {
			select {
			case dt := <-ch:
				currentPeriod := dt.Hour.Period()

				// Determine which triggers fire before releasing the lock.
				s.itemTickMu.Lock()
				fireMidnight := currentPeriod == PeriodMidnight && currentPeriod != s.lastTimePeriod
				fireDawn := currentPeriod == PeriodDawn && currentPeriod != s.lastTimePeriod
				if currentPeriod != s.lastTimePeriod {
					s.lastTimePeriod = currentPeriod
				}
				s.itemTickMu.Unlock()

				// tickItemRecharge performs DB I/O and proto pushes — must NOT be called under lock.
				if dt.Hour == 0 {
					s.tickItemRecharge("daily")
				}
				if fireMidnight {
					s.tickItemRecharge("midnight")
				}
				if fireDawn {
					s.tickItemRecharge("dawn")
				}

			case <-stop:
				s.calendar.Unsubscribe(ch)
				return
			}
		}
	}()
	return func() { close(stop) }
}

// tickItemRecharge fires a recharge trigger for all online players' equipped items.
func (s *GameServiceServer) tickItemRecharge(trigger string) {
	ctx := context.Background()
	for _, sess := range s.sessions.AllPlayers() {
		s.runRechargeForSession(ctx, sess, trigger)
	}
}
