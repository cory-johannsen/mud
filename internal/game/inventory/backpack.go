package inventory

import (
	"fmt"

	"github.com/google/uuid"
)

// ItemInstance represents a concrete instance of an item in a backpack.
type ItemInstance struct {
	InstanceID string
	ItemDefID  string
	Quantity   int
}

// Backpack is a container with slot and weight limits.
type Backpack struct {
	MaxSlots  int
	MaxWeight float64
	items     []ItemInstance
}

// NewBackpack creates a Backpack with the given limits.
//
// Precondition: maxSlots >= 0 and maxWeight >= 0.
// Postcondition: returned Backpack has zero items and the specified limits.
func NewBackpack(maxSlots int, maxWeight float64) *Backpack {
	return &Backpack{
		MaxSlots:  maxSlots,
		MaxWeight: maxWeight,
	}
}

// Add places quantity units of the given item into the backpack.
// It is atomic: if limits would be exceeded, no state is modified.
//
// Precondition: quantity > 0, itemDefID exists in reg.
// Postcondition: on success, items are added without exceeding slot or weight limits;
// on error, backpack state is unchanged.
func (b *Backpack) Add(itemDefID string, quantity int, reg *Registry) (*ItemInstance, error) {
	def, ok := reg.Item(itemDefID)
	if !ok {
		return nil, fmt.Errorf("backpack: unknown item %q", itemDefID)
	}
	if quantity <= 0 {
		return nil, fmt.Errorf("backpack: quantity must be > 0")
	}

	addedWeight := float64(quantity) * def.Weight
	currentWeight := b.TotalWeight(reg)

	if currentWeight+addedWeight > b.MaxWeight {
		return nil, fmt.Errorf("backpack: adding %d of %q would exceed weight limit (%.2f + %.2f > %.2f)",
			quantity, itemDefID, currentWeight, addedWeight, b.MaxWeight)
	}

	if def.Stackable {
		return b.addStackable(def, quantity)
	}
	return b.addNonStackable(def, quantity)
}

func (b *Backpack) addStackable(def *ItemDef, quantity int) (*ItemInstance, error) {
	// Phase 1: compute how much fits into existing stacks and how many new slots needed.
	remaining := quantity
	var mergeIdx int = -1
	var mergeAmount int

	for i := range b.items {
		if remaining <= 0 {
			break
		}
		if b.items[i].ItemDefID == def.ID && b.items[i].Quantity < def.MaxStack {
			room := def.MaxStack - b.items[i].Quantity
			take := remaining
			if take > room {
				take = room
			}
			if mergeIdx == -1 {
				mergeIdx = i
				mergeAmount = take
			}
			remaining -= take
		}
	}

	// Calculate new slots needed for the remainder.
	newSlots := 0
	rem := remaining
	for rem > 0 {
		newSlots++
		if rem > def.MaxStack {
			rem -= def.MaxStack
		} else {
			rem = 0
		}
	}

	if len(b.items)+newSlots > b.MaxSlots {
		return nil, fmt.Errorf("backpack: not enough slots")
	}

	// Phase 2: apply. Merge into first available stack.
	var result *ItemInstance
	if mergeIdx >= 0 && mergeAmount > 0 {
		b.items[mergeIdx].Quantity += mergeAmount
		result = &b.items[mergeIdx]
	}

	// Create new stacks for remainder.
	rem = remaining
	for rem > 0 {
		q := rem
		if q > def.MaxStack {
			q = def.MaxStack
		}
		inst := ItemInstance{
			InstanceID: uuid.New().String(),
			ItemDefID:  def.ID,
			Quantity:   q,
		}
		b.items = append(b.items, inst)
		result = &b.items[len(b.items)-1]
		rem -= q
	}

	if result == nil {
		// All went into merge (shouldn't happen given quantity > 0, but guard).
		result = &b.items[mergeIdx]
	}
	return result, nil
}

func (b *Backpack) addNonStackable(def *ItemDef, quantity int) (*ItemInstance, error) {
	if len(b.items)+quantity > b.MaxSlots {
		return nil, fmt.Errorf("backpack: not enough slots for %d non-stackable items", quantity)
	}

	var last *ItemInstance
	for i := 0; i < quantity; i++ {
		inst := ItemInstance{
			InstanceID: uuid.New().String(),
			ItemDefID:  def.ID,
			Quantity:   1,
		}
		b.items = append(b.items, inst)
		last = &b.items[len(b.items)-1]
	}
	return last, nil
}

// Remove removes quantity units from the instance identified by instanceID.
//
// Precondition: instanceID exists in the backpack, quantity > 0 and <= instance.Quantity.
// Postcondition: if quantity == instance.Quantity, instance is removed; otherwise quantity is decremented.
func (b *Backpack) Remove(instanceID string, quantity int) error {
	for i := range b.items {
		if b.items[i].InstanceID == instanceID {
			if quantity > b.items[i].Quantity {
				return fmt.Errorf("backpack: cannot remove %d from instance with quantity %d",
					quantity, b.items[i].Quantity)
			}
			if quantity == b.items[i].Quantity {
				b.items = append(b.items[:i], b.items[i+1:]...)
			} else {
				b.items[i].Quantity -= quantity
			}
			return nil
		}
	}
	return fmt.Errorf("backpack: instance %q not found", instanceID)
}

// Items returns a snapshot copy of all items in the backpack.
//
// Postcondition: returned slice is a copy; mutations do not affect the backpack.
func (b *Backpack) Items() []ItemInstance {
	out := make([]ItemInstance, len(b.items))
	copy(out, b.items)
	return out
}

// UsedSlots returns the number of occupied slots.
//
// Postcondition: result >= 0 and <= MaxSlots.
func (b *Backpack) UsedSlots() int {
	return len(b.items)
}

// TotalWeight returns the sum of quantity*weight for all items.
//
// Precondition: all ItemDefIDs in the backpack exist in reg.
// Postcondition: result >= 0.
func (b *Backpack) TotalWeight(reg *Registry) float64 {
	var total float64
	for _, inst := range b.items {
		if def, ok := reg.Item(inst.ItemDefID); ok {
			total += float64(inst.Quantity) * def.Weight
		}
	}
	return total
}

// FindByItemDefID returns all instances matching the given item definition ID.
//
// Postcondition: returned slice is a copy.
func (b *Backpack) FindByItemDefID(itemDefID string) []ItemInstance {
	var out []ItemInstance
	for _, inst := range b.items {
		if inst.ItemDefID == itemDefID {
			out = append(out, inst)
		}
	}
	return out
}
