import { InventoryDrawer } from './InventoryDrawer'
import { EquipmentDrawer } from './EquipmentDrawer'
import { SkillsDrawer } from './SkillsDrawer'
import { FeatsDrawer } from './FeatsDrawer'
import { StatsDrawer } from './StatsDrawer'
import { TechnologyDrawer } from './TechnologyDrawer'
import { JobDrawer } from './JobDrawer'
import { ExploreDrawer } from './ExploreDrawer'
import { QuestsDrawer } from './QuestsDrawer'
import { EffectsDrawer } from './EffectsDrawer'

export type DrawerType = 'inventory' | 'equipment' | 'skills' | 'feats' | 'stats' | 'technology' | 'job' | 'explore' | 'quests' | 'effects'

interface Props {
  openDrawer: DrawerType | null
  onClose: () => void
}

export function DrawerContainer({ openDrawer, onClose }: Props) {
  if (openDrawer === null) return null

  return (
    <>
      <div className="drawer-backdrop" onClick={onClose} />
      <div className="drawer-panel">
        {openDrawer === 'inventory'   && <InventoryDrawer onClose={onClose} />}
        {openDrawer === 'equipment'   && <EquipmentDrawer onClose={onClose} />}
        {openDrawer === 'skills'      && <SkillsDrawer onClose={onClose} />}
        {openDrawer === 'feats'       && <FeatsDrawer onClose={onClose} />}
        {openDrawer === 'stats'       && <StatsDrawer onClose={onClose} />}
        {openDrawer === 'technology'  && <TechnologyDrawer onClose={onClose} />}
        {openDrawer === 'job'         && <JobDrawer onClose={onClose} />}
        {openDrawer === 'explore'     && <ExploreDrawer onClose={onClose} />}
        {openDrawer === 'quests'      && <QuestsDrawer onClose={onClose} />}
        {openDrawer === 'effects'     && <EffectsDrawer onClose={onClose} />}
      </div>
    </>
  )
}
