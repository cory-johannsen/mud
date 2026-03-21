# Downtime Activity Queue

Allow players to queue multiple downtime activities upfront. After one activity completes, the next starts automatically without player intervention. Extends the single-activity downtime system from the `downtime` feature.

## Requirements

- [ ] Queue management
  - [ ] `downtime queue <activity> [days]` — add activity to end of queue
  - [ ] `downtime queue list` — show queued activities with estimated completion times
  - [ ] `downtime queue clear` — cancel all queued activities (no refund of materials reserved for future items)
  - [ ] `downtime queue remove <position>` — remove a specific queued activity
- [ ] Auto-start — on current activity completion, next queued activity starts automatically if location requirements are still met
- [ ] DB persistence — queue persisted to `character_downtime_queue` table; survives reconnect
- [ ] Location re-validation — if room tags no longer satisfy the next queued activity on auto-start, activity is skipped with a notification and the next eligible activity begins
