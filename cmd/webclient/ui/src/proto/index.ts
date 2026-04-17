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
  tradition?: string
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
  coverTier?: string  // "lesser" | "standard" | "greater" | "" (not cover)
}

export interface ResistanceEntry {
  damageType?: string
  damage_type?: string
  value?: number
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

export interface ProficiencyEntry {
  category?: string
  name?: string
  rank?: string
  bonus?: number
  kind?: string
}

export interface FeatEntry {
  featId?: string
  name: string
  category?: string
  active?: boolean
  description?: string
  activateText?: string
  isReaction?: boolean
  armorCategory?: string
}

export interface PreparedSlotView {
  techId?: string
  tech_id?: string
  expended?: boolean
  techName?: string
  tech_name?: string
  description?: string
  effectsSummary?: string
  shortName?: string
  short_name?: string
  techLevel?: number
  tech_level?: number
}

export interface InnateSlotView {
  techId?: string
  tech_id?: string
  usesRemaining?: number
  uses_remaining?: number
  maxUses?: number
  max_uses?: number
  techName?: string
  tech_name?: string
  description?: string
  isReaction?: boolean
  effectsSummary?: string
  shortName?: string
  short_name?: string
  passive?: boolean
  techLevel?: number
  tech_level?: number
}

export interface HardwiredSlotView {
  techId?: string
  tech_id?: string
  techName?: string
  tech_name?: string
  description?: string
  effectsSummary?: string
  shortName?: string
  short_name?: string
  techLevel?: number
  tech_level?: number
}

export interface SpontaneousKnownEntry {
  techId?: string
  tech_id?: string
  techName?: string
  tech_name?: string
  techLevel?: number
  tech_level?: number
  description?: string
  effectsSummary?: string
  shortName?: string
  short_name?: string
}

export interface SpontaneousUsePoolView {
  techLevel?: number
  tech_level?: number
  usesRemaining?: number
  uses_remaining?: number
  maxUses?: number
  max_uses?: number
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
  awareness?: number
  playerResistances?: ResistanceEntry[]
  player_resistances?: ResistanceEntry[]
  playerWeaknesses?: ResistanceEntry[]
  player_weaknesses?: ResistanceEntry[]
  experience?: number
  xpToNext?: number
  xp_to_next?: number
  pendingBoosts?: number
  pending_boosts?: number
  pendingSkillIncreases?: number
  pending_skill_increases?: number
  heroPoints?: number
  hero_points?: number
  className?: string
  class_name?: string
  preparedSlots?: PreparedSlotView[]
  prepared_slots?: PreparedSlotView[]
  innateSlots?: InnateSlotView[]
  innate_slots?: InnateSlotView[]
  hardwiredSlots?: HardwiredSlotView[]
  hardwired_slots?: HardwiredSlotView[]
  spontaneousKnown?: SpontaneousKnownEntry[]
  spontaneous_known?: SpontaneousKnownEntry[]
  spontaneousUsePools?: SpontaneousUsePoolView[]
  spontaneous_use_pools?: SpontaneousUsePoolView[]
  focusPoints?: number
  focus_points?: number
  maxFocusPoints?: number
  max_focus_points?: number
  proficiencies?: ProficiencyEntry[]
  armorCategories?: Record<string, string>
  armor_categories?: Record<string, string>
  proficiencyAcBonus?: number
  proficiency_ac_bonus?: number
  effectiveArmorCategory?: string
  effective_armor_category?: string
  // Attack bonus breakdown components (REQ-WEC-71).
  mainHandAbilityBonus?: number
  main_hand_ability_bonus?: number
  mainHandProfBonus?: number
  main_hand_prof_bonus?: number
  mainHandProfRank?: string
  main_hand_prof_rank?: string
  offHandAbilityBonus?: number
  off_hand_ability_bonus?: number
  offHandProfBonus?: number
  off_hand_prof_bonus?: number
  offHandProfRank?: string
  off_hand_prof_rank?: string
  exploreMode?: string
  explore_mode?: string
  techTradition?: string
  tech_tradition?: string
}

export interface QuestObjectiveView {
  id?: string
  description?: string
  current?: number
  required?: number
}

export interface QuestEntryView {
  questId?: string
  quest_id?: string
  title?: string
  description?: string
  xpReward?: number
  xp_reward?: number
  creditsReward?: number
  credits_reward?: number
  objectives?: QuestObjectiveView[]
  status?: string // "available" | "active" | "completed" | "locked"
}

export interface QuestGiverView {
  npcName?: string
  npc_name?: string
  npcInstanceId?: string
  npc_instance_id?: string
  quests?: QuestEntryView[]
}

export interface QuestLogView {
  quests?: QuestEntryView[]
}

export interface QuestCompleteEvent {
  questId?: string
  quest_id?: string
  title?: string
  xpReward?: number
  xp_reward?: number
  creditsReward?: number
  credits_reward?: number
  itemRewards?: string[]
  item_rewards?: string[]
}

export interface InventoryItem {
  instanceId?: string
  name: string
  kind?: string
  quantity?: number
  weight?: number
  itemDefId?: string
  item_def_id?: string
  armorSlot?: string
  armor_slot?: string
  armorCategory?: string
  armor_category?: string
  effectsSummary?: string
  effects_summary?: string
  throwable?: boolean
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
  totalCrypto?: number
}

export interface PoiWithNpc {
  poiId?: string
  poi_id?: string
  npcName?: string
  npc_name?: string
}

export interface ZoneExitInfo {
  direction?: string
  destZoneId?: string
  dest_zone_id?: string
  destZoneName?: string
  dest_zone_name?: string
}

export interface SameZoneExitTarget {
  direction?: string
  targetRoomId?: string
  target_room_id?: string
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
  poiNpcs?: PoiWithNpc[]
  poi_npcs?: PoiWithNpc[]
  zoneExits?: ZoneExitInfo[]
  zone_exits?: ZoneExitInfo[]
  sameZoneExitTargets?: SameZoneExitTarget[]
  same_zone_exit_targets?: SameZoneExitTarget[]
}

export interface WorldZoneTile {
  zoneId?: string
  zoneName?: string
  worldX?: number
  worldY?: number
  discovered?: boolean
  current?: boolean
  dangerLevel?: string
  danger_level?: string
}

export interface MapResponse {
  tiles?: MapTile[]
  worldTiles?: WorldZoneTile[]
  zoneName?: string
}

export interface HealerView {
  npcName?: string
  npc_name?: string
  description?: string
  pricePerHp?: number
  price_per_hp?: number
  missingHp?: number
  missing_hp?: number
  fullHealCost?: number
  full_heal_cost?: number
  capacityRemaining?: number
  capacity_remaining?: number
  playerCurrency?: number
  player_currency?: number
  currentHp?: number
  current_hp?: number
  maxHp?: number
  max_hp?: number
}

export interface RestView {
  npcName?: string
  npc_name?: string
  description?: string
  npcType?: string
  npc_type?: string
  restCost?: number
  rest_cost?: number
  playerCurrency?: number
  player_currency?: number
  currentHp?: number
  current_hp?: number
  maxHp?: number
  max_hp?: number
}

export interface JobOfferEntry {
  jobId?: string
  job_id?: string
  jobName?: string
  job_name?: string
  trainingCost?: number
  training_cost?: number
  available?: boolean
  unavailableReason?: string
  unavailable_reason?: string
  alreadyTrained?: boolean
  already_trained?: boolean
}

export interface TrainerView {
  npcName?: string
  npc_name?: string
  description?: string
  jobs?: JobOfferEntry[]
  playerCurrency?: number
  player_currency?: number
}

export interface FixerView {
  npcName?: string
  npc_name?: string
  description?: string
  currentWanted?: number
  current_wanted?: number
  maxWanted?: number
  max_wanted?: number
  playerCurrency?: number
  player_currency?: number
  bribeCosts?: Record<number, number>
  bribe_costs?: Record<number, number>
}

export interface TechOfferEntry {
  techId?: string
  tech_id?: string
  techName?: string
  tech_name?: string
  description?: string
  cost?: number
  techLevel?: number
  tech_level?: number
}

export interface TechTrainerView {
  npcName?: string
  npc_name?: string
  tradition?: string
  offers?: TechOfferEntry[]
  playerCurrency?: number
  player_currency?: number
}

export interface ShopItem {
  name?: string
  itemId?: string
  item_id?: string
  buyPrice?: number
  buy_price?: number
  sellPrice?: number
  sell_price?: number
  stock?: number
  kind?: string
  description?: string
  // Weapon stats (when kind == "weapon")
  weaponDamage?: string
  weapon_damage?: string
  weaponDamageType?: string
  weapon_damage_type?: string
  weaponRange?: number
  weapon_range?: number
  weaponTraits?: string[]
  weapon_traits?: string[]
  // Armor stats (when kind == "armor")
  armorAcBonus?: number
  armor_ac_bonus?: number
  armorSlot?: string
  armor_slot?: string
  armorCheckPenalty?: number
  armor_check_penalty?: number
  armorSpeedPenalty?: number
  armor_speed_penalty?: number
  armorProfCategory?: string
  armor_prof_category?: string
  // Consumable stats (when kind == "consumable")
  effectsSummary?: string
  effects_summary?: string
}

export interface ShopView {
  npcName?: string
  npc_name?: string
  items?: ShopItem[]
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

export interface CombatantPosition {
  name: string
  x: number
  y: number
  apRemaining?: number
  ap_remaining?: number
  apTotal?: number
  ap_total?: number
  hpCurrent?: number
  hp_current?: number
  hpMax?: number
  hp_max?: number
}

export interface APUpdateEvent {
  name?: string
  apRemaining?: number
  ap_remaining?: number
  apTotal?: number
  ap_total?: number
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
  attackerPosition?: number  // deprecated
  attackerX?: number
  attackerY?: number
  flanking?: boolean
}

export interface RoundStartEvent {
  round?: number
  actionsPerTurn?: number
  actions_per_turn?: number
  durationMs?: number
  turnOrder?: string[]
  turn_order?: string[]
  initialPositions?: CombatantPosition[]
  gridWidth?: number
  grid_width?: number
  gridHeight?: number
  grid_height?: number
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

export interface WeatherEvent {
  weatherName?: string
  weather_name?: string
  active?: boolean
  description?: string
}

export interface JobFeatGrant {
  grantLevel?: number
  grant_level?: number
  featId?: string
  feat_id?: string
  featName?: string
  feat_name?: string
}

export interface FeatOption {
  featId?: string
  feat_id?: string
  name?: string
  description?: string
  category?: string
}

export interface PendingFeatChoice {
  grantLevel?: number
  grant_level?: number
  count?: number
  options?: FeatOption[]
}

export interface ChooseFeatRequest {
  grantLevel?: number
  grant_level?: number
  featId?: string
  feat_id?: string
}

export interface JobTechGrant {
  grantLevel?: number
  grant_level?: number
  techId?: string
  tech_id?: string
  techName?: string
  tech_name?: string
  techLevel?: number
  tech_level?: number
  techType?: string
  tech_type?: string
}

export interface JobGrantsResponse {
  featGrants?: JobFeatGrant[]
  feat_grants?: JobFeatGrant[]
  techGrants?: JobTechGrant[]
  tech_grants?: JobTechGrant[]
  pendingFeatChoices?: PendingFeatChoice[]
  pending_feat_choices?: PendingFeatChoice[]
}

export interface LoadoutWeaponPreset {
  mainHand?: string
  offHand?: string
  mainHandDamage?: string
  offHandDamage?: string
}

export interface LoadoutView {
  presets?: LoadoutWeaponPreset[]
  activeIndex?: number
}

export interface HotbarSlot {
  kind: string        // "command" | "feat" | "technology" | "throwable" | "consumable"
  ref: string         // command text or item/feat/tech ID
  displayName?: string
  display_name?: string
  description?: string
  usesRemaining?: number
  uses_remaining?: number
  maxUses?: number
  max_uses?: number
  rechargeCondition?: string
  recharge_condition?: string
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

export interface UseRequest {
  itemRef?: string
  targetName?: string
  target_name?: string
  target_x?: number // 0-based grid column; -1 (or omit) means unset / no AoE
  target_y?: number // 0-based grid row; -1 (or omit) means unset / no AoE
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
  | { type: 'WeatherEvent'; payload: WeatherEvent }
  | { type: 'JobGrantsResponse'; payload: JobGrantsResponse }
  | { type: 'APUpdateEvent'; payload: APUpdateEvent }
  | { type: 'QuestGiverView'; payload: QuestGiverView }
  | { type: 'QuestLogView'; payload: QuestLogView }
  | { type: 'QuestCompleteEvent'; payload: QuestCompleteEvent }
  | { type: string; payload: unknown }
