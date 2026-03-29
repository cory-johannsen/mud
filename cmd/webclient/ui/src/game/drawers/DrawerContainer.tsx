import { InventoryDrawer } from './InventoryDrawer'
import { EquipmentDrawer } from './EquipmentDrawer'
import { SkillsDrawer } from './SkillsDrawer'
import { FeatsDrawer } from './FeatsDrawer'

type DrawerType = 'inventory' | 'equipment' | 'skills' | 'feats'

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
        {openDrawer === 'inventory' && <InventoryDrawer onClose={onClose} />}
        {openDrawer === 'equipment' && <EquipmentDrawer onClose={onClose} />}
        {openDrawer === 'skills'    && <SkillsDrawer onClose={onClose} />}
        {openDrawer === 'feats'     && <FeatsDrawer onClose={onClose} />}
      </div>
    </>
  )
}
