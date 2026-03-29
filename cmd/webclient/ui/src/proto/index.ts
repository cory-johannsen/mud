// TypeScript type definitions mirroring proto/game/v1/game.proto messages.
// These are hand-written mirrors of the proto messages, using camelCase to
// match protojson output (which converts snake_case to camelCase by default).

export interface ExitInfo {
  direction: string
  targetRoomId?: string
  locked?: boolean
  hidden?: boolean
  targetTitle?: string
}

export interface NpcInfo {
  instanceId?: string
  id?: string
  name: string
  healthDescription?: string
  health_description?: string
  fightingTarget?: string
  fighting_target?: string
  conditions?: string[]
  npcType?: string
  npc_type?: string
}

export interface FloorItem {
  instanceId?: string
  name: string
  quantity?: number
}

export interface RoomEquipmentItem {
  instanceId?: string
  name: string
  quantity?: number
  immovable?: boolean
  usable?: boolean
}

export interface ConditionInfo {
  id?: string
  name: string
  stacks?: number
  durationRemaining?: number
  duration_remaining?: number
}

export interface RoomView {
  roomId?: string
  title: string
  description: string
  exits?: ExitInfo[]
  players?: string[]
  npcs?: NpcInfo[]
  activeConditions?: ConditionInfo[]
  active_conditions?: ConditionInfo[]
  floorItems?: FloorItem[]
  floor_items?: FloorItem[]
  equipment?: RoomEquipmentItem[]
  hour?: number
  period?: string
  zoneName?: string
  zone_name?: string
}

export interface CharacterInfo {
  characterId?: number
  id?: number
  name: string
  region?: string
  class?: string
  className?: string
  class_name?: string
  level: number
  experience?: number
  maxHp?: number
  max_hp?: number
  currentHp?: number
  current_hp?: number
  brutality?: number
  quickness?: number
  grit?: number
  reasoning?: number
  savvy?: number
  flair?: number
  team?: string
}

export interface SkillEntry {
  skillId?: string
  name: string
  ability?: string
  proficiency?: string
  bonus?: number
  description?: string
}

export interface FeatEntry {
  featId?: string
  name: string
  category?: string
  active?: boolean
  description?: string
  activateText?: string
}

export interface CharacterSheetView {
  name: string
  job?: string
  archetype?: string
  team?: string
  level: number
  brutality?: number
  grit?: number
  quickness?: number
  reasoning?: number
  savvy?: number
  flair?: number
  currentHp?: number
  current_hp?: number
  maxHp?: number
  max_hp?: number
  acBonus?: number
  checkPenalty?: number
  speedPenalty?: number
  currency?: string
  armor?: Record<string, string>
  accessories?: Record<string, string>
  mainHand?: string
  main_hand?: string
  offHand?: string
  off_hand?: string
  mainHandAttackBonus?: string
  main_hand_attack_bonus?: string
  mainHandDamage?: string
  main_hand_damage?: string
  offHandAttackBonus?: string
  offHandDamage?: string
  totalAc?: number
  skills?: SkillEntry[]
  feats?: FeatEntry[]
  toughnessSave?: number
  hustleSave?: number
  coolSave?: number
  experience?: number
  xpToNext?: number
  pendingBoosts?: number
  pendingSkillIncreases?: number
  heroPoints?: number
  hero_points?: number
  className?: string
  class_name?: string
}

export interface InventoryItem {
  instanceId?: string
  name: string
  kind?: string
  quantity?: number
  weight?: number
}

export interface InventoryView {
  items?: InventoryItem[]
  usedSlots?: number
  used_slots?: number
  maxSlots?: number
  max_slots?: number
  totalWeight?: number
  total_weight?: number
  maxWeight?: number
  currency?: string
  totalRounds?: number
}

export interface MapTile {
  roomId?: string
  roomName?: string
  x?: number
  y?: number
  current?: boolean
  exits?: string[]
  dangerLevel?: string
  danger_level?: string
  pois?: string[]
  bossRoom?: boolean
  boss?: boolean
  name?: string
}

export interface WorldZoneTile {
  zoneId?: string
  zoneName?: string
  worldX?: number
  worldY?: number
  discovered?: boolean
  current?: boolean
  dangerLevel?: string
}

export interface MapResponse {
  tiles?: MapTile[]
  worldTiles?: WorldZoneTile[]
  zoneName?: string
}

export interface MessageEvent {
  sender?: string
  content?: string
  type?: string
}

export interface RoomEvent {
  player?: string
  type?: string
  direction?: string
  action?: string
  message?: string
}

export interface CombatEvent {
  type?: string
  attacker?: string
  target?: string
  attackRoll?: number
  attackTotal?: number
  outcome?: string
  damage?: number
  targetHp?: number
  narrative?: string
  weaponName?: string
  targetMaxHp?: number
  attackerPosition?: number
}

export interface RoundStartEvent {
  round?: number
  actionsPerTurn?: number
  actions_per_turn?: number
  durationMs?: number
  turnOrder?: string[]
  turn_order?: string[]
}

export interface RoundEndEvent {
  round?: number
}

export interface ErrorEvent {
  message?: string
}

export interface TimeOfDayEvent {
  hour?: number
  period?: string
  day?: number
  month?: number
}

export interface ConditionEvent {
  targetUid?: string
  targetName?: string
  conditionId?: string
  conditionName?: string
  condition_name?: string
  stacks?: number
  applied?: boolean
}

export type ServerEvent =
  | { type: 'RoomView'; payload: RoomView }
  | { type: 'CharacterInfo'; payload: CharacterInfo }
  | { type: 'CharacterSheetView'; payload: CharacterSheetView }
  | { type: 'InventoryView'; payload: InventoryView }
  | { type: 'MapResponse'; payload: MapResponse }
  | { type: 'MessageEvent'; payload: MessageEvent }
  | { type: 'RoomEvent'; payload: RoomEvent }
  | { type: 'CombatEvent'; payload: CombatEvent }
  | { type: 'RoundStartEvent'; payload: RoundStartEvent }
  | { type: 'RoundEndEvent'; payload: RoundEndEvent }
  | { type: 'ErrorEvent'; payload: ErrorEvent }
  | { type: 'Disconnected'; payload: Record<string, never> }
  | { type: string; payload: unknown }
