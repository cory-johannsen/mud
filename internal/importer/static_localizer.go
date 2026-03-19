package importer

import (
	"context"

	"github.com/cory-johannsen/mud/internal/game/technology"
)

// staticTransform holds the localized Name and Description for a technology ID.
type staticTransform struct {
	Name        string
	Description string
}

// StaticLocalizer applies pre-computed lore transforms, producing output
// byte-identical to the committed technology YAML files.
// If an ID has no entry in the table, the original values are preserved.
type StaticLocalizer struct{}

// Localize applies the static transform for def.ID if one exists.
func (StaticLocalizer) Localize(_ context.Context, def *technology.TechnologyDef) error {
	if t, ok := staticTransforms[def.ID]; ok {
		def.Name = t.Name
		def.Description = t.Description
	}
	return nil
}

//nolint:lll
var staticTransforms = map[string]staticTransform{
	`500_toads_bio_synthetic`: {Name: `500 Toads`, Description: `You conjure hundreds of magical toads to fill the area and hop around. The vast quantity of hopping toads provides enough weight and height for the creatures to trigger any potential trap in the area that could be triggered by the weight, movement, or position of a Medium creature. The area is difficult terrain, and the magic reconstructs any toads destroyed by traps to keep the area full of toads for the duration of the spell.`},
	`500_toads_technical`: {Name: `Swarm Unit Deploy`, Description: `Release a cloud of micro-drone units that blanket the area, disrupting movement and targeting systems.`},
	`acid_spit`: {Name: `Acid Spit`, Description: `A bio-synthetic gland secretes pressurized corrosive fluid at a single target.`},
	`acid_splash_bio_synthetic`: {Name: `Bio-Acid Spit`, Description: `You spit a concentrated glob of bio-synthesized acid that splatters on impact. Make a tech attack. On a hit, deal 1d6 acid damage plus 1 splash acid damage to nearby targets.`},
	`acid_spray`: {Name: `Acid Spray`, Description: `A bio-synthetic secretion that coats all nearby enemies in corrosive fluid.`},
	`acidic_burst_bio_synthetic`: {Name: `Acidic Burst`, Description: `You create a shell of acid around yourself that immediately bursts outward, dealing 2d6 acid damage to each creature in the area with a basic Reflex save.Heightened (+1) The damage increases by 2d6.`},
	`acidic_burst_technical`: {Name: `Corrosive Payload`, Description: `Detonate a pressurized canister of industrial acid, coating the target zone in caustic residue.`},
	`admonishing_ray_fanatic_doctrine`: {Name: `Admonishing Ray`, Description: `A ray of energy bludgeons your target into submission without causing lasting harm. When you cast this spell, you choose whether the ray feels like a strong punch or slap. Make a spell attack roll. The ray deals 2d6 bludgeoning damage.

Critical Success The target takes double damage.
Success The target takes full damage.

Heightened (+1) The damage increases by 2d6.`},
	`admonishing_ray_technical`: {Name: `Compliance Beam`, Description: `Fire a calibrated electromagnetic pulse that shocks the target into compliance without lethal force.`},
	`agitate_neural`: {Name: `Agitate`, Description: `You send the target's mind and body into overdrive, forcing it to become restless and hyperactive. During the duration, the target must Stride, Fly, or Swim at least once each turn or take 2d8 mental damage at the end of its turn. The GM might decide to add additional move actions to the list for creatures that possess only a more unusual form of movement. The duration of this effect depends on the target's Will save.Critical Success The spell has no effect.
Success The duration is 1 round.
Failure The duration is 2 rounds.
Critical Failure The duration is 4 rounds.Heightened (+1) The damage increases by 2d8.`},
	`agitate_technical`: {Name: `Feedback Overload`, Description: `Broadcast a high-frequency signal that overloads sensory augmentations and drives targets into agitation.`},
	`air_bubble_bio_synthetic`: {Name: `Air Bubble`, Description: `Trigger A creature within range enters an environment where it can't breathe.
A bubble of pure air appears around the target's head, allowing it to breathe normally. The effect ends as soon as the target returns to an environment where it can breathe normally.`},
	`air_bubble_fanatic_doctrine`: {Name: `Air Bubble`, Description: `Trigger A creature within range enters an environment where it can't breathe.
A bubble of pure air appears around the target's head, allowing it to breathe normally. The effect ends as soon as the target returns to an environment where it can breathe normally.`},
	`air_bubble_technical`: {Name: `Emergency Atmos Pack`, Description: `Deploy a portable atmospheric filter that provides clean breathable air in contaminated environments.`},
	`airburst_bio_synthetic`: {Name: `Airburst`, Description: `A blast of wind wildly pushes everything nearby. Unattended objects of 1 Bulk or less are pushed 5 feet away from you. Large or smaller creatures must attempt a Fortitude save.

Critical Success The creature is unaffected.
Success The creature takes a -2 status penalty to checks made during its reactions until the end of your turn.
Failure As success, and the creature is pushed 5 feet away from you.
Critical Failure The creature is pushed 5 feet away from you and can't use reactions until the end of your turn.

Heightened (4th) Increase the area to a 10-foot emanation and increase the distance objects and creatures are pushed to 10 feet.`},
	`airburst_technical`: {Name: `Concussive Drone Strike`, Description: `Trigger an aerial micro-explosive at optimal altitude, generating a targeted pressure wave.`},
	`alarm_bio_synthetic`: {Name: `Alarm`, Description: `You ward an area to alert you when creatures enter without your permission. When you cast alarm, select a password. Whenever a Small or larger corporeal creature enters the spell's area without speaking the password, alarm sends your choice of a mental alert (in which case the spell gains the mental trait) or an audible alarm with the sound and volume of a hand bell (in which case the spell gains the auditory trait). Either option automatically awakens you, and the bell allows each creature in the area to attempt a @Check[perception|dc:15] check to wake up. A creature aware of the alarm must succeed at a Stealth check against the spell's DC or trigger the spell when moving into the area.

Heightened (3rd) You can specify a trigger for which types of creatures sound the alarm spell.`},
	`alarm_fanatic_doctrine`: {Name: `Alarm`, Description: `You ward an area to alert you when creatures enter without your permission. When you cast alarm, select a password. Whenever a Small or larger corporeal creature enters the spell's area without speaking the password, alarm sends your choice of a mental alert (in which case the spell gains the mental trait) or an audible alarm with the sound and volume of a hand bell (in which case the spell gains the auditory trait). Either option automatically awakens you, and the bell allows each creature in the area to attempt a @Check[perception|dc:15] check to wake up. A creature aware of the alarm must succeed at a Stealth check against the spell's DC or trigger the spell when moving into the area.

Heightened (3rd) You can specify a trigger for which types of creatures sound the alarm spell.`},
	`alarm_neural`: {Name: `Alarm`, Description: `You ward an area to alert you when creatures enter without your permission. When you cast alarm, select a password. Whenever a Small or larger corporeal creature enters the spell's area without speaking the password, alarm sends your choice of a mental alert (in which case the spell gains the mental trait) or an audible alarm with the sound and volume of a hand bell (in which case the spell gains the auditory trait). Either option automatically awakens you, and the bell allows each creature in the area to attempt a @Check[perception|dc:15] check to wake up. A creature aware of the alarm must succeed at a Stealth check against the spell's DC or trigger the spell when moving into the area.

Heightened (3rd) You can specify a trigger for which types of creatures sound the alarm spell.`},
	`alarm_technical`: {Name: `Perimeter Sensor Grid`, Description: `Lay down a network of motion-activated sensors that alert on unauthorized entry.`},
	`ancient_dust_fanatic_doctrine`: {Name: `Doctrine's Reckoning`, Description: `You exhale a cloud of void-charged particulate — the Doctrine's judgment made tangible. Each creature in the area takes void damage equal to your doctrine modifier and 1 persistent void damage depending on their Fortitude save.`},
	`animal_allies`: {Name: `Animal Allies`, Description: `You summon tiny, ordinary animals from the environment, such as insects, birds, or fish, to quickly lash out at nearby foes. The animals swarm around the creatures in the area, dealing each of them 3d4 piercing damage with a basic Reflex save.

Heightened (+1) The damage increases by 3d4`},
	`animate_rope_neural`: {Name: `Animate Rope`, Description: `You cause a length or section of @UUID[Compendium.pf2e.equipment-srd.Item.Rope] or a rope-like object to animate and follow simple commands. You can give it two commands when you Cast the Spell, and one command each time you Sustain the Spell.


Bind (attack) The Rope attempts to partially bind a creature. Attempt a spell attack roll against the target's Reflex DC. If you succeed, the target takes a –10-foot circumstance penalty to its Speed (-20-foot on a critical success). This ends if the target Escapes against your spell DC or breaks the Rope. (A standard adventuring Rope has Hardness 2, HP 8, and a Broken Threshold of 4.) @UUID[Compendium.pf2e.spell-effects.Item.Spell Effect: Animate Rope]
Coil The Rope forms a tidy, coiled stack.
Crawl The Rope inches along the ground like a snake, moving one of its ends 10 feet. The Rope must move along a surface, but that surface doesn't need to be horizontal.
Knot The Rope ties a sturdy knot in itself.
Loop The Rope forms a simple loop at one or both ends, or straightens itself back out.
Tie The Rope ties itself around a willing creature or an object that's unattended or attended by a willing creature.
Undo The Rope undoes one of its knots, ties, or bindings.


Heightened (+2) The range increases by 50 feet, and you can animate 50 more feet of Rope.`},
	`animate_rope_technical`: {Name: `Servo-Cable Actuator`, Description: `Activate embedded servo motors in a cable, allowing remote mechanical manipulation.`},
	`ant_haul_bio_synthetic`: {Name: `Ant Haul`, Description: `You reinforce the target's musculoskeletal system to bear more weight. The target can carry 3 more Bulk than normal before becoming @UUID[Compendium.pf2e.conditionitems.Item.Encumbered] and up to a maximum of 6 more Bulk.
@UUID[Compendium.pf2e.spell-effects.Item.Spell Effect: Ant Haul]`},
	`ant_haul_technical`: {Name: `Load-Bearing Exo-Frame`, Description: `Activate an exoskeletal harness that dramatically increases the user's carrying capacity.`},
	`anticipate_peril_neural`: {Name: `Anticipate Peril`, Description: `You grant the target brief foresight. The target gains a +1 status bonus to its next initiative roll, after which the spell ends.

Heightened (+2) The status bonus increases by 1, to a maximum of +4 at 7th rank.
@UUID[Compendium.pf2e.spell-effects.Item.Spell Effect: Anticipate Peril]`},
	`anticipate_peril_technical`: {Name: `Threat Prediction Algorithm`, Description: `Run a combat prediction subroutine that flags incoming hazards before they materialize.`},
	`approximate_bio_synthetic`: {Name: `Chemical Census Sweep`, Description: `You release a micro-sensor sweep to catalog and count specific compounds or biological objects in the area.`},
	`approximate_fanatic_doctrine`: {Name: `Doctrine's Census`, Description: `The Doctrine illuminates the area with analytical clarity. Name a type of object; you instantly receive an estimate of how many are visible within the zone. The count rounds to the largest digit — the Doctrine deals in approximations, not accountancy.`},
	`approximate_neural`: {Name: `Rapid Scan`, Description: `You sweep the area with enhanced sensory processing, cataloging visible objects of a chosen type in an instant. The count is rounded to the largest digit — precision analysis requires more time.`},
	`aqueous_blast_bio_synthetic`: {Name: `Aqueous Blast`, Description: `You evoke a mass of water into the air around your outstretched fist. For the remainder of your turn, you can blast targets within 30 feet with this water by spending a single action which has the attack and concentrate traits. When you do so, attempt a ranged spell attack roll. If you hit, you inflict 2d8 bludgeoning damage. On a critical hit, the blast knocks the target @UUID[Compendium.pf2e.conditionitems.Item.Prone].

Heightened (+1) The damage increases by 1d8.`},
	`aqueous_blast_neural`: {Name: `Aqueous Blast`, Description: `You evoke a mass of water into the air around your outstretched fist. For the remainder of your turn, you can blast targets within 30 feet with this water by spending a single action which has the attack and concentrate traits. When you do so, attempt a ranged spell attack roll. If you hit, you inflict 2d8 bludgeoning damage. On a critical hit, the blast knocks the target @UUID[Compendium.pf2e.conditionitems.Item.Prone].

Heightened (+1) The damage increases by 1d8.`},
	`aqueous_blast_technical`: {Name: `Hydraulic Impact Round`, Description: `Fire a high-pressure water slug from a compact launcher, delivering blunt force at range.`},
	`arc_lights`: {Name: `Arc Lights`, Description: `Projects three hovering electromagnetic arc-light drones that illuminate and disorient.`},
	`armor_of_thorn_and_claw`: {Name: `Armor of Thorn and Claw`, Description: `Razor-sharp thorns and claws erupt from your skin or scales. Whenever a creature touches you or hits you with a melee unarmed attack, it takes 1 piercing damage. Additionally, if you become @UUID[Compendium.pf2e.conditionitems.Item.Grabbed], @UUID[Compendium.pf2e.conditionitems.Item.Restrained], or otherwise held @UUID[Compendium.pf2e.conditionitems.Item.Immobilized] in a creature's grasp, such as by being engulfed or swallowed, the creature takes @Damage[(ceil(@item.rank / 2))d4[persistent,bleed]] damage.Heightened (+2) The piercing damage increases by 2, and the persistent bleed damage increases by 1d4.`},
	`atmospheric_surge`: {Name: `Atmospheric Surge`, Description: `A wrist-mounted atmospheric compressor discharges a powerful wind blast that scatters enemies.`},
	`bane_fanatic_doctrine`: {Name: `Bane`, Description: `You fill the minds of your enemies with doubt. Enemies in the area must succeed at a Will save or take a –1 status penalty to attack rolls as long as they are in the area. Once per round on subsequent turns, you can Sustain the spell to increase the emanation's radius by 10 feet and force enemies in the area that weren't yet affected to attempt another saving throw.
Bane can counteract @UUID[Compendium.pf2e.spells-srd.Item.Bless].
@UUID[Compendium.pf2e.spell-effects.Item.Spell Effect: Bane]`},
	`bane_neural`: {Name: `Bane`, Description: `You fill the minds of your enemies with doubt. Enemies in the area must succeed at a Will save or take a –1 status penalty to attack rolls as long as they are in the area. Once per round on subsequent turns, you can Sustain the spell to increase the emanation's radius by 10 feet and force enemies in the area that weren't yet affected to attempt another saving throw.
Bane can counteract @UUID[Compendium.pf2e.spells-srd.Item.Bless].
@UUID[Compendium.pf2e.spell-effects.Item.Spell Effect: Bane]`},
	`battle_fervor`: {Name: `Battle Fervor`, Description: `A surge of doctrinal conviction that sharpens combat focus.`},
	`befuddle_neural`: {Name: `Befuddle`, Description: `You sow seeds of confusion in your target's mind, causing its actions and thoughts to become clumsy.Critical Success The target is unaffected.
Success The target is Clumsy 1 and Stupefied 1.
Failure The target is Clumsy 2 and Stupefied 2.
Critical Failure The target is Clumsy 3, Stupefied 3, and @UUID[Compendium.pf2e.conditionitems.Item.Confused].`},
	`befuddle_technical`: {Name: `Cognitive Jam Signal`, Description: `Emit a tight-band neural disruptor that scrambles short-term cognition in the target.`},
	`benediction`: {Name: `Benediction`, Description: `Divine protection helps protect your companions. You and your allies gain a +1 status bonus to AC while within the emanation. Once per round on subsequent turns, you can Sustain the spell to increase the emanation's radius by 10 feet.
Benediction can counteract malediction.
@UUID[Compendium.pf2e.spell-effects.Item.Spell Effect: Benediction]`},
	`beseech_the_sphinx_fanatic_doctrine`: {Name: `Beseech the Sphinx`, Description: `You look to the great Sphinx constellation, wisest of all cosmic guides and favored of Phimater, asking them to lend their insight to the target. Choose one skill and one type of saving throw (Fortitude, Reflex, or Will). The target gains a +1 status bonus to those skill checks and saving throws for the duration.Heightened (4th) The status bonus increases to +2.
Heightened (7th) The status bonus increases to +3.
@UUID[Compendium.pf2e.spell-effects.Item.Spell Effect: Beseech the Sphinx]`},
	`beseech_the_sphinx_neural`: {Name: `Beseech the Sphinx`, Description: `You look to the great Sphinx constellation, wisest of all cosmic guides and favored of Phimater, asking them to lend their insight to the target. Choose one skill and one type of saving throw (Fortitude, Reflex, or Will). The target gains a +1 status bonus to those skill checks and saving throws for the duration.Heightened (4th) The status bonus increases to +2.
Heightened (7th) The status bonus increases to +3.
@UUID[Compendium.pf2e.spell-effects.Item.Spell Effect: Beseech the Sphinx]`},
	`biting_words`: {Name: `Biting Words`, Description: `You entwine magic with your voice, causing your taunts and jibes to physically harm your enemies. You can attack with your words once when you finish Casting the Spell, and can repeat the attack once on each of your subsequent turns by taking a single action, which has the attack, concentrate, and linguistic traits. After your third attack total, the spell ends.
When you attack with biting words, make a ranged spell attack roll against a creature within 30 feet, dealing 2d6 sonic damage if you hit (or double damage on a critical hit).

Heightened (+1) The damage increases by 2d6.`},
	`blackout_pulse`: {Name: `Blackout Pulse`, Description: `Emits a localized EM burst that kills electronic lighting and blinds optical sensors in a small radius.`},
	`bless_fanatic_doctrine`: {Name: `Bless`, Description: `Blessings from beyond help your companions strike true. You and your allies gain a +1 status bonus to attack rolls while within the emanation. Once per round on subsequent turns, you can Sustain the spell to increase the emanation's radius by 10 feet.
Bless can counteract @UUID[Compendium.pf2e.spells-srd.Item.Bane].
@UUID[Compendium.pf2e.spell-effects.Item.Aura: Bless]`},
	`bless_neural`: {Name: `Bless`, Description: `Blessings from beyond help your companions strike true. You and your allies gain a +1 status bonus to attack rolls while within the emanation. Once per round on subsequent turns, you can Sustain the spell to increase the emanation's radius by 10 feet.
Bless can counteract @UUID[Compendium.pf2e.spells-srd.Item.Bane].
@UUID[Compendium.pf2e.spell-effects.Item.Aura: Bless]`},
	`bramble_bush_bio_synthetic`: {Name: `Spike Cluster Burst`, Description: `You trigger rapid deployment of barbed wire or bio-synthetic thorn clusters, dealing piercing damage and creating difficult terrain.`},
	`breadcrumbs_bio_synthetic`: {Name: `Breadcrumbs`, Description: `You protect your target from going astray in hostile territory by tracking where it's already been, helping it deduce where it still needs to go. The target leaves a glittering trail behind it that lasts for the spell's duration. This trail doesn't denote the direction or the order of its path-it merely indicates where the target has moved during the spell's duration.

Heightened (2nd) The duration increases to 8 hours.
Heightened (3rd) The duration increases to last until your next daily preparations.`},
	`breadcrumbs_fanatic_doctrine`: {Name: `Breadcrumbs`, Description: `You protect your target from going astray in hostile territory by tracking where it's already been, helping it deduce where it still needs to go. The target leaves a glittering trail behind it that lasts for the spell's duration. This trail doesn't denote the direction or the order of its path-it merely indicates where the target has moved during the spell's duration.

Heightened (2nd) The duration increases to 8 hours.
Heightened (3rd) The duration increases to last until your next daily preparations.`},
	`breadcrumbs_neural`: {Name: `Breadcrumbs`, Description: `You protect your target from going astray in hostile territory by tracking where it's already been, helping it deduce where it still needs to go. The target leaves a glittering trail behind it that lasts for the spell's duration. This trail doesn't denote the direction or the order of its path-it merely indicates where the target has moved during the spell's duration.

Heightened (2nd) The duration increases to 8 hours.
Heightened (3rd) The duration increases to last until your next daily preparations.`},
	`breadcrumbs_technical`: {Name: `Route Logging Protocol`, Description: `Automatically log GPS waypoints as you travel, creating a traceable return path.`},
	`breathe_fire_bio_synthetic`: {Name: `Breathe Fire`, Description: `A gout of flame sprays from your mouth. You deal 2d6 fire damage to creatures in the area with a basic Reflex save.

Heightened (+1) The damage increases by 2d6.`},
	`breathe_fire_technical`: {Name: `Incendiary Spray Unit`, Description: `Release a burst of ignited accelerant from a compact wrist-mounted nozzle.`},
	`briny_bolt_bio_synthetic`: {Name: `Briny Bolt`, Description: `You hurl a bolt of saltwater from your extended hand. Make a ranged spell attack against a target within range.

Critical Success The creature takes 4d6 bludgeoning damage and is @UUID[Compendium.pf2e.conditionitems.Item.Blinded] for 1 round and @UUID[Compendium.pf2e.conditionitems.Item.Dazzled] for 1 minute as saltwater sprays into its eyes. The creature can spend an Interact action to rub its eyes and end the blinded condition, but not the dazzled condition.
Success The creature takes 2d6 bludgeoning damage and is blinded for 1 round. The creature can spend an Interact action wiping the salt water from its eyes to end the blinded condition.

Heightened (+1) The damage increases by 2d6.`},
	`briny_bolt_technical`: {Name: `Saline Projectile Round`, Description: `Fire a pressurized capsule of electrically conductive saline solution, effective against electronics.`},
	`buffeting_winds`: {Name: `Buffeting Winds`, Description: `You release a quick burst of wind that batters your living opponents without causing them lasting harm, while also blowing undead away. The wind deals 2d4 bludgeoning damage, which is nonlethal against living creatures. Against undead, the winds are more vicious, and the spell loses the nonlethal trait against such creatures. Each creature in the area must attempt a basic Reflex save. On a failure, undead creatures are also knocked back 5 feet (or 10 feet on a critical failure).

Heightened (+1) The damage increases by 2d4.`},
	`bullhorn_fanatic_doctrine`: {Name: `Doctrine Proclamation Amplifier`, Description: `You project your voice with doctrinal authority, making it audible and resonant to all within 500 feet.`},
	`bullhorn_neural`: {Name: `Broadcast Amplifier`, Description: `You extend your vocal range through psionic projection, making your voice clearly audible at extreme distances for the duration.`},
	`buoyant_bubbles_bio_synthetic`: {Name: `Buoyant Bubbles`, Description: `You create a thin layer of foamy bubbles that adhere to the target, causing it to float in water and similar liquids. The target doesn't sink, even if it hasn't succeeded at a Swim check this round; an already-sinking target resurfaces with the bubbles' help over the course of 1 round. If on a plane where the water or liquid has a surface, the bubbles also prevent the target from diving beneath that surface unless it succeeds at a Fortitude save against your spell DC.

Heightened (4th) You can target up to 5 creatures.`},
	`buoyant_bubbles_technical`: {Name: `Hydrophobic Foam Coat`, Description: `Apply a fast-hardening hydrophobic foam that provides temporary buoyancy.`},
	`camel_spit_bio_synthetic`: {Name: `Camel Spit`, Description: `You alter your stomach, esophagus, and tongue to be able to spit partially digested food with force. You can spit at a foe once you finish Casting the Spell and can repeat the attack once on each of your subsequent turns by taking a single action, which has the acid, attack, and concentrate traits. After your third spit attack, the spell ends. When you attack with camel spit, make a ranged spell attack roll against a creature within 15 feet, dealing 1d6 acid damage and causing the target to be @UUID[Compendium.pf2e.conditionitems.Item.Dazzled] for 1 round if you hit. On a critical hit, you deal double damage and the target takes @Damage[@item.level[persistent,acid]] damage.

Heightened (+1) The damage increases by 1d6, and the persistent damage on a critical hit is increased by 1.`},
	`camel_spit_technical`: {Name: `Repellent Spray Launcher`, Description: `Project a sticky, foul compound that degrades target morale and sensor clarity.`},
	`carryall_neural`: {Name: `Carryall`, Description: `A small platform of magical force materializes adjacent to you to carry cargo. It is @UUID[Compendium.pf2e.conditionitems.Item.Invisible] or has a ghostly appearance, is 2 feet in diameter, and follows 5 feet behind you, floating just above the ground. It holds up to 5 Bulk of objects (if they can fit on it). Any objects atop the platform fall to the ground when the spell ends. You can Sustain the spell to move the platform up to 30 feet along the ground, to make it stay in place, or to have it return to you and resume following you. The spell ends if a creature tries to ride atop the platform, if the platform is overloaded, if anyone tries to lift or force the platform higher above the ground, or if you move more than 60 feet away from the platform.Heightened (4th) The platform can carry 10 Bulk, creatures can ride atop it, and it can hover in the air, not just on the ground`},
	`carryall_technical`: {Name: `Telekinetic Load Platform`, Description: `Activate a magnetically levitated cargo platform that follows and carries gear autonomously.`},
	`caustic_blast_bio_synthetic`: {Name: `Acid Cluster Spray`, Description: `You release a cluster of bio-synthesized acid nodules that detonate in a burst, dealing acid damage to everything in the area.`},
	`celestial_accord_fanatic_doctrine`: {Name: `Celestial Accord`, Description: `You intervene in a heated disagreement between two creatures, encouraging them to put aside their differences and find some common ground. The emotional heat of the prior moment becomes only a memory. Each target must make a Will save. A creature currently engaged in combat can't get a result worse than success, and a target that is subject to hostility from any other creature ceases to be affected by celestial accord.

Critical Success The creature is unaffected.
Success The creature is filled with doubt about its own intentions and feels an urge to cooperate with the other. It has a -2 status penalty to attack rolls against the other target for 1 round.
Failure The creature can't make hostile actions against the other target and its attitude toward the other target improves to indifferent for the spell's duration.
Critical Failure As failure, but the creature's attitude toward the other target improves to friendly for the duration and is indifferent thereafter (until something happens to change that attitude normally).`},
	`celestial_accord_neural`: {Name: `Celestial Accord`, Description: `You intervene in a heated disagreement between two creatures, encouraging them to put aside their differences and find some common ground. The emotional heat of the prior moment becomes only a memory. Each target must make a Will save. A creature currently engaged in combat can't get a result worse than success, and a target that is subject to hostility from any other creature ceases to be affected by celestial accord.

Critical Success The creature is unaffected.
Success The creature is filled with doubt about its own intentions and feels an urge to cooperate with the other. It has a -2 status penalty to attack rolls against the other target for 1 round.
Failure The creature can't make hostile actions against the other target and its attitude toward the other target improves to indifferent for the spell's duration.
Critical Failure As failure, but the creature's attitude toward the other target improves to friendly for the duration and is indifferent thereafter (until something happens to change that attitude normally).`},
	`charm_bio_synthetic`: {Name: `Charm`, Description: `To the target, your words are honey and your visage seems bathed in a dreamy haze. It must attempt a Will save, with a +4 circumstance bonus if you or your allies recently threatened it or used hostile actions against it.
You can Dismiss the spell. If you use hostile actions against the target, the spell ends. When the spell ends, the target doesn't necessarily realize it was charmed unless its friendship with you or the actions you convinced it to take clash with its expectations, meaning you could potentially convince the target to continue being your friend via mundane means.

Critical Success The target is unaffected and aware you tried to charm it.
Success The target is unaffected but thinks your spell was something harmless instead of charm, unless it identifies the spell.
Failure The target's attitude becomes @UUID[Compendium.pf2e.conditionitems.Item.Friendly] toward you. If it was Friendly, it becomes @UUID[Compendium.pf2e.conditionitems.Item.Helpful]. It can't use hostile actions against you.
Critical Failure The target's attitude becomes Helpful toward you, and it can't use hostile actions against you.

Heightened (4th) The duration lasts until the next time you make your daily preparations.
Heightened (8th) The duration lasts until the next time you make your daily preparations, and you can target up to 10 creatures.`},
	`charm_neural`: {Name: `Charm`, Description: `To the target, your words are honey and your visage seems bathed in a dreamy haze. It must attempt a Will save, with a +4 circumstance bonus if you or your allies recently threatened it or used hostile actions against it.
You can Dismiss the spell. If you use hostile actions against the target, the spell ends. When the spell ends, the target doesn't necessarily realize it was charmed unless its friendship with you or the actions you convinced it to take clash with its expectations, meaning you could potentially convince the target to continue being your friend via mundane means.

Critical Success The target is unaffected and aware you tried to charm it.
Success The target is unaffected but thinks your spell was something harmless instead of charm, unless it identifies the spell.
Failure The target's attitude becomes @UUID[Compendium.pf2e.conditionitems.Item.Friendly] toward you. If it was Friendly, it becomes @UUID[Compendium.pf2e.conditionitems.Item.Helpful]. It can't use hostile actions against you.
Critical Failure The target's attitude becomes Helpful toward you, and it can't use hostile actions against you.

Heightened (4th) The duration lasts until the next time you make your daily preparations.
Heightened (8th) The duration lasts until the next time you make your daily preparations, and you can target up to 10 creatures.`},
	`charm_technical`: {Name: `Social Override Protocol`, Description: `Broadcast a subvocal frequency pattern that biases target emotional responses toward compliance.`},
	`chilling_spray_bio_synthetic`: {Name: `Chilling Spray`, Description: `A cone of icy shards bursts from your spread hands and coats the targets in a layer of frost. You deal 2d4 cold damage to creatures in the area; they must each attempt a Reflex save.Critical Success The creature is unaffected.
Success The creature takes half damage.
Failure The creature takes full damage and takes a –5-foot status penalty to its Speeds for 2 rounds.
Critical Failure The creature takes double damage and takes a –10-foot status penalty to its Speeds for 2 rounds.Heightened (+1) The damage increases by 2d4.`},
	`chilling_spray_technical`: {Name: `Cryogenic Aerosol Burst`, Description: `Release a directed cryogenic mist from a pressurized canister, flash-freezing the target area.`},
	`chrome_reflex`: {Name: `Chrome Reflex`, Description: `A neural-augmented reflex burst that overrides the nervous system and forces a second attempt at a failed saving throw.`},
	`cleanse_cuisine_bio_synthetic`: {Name: `Cleanse Cuisine`, Description: `You transform all food and beverages in the area into delicious fare, changing water into wine or another fine beverage, or enhancing the food's taste and ingredients to make it a gourmet treat. You can also choose to remove all toxins and contaminations from the food. This spell doesn't prevent future contamination, natural decay, or spoilage, nor does it make the food any more nutritious.

Heightened (+2) Add another cubic foot to the area, which must be contiguous with the rest.`},
	`cleanse_cuisine_fanatic_doctrine`: {Name: `Cleanse Cuisine`, Description: `You transform all food and beverages in the area into delicious fare, changing water into wine or another fine beverage, or enhancing the food's taste and ingredients to make it a gourmet treat. You can also choose to remove all toxins and contaminations from the food. This spell doesn't prevent future contamination, natural decay, or spoilage, nor does it make the food any more nutritious.

Heightened (+2) Add another cubic foot to the area, which must be contiguous with the rest.`},
	`command_fanatic_doctrine`: {Name: `Command`, Description: `You shout a command that's hard to ignore. You can command the target to approach you, run away (as if it had the @UUID[Compendium.pf2e.conditionitems.Item.Fleeing] condition), release what it's holding, drop @UUID[Compendium.pf2e.conditionitems.Item.Prone], or stand in place. It can't @UUID[Compendium.pf2e.actionspf2e.Item.Delay] or take any reactions until it has obeyed your command. The effects depend on the target's Will save.

Success The creature is unaffected.
Failure For the first action on its next turn, the creature must use a single action to do as you command.
Critical Failure The target must use all its actions on its next turn to obey your command.

Heightened (5th) You can target up to 10 creatures.`},
	`command_neural`: {Name: `Command`, Description: `You shout a command that's hard to ignore. You can command the target to approach you, run away (as if it had the @UUID[Compendium.pf2e.conditionitems.Item.Fleeing] condition), release what it's holding, drop @UUID[Compendium.pf2e.conditionitems.Item.Prone], or stand in place. It can't @UUID[Compendium.pf2e.actionspf2e.Item.Delay] or take any reactions until it has obeyed your command. The effects depend on the target's Will save.

Success The creature is unaffected.
Failure For the first action on its next turn, the creature must use a single action to do as you command.
Critical Failure The target must use all its actions on its next turn to obey your command.

Heightened (5th) You can target up to 10 creatures.`},
	`command_technical`: {Name: `Authority Compliance Chip`, Description: `Trigger a pre-loaded compliance subroutine via radio signal, forcing a single commanded action.`},
	`concordant_choir_fanatic_doctrine`: {Name: `Concordant Choir`, Description: `You unleash a dangerous consonance of reverberating sound, focusing on a single target or spreading out to damage many foes. The number of actions you spend Casting this Spell determines its targets, range, area, and other parameters.1 The spell deals 1d4 sonic damage to a single enemy, with a basic Fortitude save.
2 (manipulate) The spell deals 2d4 sonic damage to all creatures in a @Template[burst|distance:10], with a basic Fortitude save.
3 (manipulate) The spell deals 2d4 sonic damage to all creatures in a @Template[emanation|distance:30], with a basic Fortitude save.Heightened (+1) The damage increases by 1d4 for the 1-action version, or 2d4 for the other versions.`},
	`concordant_choir_neural`: {Name: `Concordant Choir`, Description: `You unleash a dangerous consonance of reverberating sound, focusing on a single target or spreading out to damage many foes. The number of actions you spend Casting this Spell determines its targets, range, area, and other parameters.1 The spell deals 1d4 sonic damage to a single enemy, with a basic Fortitude save.
2 (manipulate) The spell deals 2d4 sonic damage to all creatures in a @Template[burst|distance:10], with a basic Fortitude save.
3 (manipulate) The spell deals 2d4 sonic damage to all creatures in a @Template[emanation|distance:30], with a basic Fortitude save.Heightened (+1) The damage increases by 1d4 for the 1-action version, or 2d4 for the other versions.`},
	`conductive_weapon_bio_synthetic`: {Name: `Conductive Weapon`, Description: `You channel powerful electric current through the metal of a weapon, zapping anyone the item hits. The target becomes a +1 shock weapon. If any target of an attack with the weapon is wearing metal armor or is primarily made of metal, the electricity damage die from the shock rune is 1d12.
@UUID[Compendium.pf2e.spell-effects.Item.Spell Effect: Conductive Weapon]`},
	`conductive_weapon_technical`: {Name: `Electroconductive Coating`, Description: `Apply a thin electroconductive layer to a weapon, causing it to discharge electricity on contact.`},
	`create_water_bio_synthetic`: {Name: `Create Water`, Description: `As you cup your hands, water begins to flow forth from them. You create 2 gallons of water. If no one drinks it, it evaporates after 1 day.`},
	`create_water_fanatic_doctrine`: {Name: `Create Water`, Description: `As you cup your hands, water begins to flow forth from them. You create 2 gallons of water. If no one drinks it, it evaporates after 1 day.`},
	`create_water_technical`: {Name: `Atmospheric Condenser`, Description: `Run a portable condenser unit to extract moisture from ambient air, producing clean water.`},
	`curse_of_recoil_fanatic_doctrine`: {Name: `Curse of Recoil`, Description: `Trigger An enemy you can see is about to make a ranged attack.You curse an enemy to suffer a kickback as they make a ranged attack, potentially causing them to miss. The triggering enemy attempts a Will save.Critical Success The target is unaffected.
Success The recoil from their ranged attack causes the target to be @UUID[Compendium.pf2e.conditionitems.Item.Off-Guard] until the beginning of their next turn.
Failure The recoil imposes a –1 status penalty to the ranged attack and renders the target off-guard until the beginning of their next turn.
@UUID[Compendium.pf2e.spell-effects.Item.Spell Effect: Curse of Recoil (Failure)]
Critical Failure The recoil imposes a –2 status penalty to the ranged attack and renders the target off-guard until the beginning of their next turn. Until the start of their next turn, any additional ranged attacks made with the same weapon, spell, or ability take the same penalty.
@UUID[Compendium.pf2e.spell-effects.Item.Spell Effect: Curse of Recoil (Critical Failure)]`},
	`curse_of_recoil_neural`: {Name: `Curse of Recoil`, Description: `Trigger An enemy you can see is about to make a ranged attack.You curse an enemy to suffer a kickback as they make a ranged attack, potentially causing them to miss. The triggering enemy attempts a Will save.Critical Success The target is unaffected.
Success The recoil from their ranged attack causes the target to be @UUID[Compendium.pf2e.conditionitems.Item.Off-Guard] until the beginning of their next turn.
Failure The recoil imposes a –1 status penalty to the ranged attack and renders the target off-guard until the beginning of their next turn.
@UUID[Compendium.pf2e.spell-effects.Item.Spell Effect: Curse of Recoil (Failure)]
Critical Failure The recoil imposes a –2 status penalty to the ranged attack and renders the target off-guard until the beginning of their next turn. Until the start of their next turn, any additional ranged attacks made with the same weapon, spell, or ability take the same penalty.
@UUID[Compendium.pf2e.spell-effects.Item.Spell Effect: Curse of Recoil (Critical Failure)]`},
	`cycle_of_retribution_fanatic_doctrine`: {Name: `Cycle of Retribution`, Description: `An understanding of how violence begets more violence fills your target, causing it mental anguish in the form of splitting headaches when it attempts to take violent actions. The target must attempt a Will saving throw, with the following results.Critical Success The target is unaffected.
Success The next time the target takes a hostile action, the target takes 1d4 mental damage; the spell's duration then ends.
Failure The first time in a round when the target takes a hostile action, the target takes 1d4 mental damage.
Critical Failure Each time the target takes a hostile action, the target takes 1d4 mental damage.Heightened (+1) The mental damage increases by 1d4.`},
	`cycle_of_retribution_neural`: {Name: `Cycle of Retribution`, Description: `An understanding of how violence begets more violence fills your target, causing it mental anguish in the form of splitting headaches when it attempts to take violent actions. The target must attempt a Will saving throw, with the following results.Critical Success The target is unaffected.
Success The next time the target takes a hostile action, the target takes 1d4 mental damage; the spell's duration then ends.
Failure The first time in a round when the target takes a hostile action, the target takes 1d4 mental damage.
Critical Failure Each time the target takes a hostile action, the target takes 1d4 mental damage.Heightened (+1) The mental damage increases by 1d4.`},
	`daze_fanatic_doctrine`: {Name: `Doctrine's Rebuke`, Description: `You deliver a crushing psychic rebuke in the Doctrine's name, dealing 1d6 mental damage. On a critical failure, the target is Stunned 1.`},
	`daze_neural`: {Name: `Cranial Shock`, Description: `You deliver a sharp mental jolt to the target, dealing 1d6 mental damage. On a critical failure, the target is also Stunned 1.`},
	`deep_breath_bio_synthetic`: {Name: `Extended Oxygen Reserve`, Description: `You activate a biological oxygen storage organ, allowing you to hold your breath without loss from physical exertion.`},
	`defended_by_spirits_fanatic_doctrine`: {Name: `Defended by Spirits`, Description: `You entreat a spirit or the spiritual energies in your location to surround and protect an ally from an certain foe. Each time the enemy attacks and damages the ally, the enemy takes 1d6 spirit damage. The enemy is aware of these spirits and has a general sense that attacking the ally will draw the spirits' ire.Heightened (+2) The damage increases by 1d6.`},
	`defended_by_spirits_neural`: {Name: `Defended by Spirits`, Description: `You entreat a spirit or the spiritual energies in your location to surround and protect an ally from an certain foe. Each time the enemy attacks and damages the ally, the enemy takes 1d6 spirit damage. The enemy is aware of these spirits and has a general sense that attacking the ally will draw the spirits' ire.Heightened (+2) The damage increases by 1d6.`},
	`dehydrate_bio_synthetic`: {Name: `Dehydrate`, Description: `You stir the inner fire of all things within the area, driving out moisture. All creatures in the area take 1d6 persistent fire damage with a basic Fortitude save; creatures with the water or plant traits get a result one degree of success worse than they rolled. The spell ends for a creature when its persistent damage ends.
A creature affected by dehydrate attempts an additional Fortitude save at the end of each of its turns, before rolling to recover from the persistent damage. It can forgo this additional save if it consumed water or a similar hydrating liquid within the last round (drinking typically requires a single action).

Success The creature takes no additional effect.
Failure The creature is Enfeebled 1 until the end of its next turn.
Critical Failure The creature is Enfeebled 2 until the end of its next turn.

Heightened (+2) The range increases by 10 feet, the burst increases by 5 feet, and the persistent fire damage increases by 3d6.`},
	`dehydrate_technical`: {Name: `Desiccant Pulse Emitter`, Description: `Emit a focused dehydration field that draws moisture from biological tissue.`},
	`detect_alignment_fanatic_doctrine`: {Name: `Detect Alignment`, Description: `Your eyes glow as you sense aligned auras. Choose chaotic, evil, good, or lawful. You detect auras of that alignment. You receive no information beyond presence or absence. You can choose not to detect creatures or effects you're aware have that alignment.
Only creatures of 6th level or higher-unless divine spellcasters, undead, or beings from the Outer Sphere-have alignment auras.

Heightened (2nd) You learn each aura's location and strength.`},
	`detect_alignment_neural`: {Name: `Detect Alignment`, Description: `Your eyes glow as you sense aligned auras. Choose chaotic, evil, good, or lawful. You detect auras of that alignment. You receive no information beyond presence or absence. You can choose not to detect creatures or effects you're aware have that alignment.
Only creatures of 6th level or higher-unless divine spellcasters, undead, or beings from the Outer Sphere-have alignment auras.

Heightened (2nd) You learn each aura's location and strength.`},
	`detect_magic_bio_synthetic`: {Name: `Bio-Energy Sensor Sweep`, Description: `You pulse a biochemical sensor sweep, detecting unusual energy signatures and active bio-augmentations nearby.`},
	`detect_magic_fanatic_doctrine`: {Name: `Forbidden Power Scan`, Description: `You sweep the area with doctrinal sensors, detecting the presence of unauthorized powers or forbidden technologies.`},
	`detect_magic_neural`: {Name: `EM Field Scan`, Description: `You pulse an electromagnetic sensor sweep through the area, detecting active energy signatures and powered systems.`},
	`detect_metal_bio_synthetic`: {Name: `Metallic Compound Sensor`, Description: `You activate a metallic compound detector, sensing the location and mass of metal objects within range.`},
	`detect_metal_fanatic_doctrine`: {Name: `Contraband Metal Detection`, Description: `You activate detection rites that reveal the presence and location of metallic objects the Doctrine deems contraband.`},
	`detect_metal_neural`: {Name: `Magnetic Resonance Scan`, Description: `You extend your sensory field to detect metallic objects and magnetic signatures in a cone ahead.`},
	`detect_poison_bio_synthetic`: {Name: `Detect Poison`, Description: `You detect whether a creature is venomous or poisonous, or if an object is poison or has been poisoned. You do not ascertain whether the target is poisonous in multiple ways, nor do you learn the type or types of poison. Certain substances, like lead and alcohol, are poisons and so mask other poisons.

Heightened (2nd) You learn the number and types of poison.`},
	`detect_poison_fanatic_doctrine`: {Name: `Detect Poison`, Description: `You detect whether a creature is venomous or poisonous, or if an object is poison or has been poisoned. You do not ascertain whether the target is poisonous in multiple ways, nor do you learn the type or types of poison. Certain substances, like lead and alcohol, are poisons and so mask other poisons.

Heightened (2nd) You learn the number and types of poison.`},
	`disguise_magic_neural`: {Name: `Disguise Magic`, Description: `You alter how an item's or spell's magical aura appears to effects like detect magic. You can hide the auras entirely, have an item register as a common item of lower level, or make a spell register as a common spell of the same or lower rank. You can Dismiss the spell. A caster using @UUID[Compendium.pf2e.spells-srd.Item.Detect Magic] or @UUID[Compendium.pf2e.spells-srd.Item.Read Aura] of a higher rank than disguise magic can attempt to disbelieve the illusion using the skill matching the tradition of the spell (Arcana for arcane, Religion for divine, Occultism for occult, or Nature for primal). Further attempts by the same caster get the same result as the initial check to disbelieve.

Heightened (2nd) You can Cast this Spell on a creature, disguising all items and spell effects on it.`},
	`disguise_magic_technical`: {Name: `Signal Masking Layer`, Description: `Apply an EM-dampening skin to hide technological emissions from scanner detection.`},
	`divine_lance`: {Name: `Doctrine's Judgment Bolt`, Description: `You call down a bolt of the Doctrine's pure ideological energy, dealing damage aligned to your Doctrine's chosen aspect.`},
	`dizzying_colors_neural`: {Name: `Dizzying Colors`, Description: `You unleash a swirling multitude of colors that overwhelms creatures based on their Will saves.

Critical Success The creature is unaffected.
Success The creature is @UUID[Compendium.pf2e.conditionitems.Item.Dazzled] for 1 round.
Failure The creature is Stunned 1, @UUID[Compendium.pf2e.conditionitems.Item.Blinded] for 1 round, and dazzled for 1 minute.
Critical Failure The creature is stunned for 1 round and blinded for 1 minute.`},
	`dizzying_colors_technical`: {Name: `Strobing Optical Disruptor`, Description: `Flash a rapid-cycle light array at target frequencies that trigger disorientation and nausea.`},
	`dj_vu_neural`: {Name: `Déjà Vu`, Description: `You loop a thought process in the target's mind, forcing it to repeat a moment's worth of actions. The target must attempt a Will save. If the target fails, whatever actions the target uses on its next turn, it must repeat on its following turn. The actions must be repeated in the same order and as close to the same specifics as possible. For example, if the target makes an attack, it must repeat the attack against the same creature, if possible, and if the target moves, it must move the same distance and direction, if possible, on its next turn.
If the target can't repeat an action, such as Casting a Spell that has been exhausted or needing to target a creature that has died, it can act as it chooses for that action but becomes Stupefied 1 until the end of its turn.`},
	`dj_vu_technical`: {Name: `Memory Loop Injection`, Description: `Feed a repeating sensory loop into target neural interfaces, creating disorienting temporal confusion.`},
	`draw_ire_neural`: {Name: `Draw Ire`, Description: `You cause mental distress to a creature, goading it to strike back at you. You deal 1d10 mental damage to the creature and cause it to take a -1 status penalty to attack rolls against creatures other than you. The creature must attempt a Will saving throw.Critical Success The target is unaffected.
Success The target takes half damage and the penalty. The spell ends at the end of the target's next turn. @UUID[Compendium.pf2e.spell-effects.Item.Spell Effect: Draw Ire (Success)]
Failure The target takes full damage and the penalty.
Critical Failure The target takes double damage, and the status penalty is -2.
@UUID[Compendium.pf2e.spell-effects.Item.Spell Effect: Draw Ire]Heightened (+1) The damage increases by 1d10.`},
	`draw_ire_technical`: {Name: `Threat Beacon Broadcast`, Description: `Emit an aggressive targeting signature that draws hostile attention toward the user.`},
	`draw_moisture_bio_synthetic`: {Name: `Moisture Siphon`, Description: `You extract moisture from an object using bio-organic osmosis, completely drying it as a byproduct.`},
	`draw_moisture_fanatic_doctrine`: {Name: `Doctrine's Desiccation`, Description: `You invoke the Doctrine's purifying will to drive moisture from an object, completely drying it.`},
	`eat_fire_bio_synthetic`: {Name: `Thermal Conversion Organ`, Description: `Reaction — triggered when you would take fire damage. A thermal conversion organ absorbs and metabolizes the heat energy.`},
	`eat_fire_neural`: {Name: `Thermal Absorption`, Description: `Reaction — triggered when you would take fire damage. Your neural augments reroute the thermal energy, absorbing the damage.`},
	`echoing_weapon_fanatic_doctrine`: {Name: `Echoing Weapon`, Description: `You channel magical energy into the target weapon, and the air around it faintly hums each time you strike a blow, as the impact is absorbed into the weapon. If a creature is wielding the weapon at the end of its turn, the weapon discharges a burst of sound targeting one creature adjacent to the wielder (if any). The sonic damage this deals is equal to the number of successful Strikes with the target weapon that the wielder made that turn (to a maximum of 4 sonic damage if the wielder hits with four Strikes).

Heightened (+2) The sonic damage increases by 1 per Strike (and the maximum damage increases by 4).`},
	`echoing_weapon_neural`: {Name: `Echoing Weapon`, Description: `You channel magical energy into the target weapon, and the air around it faintly hums each time you strike a blow, as the impact is absorbed into the weapon. If a creature is wielding the weapon at the end of its turn, the weapon discharges a burst of sound targeting one creature adjacent to the wielder (if any). The sonic damage this deals is equal to the number of successful Strikes with the target weapon that the wielder made that turn (to a maximum of 4 sonic damage if the wielder hits with four Strikes).

Heightened (+2) The sonic damage increases by 1 per Strike (and the maximum damage increases by 4).`},
	`echoing_weapon_technical`: {Name: `Kinetic Echo Amplifier`, Description: `Attach a resonance module to a weapon that creates a trailing kinetic duplicate of each strike.`},
	`electric_arc_bio_synthetic`: {Name: `Bio-Electric Discharge`, Description: `You discharge stored bio-electric energy in an arc that leaps between two nearby targets, dealing lightning damage to both.`},
	`elemental_counter_bio_synthetic`: {Name: `Elemental Resistance Compound`, Description: `Reaction — triggered when a creature takes elemental damage. You administer a fast-acting resistance compound, granting temporary elemental resistance.`},
	`elysian_whimsy_bio_synthetic`: {Name: `Elysian Whimsy`, Description: `You overwhelm the target with an unexpected and unpredictable desire if it fails a Will save. Roll [[/r 1d4]] to determine the spell's effect.



1d4
Effect




1
The target feels a powerful urge to dance. For 1 round, it takes a –5-foot status penalty to its Speeds (-10-foot status penalty on a critical failure), capering and prancing as it moves.


2
The target is compelled to loudly sing a song. Its first action on its next turn must be to @UUID[Compendium.pf2e.actionspf2e.Item.Perform] a song it knows, or to babble pleasingly if it knows no songs. On a critical failure, the target must use all its actions on its next turn to Perform a song.


3
The target is filled with an irresistible urge to support a nearby creature's entertainment career. Its first action on its next turn must be to prepare to Aid a Perform check for the nearest creature it can see and the target can use the next reaction it gains only to Aid the creature it helped. On a critical failure, it must spend all its actions on its next turn preparing to Aid a Perform check.


4
The target is overcome with a desire to give away its wealth. Its first action on its next turn must be to Interact to pull out a non-magical item of value it is carrying (such as a coin, piece of jewelry, or an item made of precious metal), if it doesn't already have one in hand. It then Releases the valuable item. If the target neither holds nor carries an appropriate item, it instead spends its first action loudly apologizing for having nothing to give. On a critical failure, the target must spend any actions remaining on its turn apologizing for not giving more.`},
	`elysian_whimsy_fanatic_doctrine`: {Name: `Elysian Whimsy`, Description: `You overwhelm the target with an unexpected and unpredictable desire if it fails a Will save. Roll [[/r 1d4]] to determine the spell's effect.



1d4
Effect




1
The target feels a powerful urge to dance. For 1 round, it takes a –5-foot status penalty to its Speeds (-10-foot status penalty on a critical failure), capering and prancing as it moves.


2
The target is compelled to loudly sing a song. Its first action on its next turn must be to @UUID[Compendium.pf2e.actionspf2e.Item.Perform] a song it knows, or to babble pleasingly if it knows no songs. On a critical failure, the target must use all its actions on its next turn to Perform a song.


3
The target is filled with an irresistible urge to support a nearby creature's entertainment career. Its first action on its next turn must be to prepare to Aid a Perform check for the nearest creature it can see and the target can use the next reaction it gains only to Aid the creature it helped. On a critical failure, it must spend all its actions on its next turn preparing to Aid a Perform check.


4
The target is overcome with a desire to give away its wealth. Its first action on its next turn must be to Interact to pull out a non-magical item of value it is carrying (such as a coin, piece of jewelry, or an item made of precious metal), if it doesn't already have one in hand. It then Releases the valuable item. If the target neither holds nor carries an appropriate item, it instead spends its first action loudly apologizing for having nothing to give. On a critical failure, the target must spend any actions remaining on its turn apologizing for not giving more.`},
	`elysian_whimsy_technical`: {Name: `Behavioral Randomizer`, Description: `Inject a chaotic signal into target neural systems, causing unpredictable and erratic behavior.`},
	`endure_neural`: {Name: `Endure`, Description: `You invigorate the touched creature's mind and urge it to press on. You grant the touched creature 5 temporary Hit Points that last for 1 minute.Heightened (+1) The temporary Hit Points increase by 5.
@UUID[Compendium.pf2e.spell-effects.Item.Spell Effect: Endure]`},
	`endure_technical`: {Name: `Metabolic Stabilizer Injection`, Description: `Administer a rapid-acting compound that stabilizes vital signs and prevents debilitating conditions.`},
	`enfeeble_fanatic_doctrine`: {Name: `Enfeeble`, Description: `You sap the target's strength, depending on its Fortitude save.

Critical Success The target is unaffected.
Success The target is Enfeebled 1 until the start of your next turn.
Failure The target is Enfeebled 2 for 1 minute.
Critical Failure The target is Enfeebled 3 for 1 minute.`},
	`enfeeble_neural`: {Name: `Enfeeble`, Description: `You sap the target's strength, depending on its Fortitude save.

Critical Success The target is unaffected.
Success The target is Enfeebled 1 until the start of your next turn.
Failure The target is Enfeebled 2 for 1 minute.
Critical Failure The target is Enfeebled 3 for 1 minute.`},
	`enfeeble_technical`: {Name: `Muscle Inhibitor Field`, Description: `Generate a localized field that disrupts motor control signals, reducing target physical output.`},
	`equal_footing_bio_synthetic`: {Name: `Equal Footing`, Description: `You level the field between yourself and another creature, hampering its movements if it's quicker than you. The target attempts a Will save.Critical Success The target is unaffected.
Success The target is Clumsy 1 and takes a –10-foot status penalty to all its Speeds until the end of your next turn.
@UUID[Compendium.pf2e.spell-effects.Item.Spell Effect: Equal Footing (Success)]
Failure The target is clumsy 1 and takes a –15-foot status penalty to all its Speeds for 1 minute. During this time, it can't benefit from bonuses to its Speeds or take other penalties to its Speeds.
Critical Failure The target is Clumsy 2 and takes a –15-foot status penalty to all its Speeds for 1 minute. During this time, it can't benefit from bonuses to its Speeds or take other penalties to its Speeds.
@UUID[Compendium.pf2e.spell-effects.Item.Spell Effect: Equal Footing (Failure or Critical Failure)]`},
	`equal_footing_neural`: {Name: `Equal Footing`, Description: `You level the field between yourself and another creature, hampering its movements if it's quicker than you. The target attempts a Will save.Critical Success The target is unaffected.
Success The target is Clumsy 1 and takes a –10-foot status penalty to all its Speeds until the end of your next turn.
@UUID[Compendium.pf2e.spell-effects.Item.Spell Effect: Equal Footing (Success)]
Failure The target is clumsy 1 and takes a –15-foot status penalty to all its Speeds for 1 minute. During this time, it can't benefit from bonuses to its Speeds or take other penalties to its Speeds.
Critical Failure The target is Clumsy 2 and takes a –15-foot status penalty to all its Speeds for 1 minute. During this time, it can't benefit from bonuses to its Speeds or take other penalties to its Speeds.
@UUID[Compendium.pf2e.spell-effects.Item.Spell Effect: Equal Footing (Failure or Critical Failure)]`},
	`equal_footing_technical`: {Name: `Size Normalization Harness`, Description: `Deploy a gravity-adjustment harness that equalizes physical reach and momentum regardless of size.`},
	`exchange_image_neural`: {Name: `Exchange Image`, Description: `To mislead pursuers, the Lacunafex developed the means to swap visages. You trade appearances with the target, with the effects depending on the result of the target's Will saving throw. Willing and @UUID[Compendium.pf2e.conditionitems.Item.Unconscious] targets automatically critically fail this saving throw.

Critical Success No effect.
Success You take on the target's appearance, and they take yours. This has the same effects as a 1st-rank @UUID[Compendium.pf2e.spells-srd.Item.Illusory Disguise] spell, except that the target can't Dismiss the disguise. The duration is 1 minute or until Dismissed.
Failure As success, but the duration is 1 hour or until Dismissed.
Critical Failure As success, but the duration is 24 hours.`},
	`exchange_image_technical`: {Name: `Holographic Identity Swap`, Description: `Project paired holographic overlays that swap the apparent identities of two targets.`},
	`fashionista_neural`: {Name: `Fashionista`, Description: `The target's clothes are transformed into ostentatious attire that epitomizes high-end local fashion. No details of the target's appearance transform other than their clothes, so their weapons or armor remain unchanged in appearance. The target gains a +1 status bonus on Deception checks to @UUID[Compendium.pf2e.actionspf2e.Item.Create a Diversion]. You can Dismiss this spell. At the end of the spell's duration, the target's clothes revert to their original appearance.

Heightened (+2) The status bonus increases by 1, to a maximum of +4 at 7th rank.`},
	`fashionista_technical`: {Name: `Smart Wardrobe Interface`, Description: `Access a wireless clothing control system that adjusts outfit appearance on demand.`},
	`fated_healing_fanatic_doctrine`: {Name: `Fated Healing`, Description: `You speak about the consequences of actions people take against each other and how it's possible to break cycles of violence simply by making a different choice. The targets regain 1d4 Hit Points at the end of each of their own turns while the spell is in effect. If a target uses a hostile action against the other target, the spell ends for the target that used the hostile action.Heightened (+1) The targets regain an additional 1d4 Hit Points at the end of their own turns.`},
	`fated_healing_neural`: {Name: `Fated Healing`, Description: `You speak about the consequences of actions people take against each other and how it's possible to break cycles of violence simply by making a different choice. The targets regain 1d4 Hit Points at the end of each of their own turns while the spell is in effect. If a target uses a hostile action against the other target, the spell ends for the target that used the hostile action.Heightened (+1) The targets regain an additional 1d4 Hit Points at the end of their own turns.`},
	`fear_bio_synthetic`: {Name: `Fear`, Description: `You plant fear in the target; it must attempt a Will save.

Critical Success The target is unaffected.
Success The target is Frightened 1.
Failure The target is Frightened 2.
Critical Failure The target is Frightened 3 and @UUID[Compendium.pf2e.conditionitems.Item.Fleeing] for 1 round.

Heightened (3rd) You can target up to five creatures.`},
	`fear_fanatic_doctrine`: {Name: `Fear`, Description: `You plant fear in the target; it must attempt a Will save.

Critical Success The target is unaffected.
Success The target is Frightened 1.
Failure The target is Frightened 2.
Critical Failure The target is Frightened 3 and @UUID[Compendium.pf2e.conditionitems.Item.Fleeing] for 1 round.

Heightened (3rd) You can target up to five creatures.`},
	`fear_neural`: {Name: `Fear`, Description: `You plant fear in the target; it must attempt a Will save.

Critical Success The target is unaffected.
Success The target is Frightened 1.
Failure The target is Frightened 2.
Critical Failure The target is Frightened 3 and @UUID[Compendium.pf2e.conditionitems.Item.Fleeing] for 1 round.

Heightened (3rd) You can target up to five creatures.`},
	`fear_technical`: {Name: `Threat Assessment Override`, Description: `Broadcast a danger signature directly to threat-detection augmentations, triggering fight-or-flight.`},
	`figment_neural`: {Name: `Sensory Projection`, Description: `You project a simple audio or visual illusion — a sound, a shape, a brief image — with no physical substance.`},
	`flashy_disappearance_neural`: {Name: `Flashy Disappearance`, Description: `You create a puff of colorful smoke that quickly disperses while you become temporarily @UUID[Compendium.pf2e.conditionitems.Item.Invisible]. You become @UUID[Compendium.pf2e.conditionitems.Item.Undetected] to all creatures unless they can see invisible creatures. You Stride. At the end of your movement, if you have cover, greater cover, or concealment, attempt a @Check[stealth|traits:action:hide] check to Hide. You gain a +2 status bonus to this Stealth check. The invisibility then ends, and you either become @UUID[Compendium.pf2e.conditionitems.Item.Observed] or @UUID[Compendium.pf2e.conditionitems.Item.Hidden] to creatures as determined by your check to Hide, if you made one.
@UUID[Compendium.pf2e.spell-effects.Item.Spell Effect: Flashy Disappearance]`},
	`flashy_disappearance_technical`: {Name: `Smoke-Bang Exit Package`, Description: `Deploy a combined smoke and flash package providing immediate concealment and distraction.`},
	`fleet_step_bio_synthetic`: {Name: `Fleet Step`, Description: `You gain a +30-foot status bonus to your Speed.
@UUID[Compendium.pf2e.spell-effects.Item.Spell Effect: Fleet Step]`},
	`fleet_step_technical`: {Name: `Mobility Boost Actuator`, Description: `Trigger leg actuator overcharge for a short burst of enhanced movement speed.`},
	`flense_fanatic_doctrine`: {Name: `Flense`, Description: `With a touch, you strip off the flesh, muscle, and internal organs off your target, leaving only bare bones. The effect depends on whether the target is a living creature, undead creature, or inanimate corpse. A creature or corpse that lacks flesh, muscle, and internal organs is immune to this spell.
Inanimate Corpse The flesh, muscle, viscera, and organs are stripped from the corpse and vanish, leaving only bare bones behind.
Living Creature Make a spell attack roll. On a hit, the target takes 2d6 slashing damage. On a critical hit, double the damage, and the target also takes 1d4 persistent bleed damage. If this spell's damage kills the target, the corpse is only bones.
Undead Creature Make a spell attack roll. On a hit, the target takes 2d6 slashing damage. On a critical hit, double the damage, and the target also becomes Enfeebled 1 for 1 minute. If this spell's damage destroys the target, only its bare bones remain behind.

Heightened (+1) The slashing damage to living and undead creatures increases by 2d6, and the persistent bleed damage to living creatures increases by 1d4.`},
	`flense_technical`: {Name: `Ablative Strip Charge`, Description: `Fire a shaped charge designed to ablate armor layers, exposing underlying flesh.`},
	`flourishing_flora_bio_synthetic`: {Name: `Flourishing Flora`, Description: `Plants rapidly grow up from the ground. All creatures in the target area take 2d4 damage. The type of damage depends on the type of plant you choose to grow. On a critical failure, targets experience additional effects, also depending on what you choose to grow. The type of plant and its effects are chosen when you Cast the Spell.

Cacti Piercing damage, and @Damage[(@item.level)[bleed]] damage on a critical failure.
Flowers Poison damage, and @UUID[Compendium.pf2e.conditionitems.Item.Dazzled] for 2 rounds on a critical failure.
Fruits Bludgeoning damage, and Clumsy 1 for 2 rounds on a critical failure.
Roots Bludgeoning damage, and the affected creatures fall @UUID[Compendium.pf2e.conditionitems.Item.Prone] on a critical failure.


Heightened (+1) The damage increases by 1d4, and the persistent bleed damage from cacti increases by 1.`},
	`flourishing_flora_technical`: {Name: `Rapid Growth Accelerant`, Description: `Spray a bio-accelerant compound that causes local vegetation to grow explosively fast.`},
	`fold_metal`: {Name: `Precision Metal Former`, Description: `Use a focused electromagnetic former to bend and shape metal components with machine precision.`},
	`foraging_friends`: {Name: `Foraging Friends`, Description: `Giving a cheerful whistle, you call forth a handful of small animals, such as birds or mice, to collect food for you and your allies. The animals return 1 hour later with enough foraged goods to feed four Medium creatures for 1 day, then return to their normal behavior. If you're in a particularly strange environment, as determined by your GM, you might need a minimum proficiency with primal spell DCs, equivalent to the minimum proficiency required to Subsist in strange environments.

Heightened (3rd) The animals bring back enough food for eight Medium creatures for 1 day.
Heightened (5th) The animals bring back enough food for 30 Medium creatures for 1 day.`},
	`forbidding_ward_fanatic_doctrine`: {Name: `Sanctuary Barrier`, Description: `You surround an ally with a doctrinal protective field, making it harder for enemies to strike them.`},
	`forbidding_ward_neural`: {Name: `Threat Interposition Field`, Description: `You broadcast a psionic barrier around an ally, making hostile actions against them more difficult.`},
	`force_barrage_neural`: {Name: `Force Barrage`, Description: `You fire a shard of solidified magic toward a creature that you can see. It automatically hits and deals 1d4+1 force damage. For each additional action you use when Casting the Spell, increase the number of shards you shoot by one, to a maximum of three shards for 3 actions. You choose the target for each shard individually. If you shoot more than one shard at the same target, combine the damage before applying bonuses or penalties to damage, resistances, weaknesses, and so forth.

Heightened (+2) You fire one additional shard with each action you spend.`},
	`force_barrage_technical`: {Name: `Multi-Round Kinetic Volley`, Description: `Fire a rapid burst of kinetic impactors from a multi-barrel launcher.`},
	`forced_mercy_fanatic_doctrine`: {Name: `Forced Mercy`, Description: `You soften the target's blows, ensuring they avoid vital areas and cause no lasting harm. All physical damage dealt by the target to living creatures becomes nonlethal and all persistent bleed damage dealt by the target is reduced to 0. This effect doesn't incur the typical –2 circumstance penalty for nonlethal attacks with a lethal weapon or attack. An unwilling target must attempt a Will save. A willing target can choose to critically fail their saving throw.Critical Success The creature is unaffected.
Success The creature is affected for 1 round.
Failure The creature is affected for [[/gmr 1d4 #rounds]]{1d4 rounds}.
Critical Failure The creature is affected for 1 minute.Heightened (4th) The range increases to 100 feet, and you can target up to 8 creatures.`},
	`forced_mercy_neural`: {Name: `Forced Mercy`, Description: `You soften the target's blows, ensuring they avoid vital areas and cause no lasting harm. All physical damage dealt by the target to living creatures becomes nonlethal and all persistent bleed damage dealt by the target is reduced to 0. This effect doesn't incur the typical –2 circumstance penalty for nonlethal attacks with a lethal weapon or attack. An unwilling target must attempt a Will save. A willing target can choose to critically fail their saving throw.Critical Success The creature is unaffected.
Success The creature is affected for 1 round.
Failure The creature is affected for [[/gmr 1d4 #rounds]]{1d4 rounds}.
Critical Failure The creature is affected for 1 minute.Heightened (4th) The range increases to 100 feet, and you can target up to 8 creatures.`},
	`forge_bio_synthetic`: {Name: `Forge`, Description: `Developed before the introduction of the Iron Lagoon, this cantrip for superheating metal has also found valuable combat use. You superheat the target, dealing 3d6 fire damage. If the target is a metal object, reduce its Hardness by an amount equal to the damage dealt until the end of your next turn.

Critical Success The target is unaffected.
Success The target takes half damage.
Failure The target takes full damage.
Critical Failure The target takes double damage, and if it's a metal creature, it gains weakness 2 to physical damage until the end of your next turn.

Heightened (+1) The damage increases by 2d6, and the weakness on a critical failure increases by 2.
@UUID[Compendium.pf2e.spell-effects.Item.Spell Effect: Forge (Critical Failure)]`},
	`forge_technical`: {Name: `Rapid Fabrication Unit`, Description: `Deploy a compact forge unit capable of working metal into basic shapes under field conditions.`},
	`friendfetch_neural`: {Name: `Friendfetch`, Description: `You shoot out ephemeral, telekinetic strands that drag each target directly toward you, stopping in the closest unoccupied space to you in this path. This is forced movement.`},
	`friendfetch_technical`: {Name: `Magnetic Tether Reel`, Description: `Fire a magnetic grapple that latches onto a target and reels them toward you.`},
	`frostbite_bio_synthetic`: {Name: `Cryogenic Encasement Spray`, Description: `You spray the target with rapid-expanding cryogenic fluid, encasing a limb and dealing cold damage.`},
	`funeral_flames_bio_synthetic`: {Name: `Incendiary Weapon Coat`, Description: `You coat a weapon in a bio-synthesized incendiary compound, causing it to ignite and deal fire damage on strikes.`},
	`funeral_flames_fanatic_doctrine`: {Name: `Martyr's Torch`, Description: `You sanctify a weapon with the Doctrine's funeral flame, causing it to burn with ideological fire on each strike.`},
	`gale_blast_bio_synthetic`: {Name: `Compressed Air Exhale`, Description: `You exhale a focused blast of compressed air from modified respiratory systems, creating a wind shear that buffets targets.`},
	`gentle_landing_bio_synthetic`: {Name: `Gentle Landing`, Description: `Trigger a creature within range is falling

You raise a magical updraft to arrest a fall. The target's fall slows to 60 feet per round, and the portion of the fall during the spell's duration doesn't count when calculating falling damage. If the target reaches the ground while the spell is in effect, it takes no damage from the fall. The spell ends as soon as the target lands.`},
	`gentle_landing_technical`: {Name: `Impact Absorption Foam`, Description: `Deploy quick-hardening impact foam beneath a falling target to cushion the landing.`},
	`ghost_sound_neural`: {Name: `Audio Ghost`, Description: `You generate a convincing phantom sound at a location of your choice — voices, machinery, footsteps, whatever serves your purpose.`},
	`glamorize_bio_synthetic`: {Name: `Cosmetic Compound Application`, Description: `You apply a fast-acting cosmetic compound to alter a minor physical detail of the target's appearance.`},
	`glamorize_fanatic_doctrine`: {Name: `Doctrine's Cosmetic Blessing`, Description: `You invoke a minor doctrinal glamour to alter a small aspect of the target's appearance.`},
	`glamorize_neural`: {Name: `Minor Cosmetic Overlay`, Description: `You project a minor holographic overlay to alter a small aspect of a target's appearance.`},
	`glass_shield_bio_synthetic`: {Name: `Transparent Polymer Barrier`, Description: `You extrude a pane of hardened transparent polymer as a physical shield that absorbs one hit before shattering.`},
	`glowing_trail_bio_synthetic`: {Name: `Bioluminescent Track`, Description: `Your movement leaves a trail of bioluminescent compound behind, slowly fading after you pass.`},
	`glowing_trail_fanatic_doctrine`: {Name: `Doctrine's Luminous Path`, Description: `Your footsteps leave a glowing trail of doctrinal light, visible as a guide to others who follow the same path.`},
	`glowing_trail_neural`: {Name: `Bioluminescent Trace`, Description: `Your movements leave a faint glowing trail behind you, like residual psionic heat that slowly fades.`},
	`goblin_pox_bio_synthetic`: {Name: `Goblin Pox`, Description: `Your touch afflicts the target with goblin pox, an irritating allergenic rash. The target must attempt a Fortitude save.

Goblin Pox (disease) Level 1; Creatures that have the goblin trait and goblin dogs are immune
Stage 1 Sickened 1 (1 round)
Stage 2 Sickened 1 and Slowed 1 (1 round)
Stage 3 Sickened 1 and the creature can't reduce its Sickened value below 1 (1 day)

Critical Success The target is unaffected.
Success The target is Sickened 1.
Failure The target is afflicted with goblin pox at stage 1.
Critical Failure The target is afflicted with goblin pox at stage 2.`},
	`goblin_pox_technical`: {Name: `Synthetic Pathogen Dispersal`, Description: `Release a fast-spreading synthetic pathogen that causes debilitating sores and illness.`},
	`gouging_claw_bio_synthetic`: {Name: `Extrude Blade Claw`, Description: `You trigger a rapid bio-synthetic claw extrusion from a limb, making a slashing melee attack.`},
	`gravitational_pull_neural`: {Name: `Gravitational Pull`, Description: `By suddenly altering gravity, you pull the target toward you. The target is pulled 10 feet closer to you unless it succeeds at a Fortitude save. On a critical failure, it's also knocked @UUID[Compendium.pf2e.conditionitems.Item.Prone]. The effects of this spell change depending on the number of actions you spend when you Cast this Spell.
1 (somatic) The spell targets one creature.
2 (somatic, verbal) The spell targets one creature and pulls the target 20 feet instead of 10.
3 (material, somatic, verbal) The spell targets up to 5 creatures.`},
	`gravitational_pull_technical`: {Name: `Grav-Spike Projector`, Description: `Fire a focused gravitational anomaly that pulls targets toward a fixed point.`},
	`grease_bio_synthetic`: {Name: `Grease`, Description: `You conjure grease, choosing an area or target.

Area [4 contiguous 5-foot squares] All solid ground in the area is covered with grease. Each creature standing on the greasy surface must succeed at a Reflex save or an Acrobatics check against your spell DC or fall @UUID[Compendium.pf2e.conditionitems.Item.Prone]. Creatures using an action to move onto the greasy surface during the spell's duration must attempt either a Reflex save or an Acrobatics check to @UUID[Compendium.pf2e.actionspf2e.Item.Balance]. A creature that Steps or Crawls doesn't have to attempt a check or save.
Target [1 object of Bulk 1 or less] If you Cast the Spell on an unattended object, anyone trying to pick up the object must succeed at an Acrobatics check or Reflex save against your spell DC to do so. If you target an attended object, the creature that has the object must attempt an Acrobatics check or Reflex save. On a failure, the holder or wielder takes a –2 circumstance penalty to all checks that involve using the object; on a critical failure, the holder or wielder releases the item. The object lands in an adjacent square of the GM's choice. If you Cast this Spell on a worn object, the wearer gains a +2 circumstance bonus to Fortitude saves against attempts to grapple them.`},
	`grease_technical`: {Name: `Lubricant Spray System`, Description: `Coat a surface or target in industrial lubricant, causing loss of traction and grip.`},
	`grim_tendrils_neural`: {Name: `Grim Tendrils`, Description: `Tendrils of darkness curl out from your fingertips and race through the air. You deal 2d4 void damage and @Damage[(@item.level)[bleed]] damage to living creatures in the line. Each living creature in the line must attempt a Fortitude save.

Critical Success The creature is unaffected.
Success The creature takes half the void damage and no persistent bleed damage.
Failure The creature takes full damage.
Critical Failure The creature takes double void damage and double persistent bleed damage.

Heightened (+1) The void damage increases by 2d4, and the persistent bleed damage increases by 1.`},
	`grim_tendrils_technical`: {Name: `Void Tendril Projector`, Description: `Project writhing energy filaments that drain vitality from anything they touch.`},
	`gritty_wheeze_bio_synthetic`: {Name: `Gritty Wheeze`, Description: `You exhale desiccating grit and sand in a small cloud. Creatures in the area take 2d4 bludgeoning damage and must attempt a Fortitude save.
Water creatures and plant creatures use the outcome one degree of success worse than the result of their saving throw.

Critical Success The creature takes no damage.
Success The creature takes half damage.
Failure The creature takes full damage and is @UUID[Compendium.pf2e.conditionitems.Item.Dazzled] for 1 round.
Critical Failure The creature takes double damage and is dazzled for 1 minute.

Heightened (+1) The damage increases by 2d4.`},
	`gritty_wheeze_technical`: {Name: `Particle Spray Grenade`, Description: `Detonate an abrasive particle grenade that coats the area in choking, blinding debris.`},
	`guidance_bio_synthetic`: {Name: `Bioscan Assist Signal`, Description: `You run a rapid bio-scan of the situation and feed tactical data to a target, granting +1 to their next skill check.`},
	`guidance_fanatic_doctrine`: {Name: `Doctrine's Guidance`, Description: `You invoke the Doctrine's wisdom, granting the target +1 to their next skill check as the Doctrine's insight flows through them.`},
	`guidance_neural`: {Name: `Tactical Insight`, Description: `You broadcast a targeted advice signal, granting a creature a +1 bonus to their next skill check.`},
	`gust_of_wind_bio_synthetic`: {Name: `Gust of Wind`, Description: `A violent wind issues forth from your palm, blowing from the point where you are when you Cast the Spell to the line's opposite end. The wind extinguishes small non-magical fires, disperses fog and mist, blows objects of light Bulk or less around, and pushes larger objects. Large or smaller creatures in the area must attempt a Fortitude save. Large or smaller creatures that later move into the gust must attempt the save on entering.

Critical Success The creature is unaffected.
Success The creature can't move against the wind.
Failure The creature is knocked @UUID[Compendium.pf2e.conditionitems.Item.Prone]. If it was flying, it takes the effects of critical failure instead.
Critical Failure The creature is pushed 30 feet in the wind's direction, knocked prone, and takes 2d6 bludgeoning damage.`},
	`gust_of_wind_technical`: {Name: `High-Pressure Air Cannon`, Description: `Fire a concentrated burst of compressed air capable of moving objects and disrupting attacks.`},
	`harm`: {Name: `Harm`, Description: `You channel void energy to harm the living or heal the undead. If the target is a living creature, you deal 1d8 void damage to it, and it gets a basic Fortitude save. If the target is a willing undead creature, you restore that amount of Hit Points. The number of actions you spend when Casting this Spell determines its targets, range, area, and other parameters.
1 The spell has a range of touch.
2 (concentrate) The spell has a range of 30 feet. If you're healing an undead creature, increase the Hit Points restored by 8.
3 (concentrate) You disperse void energy in a @Template[emanation|distance:30]. This targets all living and undead creatures in the area.

Heightened (+1) The amount of healing or damage increases by 1d8, and the extra healing for the 2-action version increases by 8.`},
	`haunting_hymn_fanatic_doctrine`: {Name: `Zealous War Hymn`, Description: `You broadcast a hymn of zealous conviction that only enemies can truly hear — it deals sonic damage and disrupts their focus.`},
	`haunting_hymn_neural`: {Name: `Subliminal Screech`, Description: `You broadcast a subsonic audio spike that only nearby targets can perceive — it deals sonic damage and scrambles concentration.`},
	`heal_bio_synthetic`: {Name: `Heal`, Description: `You channel vital energy to heal the living or damage the undead. If the target is a willing living creature, you restore 1d8 Hit Points. If the target is undead, you deal that amount of vitality damage to it, and it gets a basic Fortitude save. The number of actions you spend when Casting this Spell determines its targets, range, area, and other parameters.
1 The spell has a range of touch.
2 (concentrate) The spell has a range of 30 feet. If you're healing a living creature, increase the Hit Points restored by 8.
3 (concentrate) You disperse vital energy in a @Template[emanation|distance:30]. This targets all living and undead creatures in the burst.

Heightened (+1) The amount of healing or damage increases by 1d8, and the extra healing for the 2-action version increases by 8.`},
	`heal_fanatic_doctrine`: {Name: `Heal`, Description: `You channel vital energy to heal the living or damage the undead. If the target is a willing living creature, you restore 1d8 Hit Points. If the target is undead, you deal that amount of vitality damage to it, and it gets a basic Fortitude save. The number of actions you spend when Casting this Spell determines its targets, range, area, and other parameters.
1 The spell has a range of touch.
2 (concentrate) The spell has a range of 30 feet. If you're healing a living creature, increase the Hit Points restored by 8.
3 (concentrate) You disperse vital energy in a @Template[emanation|distance:30]. This targets all living and undead creatures in the burst.

Heightened (+1) The amount of healing or damage increases by 1d8, and the extra healing for the 2-action version increases by 8.`},
	`healing_plaster`: {Name: `Bio-Patch Compound`, Description: `You synthesize a healing plaster from available organic material, applying it to accelerate wound closure and restore Hit Points over time.`},
	`helpful_steps_bio_synthetic`: {Name: `Helpful Steps`, Description: `You call forth a ladder or staircase to help you reach greater heights. The ladder or staircase appears in a space you designate and either stands freely or connects to a nearby wall if possible. You decide the height of the ladder or staircase when casting the spell, up to a maximum height of 40 feet. The ladder or staircase is locked in place and magically supported, allowing you to ascend even if it's in an open area. The conjured ladder is simple in design and made of wood. The staircase is a spiral staircase made of wood. While both are supported and have no risk of falling, they can be damaged and destroyed as normal. The staircase is typically easier to ascend, though it's less discreet than a ladder and could possibly draw more attention. You can Dismiss the spell.

Heightened (+1) The maximum height increases by 40 feet.`},
	`helpful_steps_fanatic_doctrine`: {Name: `Helpful Steps`, Description: `You call forth a ladder or staircase to help you reach greater heights. The ladder or staircase appears in a space you designate and either stands freely or connects to a nearby wall if possible. You decide the height of the ladder or staircase when casting the spell, up to a maximum height of 40 feet. The ladder or staircase is locked in place and magically supported, allowing you to ascend even if it's in an open area. The conjured ladder is simple in design and made of wood. The staircase is a spiral staircase made of wood. While both are supported and have no risk of falling, they can be damaged and destroyed as normal. The staircase is typically easier to ascend, though it's less discreet than a ladder and could possibly draw more attention. You can Dismiss the spell.

Heightened (+1) The maximum height increases by 40 feet.`},
	`helpful_steps_neural`: {Name: `Helpful Steps`, Description: `You call forth a ladder or staircase to help you reach greater heights. The ladder or staircase appears in a space you designate and either stands freely or connects to a nearby wall if possible. You decide the height of the ladder or staircase when casting the spell, up to a maximum height of 40 feet. The ladder or staircase is locked in place and magically supported, allowing you to ascend even if it's in an open area. The conjured ladder is simple in design and made of wood. The staircase is a spiral staircase made of wood. While both are supported and have no risk of falling, they can be damaged and destroyed as normal. The staircase is typically easier to ascend, though it's less discreet than a ladder and could possibly draw more attention. You can Dismiss the spell.

Heightened (+1) The maximum height increases by 40 feet.`},
	`helpful_steps_technical`: {Name: `Deployable Step Scaffold`, Description: `Rapidly extrude a lightweight scaffold structure providing footing across obstacles.`},
	`hippocampus_retreat_bio_synthetic`: {Name: `Hippocampus Retreat`, Description: `Requirements You're mostly or totally submerged in water.You temporarily shape your lower limbs into the tail of a hippocampus in order to swim away from a nearby foe after dealing a parting blow. Attempt a melee spell attack roll against the target's AC, dealing 2d6 bludgeoning damage on a hit (or double damage on a critical hit). Then, Swim up to 30 feet; if you already have a swim Speed, you can Swim up to your Speed with a +10-foot circumstance bonus. You gain a +2 circumstance bonus to your AC against reactions triggered by this movement. At the end of the movement, your lower limbs return to normal.
@UUID[Compendium.pf2e.spell-effects.Item.Spell Effect: Hippocampus Retreat]Heightened (+1) The damage increases by 1d6.`},
	`hippocampus_retreat_technical`: {Name: `Aquatic Escape Thruster`, Description: `Activate a compressed-water thruster pack for rapid aquatic movement and retreat.`},
	`horizon_thunder_sphere_bio_synthetic`: {Name: `Horizon Thunder Sphere`, Description: `You gather magical energy into your palm, forming a concentrated ball of electricity that crackles and rumbles like impossibly distant thunder. Make a ranged spell attack roll against your target's AC. On a success, you deal 3d6 electricity damage. On a critical success, the target takes double damage and is @UUID[Compendium.pf2e.conditionitems.Item.Dazzled] for 1 round. The number of actions you spend when Casting this Spell determines the range and other parameters.
2 This spell has a range of 30 feet.
3 This spell has a range of 60 feet and deals half damage on a failure (but not a critical failure) as the electricity lashes out and jolts the target.
Two Rounds If you spend 3 actions Casting the Spell, you can avoid finishing the spell and spend another 3 actions on your next turn to empower the spell even further. If you do, after attacking the target, whether you hit or miss, the ball of lightning explodes, dealing @Damage[(@item.rank*2)d6[electricity]] damage to all other creatures in a @Template[emanation|distance:10] around the target (basic Reflex save). Additionally, you spark with electricity for 1 minute, dealing @Damage[(@item.rank)[electricity]] damage to creatures that Grab you or that hit you with an unarmed Strike or a non-reach melee weapon.

Heightened (+1) The initial damage on a hit, as well as the burst damage for two-round casting time, each increase by 2d6, and the damage creatures take if they Grapple or hit you while you're in your sparking state increases by 1.`},
	`horizon_thunder_sphere_technical`: {Name: `Charged Ball Launcher`, Description: `Fire a rolling electromagnetic sphere that builds charge as it travels before detonating.`},
	`hydraulic_push_bio_synthetic`: {Name: `Hydraulic Push`, Description: `You call forth a powerful blast of pressurized water that bludgeons the target and knocks it back. Make a ranged spell attack roll.

Critical Success The target takes 6d6 bludgeoning damage and is knocked back 10 feet.
Success The target takes 3d6 bludgeoning damage and is knocked back 5 feet.

Heightened (+1) The bludgeoning damage increases by 2d6.`},
	`hydraulic_push_technical`: {Name: `High-Pressure Fluid Cannon`, Description: `Fire a high-velocity fluid slug from a compact cannon, delivering massive knockback.`},
	`ignition_bio_synthetic`: {Name: `Contact Igniter Compound`, Description: `You apply a contact-ignition compound that sets the target alight, dealing fire damage on touch.`},
	`ill_omen`: {Name: `Ill Omen`, Description: `The target is struck with misfortune, which throws it off balance. The target must attempt a Will save.
Success The target is unaffected.
Failure The first time during the duration that the target attempts an attack roll or skill check, it must roll twice and use the worse result.
Critical Failure Every time during the duration that the target attempts an attack roll or skill check, it must roll twice and use the worse result.
@UUID[Compendium.pf2e.spell-effects.Item.Spell Effect: Ill Omen]`},
	`illuminate_bio_synthetic`: {Name: `Remote Bioluminescence Activation`, Description: `You broadcast a biochemical trigger that activates all bioluminescent or passive light sources in the area.`},
	`illuminate_fanatic_doctrine`: {Name: `Doctrine's Illumination`, Description: `The Doctrine's light floods the area, activating all dormant light sources simultaneously.`},
	`illuminate_neural`: {Name: `Light Source Activation`, Description: `You remotely trigger all non-active light sources in the area, bringing them to full brightness simultaneously.`},
	`illusory_disguise_neural`: {Name: `Illusory Disguise`, Description: `You create an illusion that causes the target to appear as another creature of the same body shape, and with roughly similar height (within 6 inches) and weight (within 50 pounds). The disguise is typically good enough to hide their identity, but not to impersonate a specific individual. The spell changes their appearance and voice, but not mannerisms. You can change the appearance of its clothing and worn items, such as making its armor look like a dress. Held items are unaffected, and any worn item removed from the creature returns to its true appearance.
Casting illusory disguise counts as setting up a disguise for the @UUID[Compendium.pf2e.actionspf2e.Item.Impersonate] use of Deception; it ignores any circumstance penalties the target might take for disguising itself as a dissimilar creature, gives a +4 status bonus to Deception checks to prevent others from seeing through the disguise, and lets the target add its level to such Deception checks even if untrained. You can Dismiss this spell.

Heightened (3rd) The target can appear as any creature of the same size, even a specific individual. You must have seen an individual to replicate its appearance, and must have heard its voice to replicate its voice.
Heightened (4th) You can target up to 10 willing creatures. If you target multiple creatures, you can choose a different disguise for each target, but none can impersonate a specific individual. You can Dismiss each disguise individually or all collectively.
Heightened (7th) As 4th, but you can choose disguises that impersonate specific individuals. You must have seen an individual to replicate its appearance, and must have heard its voice to replicate its voice.
@UUID[Compendium.pf2e.spell-effects.Item.Spell Effect: Illusory Disguise]`},
	`illusory_disguise_technical`: {Name: `Full-Body Holographic Overlay`, Description: `Project a full-body holographic skin that convincingly disguises the user's appearance.`},
	`illusory_object_neural`: {Name: `Illusory Object`, Description: `You create an illusory visual image of a stationary object. The entire image must fit within the spell's area. The object appears to animate naturally, but it doesn't make sounds or generate smells. For example, water would appear to pour down an illusory waterfall, but it would be silent.
Any creature that touches the image or uses the @UUID[Compendium.pf2e.actionspf2e.Item.Seek] action to examine it can attempt to disbelieve your illusion.

Heightened (2nd) Your image makes appropriate sounds, generates normal smells, and feels right to the touch. The spell gains the auditory and olfactory traits. The duration increases to 1 hour.
Heightened (5th) As the 2nd-rank version, but the duration is unlimited.`},
	`illusory_object_technical`: {Name: `Persistent Hard-Light Projection`, Description: `Generate a convincing hard-light projection of any object within size parameters.`},
	`imprint_message`: {Name: `Imprint Message`, Description: `You project psychic vibrations onto the target object, imprinting it with a short message or emotional theme of your design. This imprinted sensation is revealed to a creature who casts @UUID[Compendium.pf2e.spells-srd.Item.Object Reading] on the target object, replacing any emotional events the item was present for. If the object is in the area of a @UUID[Compendium.pf2e.spells-srd.Item.Retrocognition] spell, the imprinted messages appear as major events in the timeline, but they don't interfere with any other visions.
If the object is targeted with @UUID[Compendium.pf2e.spells-srd.Item.Read Aura] of a higher spell rank than imprint message, the caster learns that the object has been magically modified. When you Cast this Spell, any prior vibrations placed on an object by previous castings of imprint message fade.`},
	`infectious_enthusiasm_neural`: {Name: `Morale Contagion`, Description: `You broadcast a surge of manufactured enthusiasm that spreads from target to target, boosting their effectiveness.`},
	`infuse_vitality`: {Name: `Infuse Vitality`, Description: `You empower attacks with vital energy. The number of targets is equal to the number of actions you spent casting this spell. Each target's unarmed and weapon Strikes deal an extra 1d4 vitality damage. (This damage typically damages only undead). If you have the holy trait, you can add that trait to this spell and to the Strikes affected by the spell.

Heightened (3rd) The damage increases to 2d4 damage.
Heightened (5th) The damage increases to 3d4 damage.
@UUID[Compendium.pf2e.spell-effects.Item.Spell Effect: Infuse Vitality]`},
	`inkshot_bio_synthetic`: {Name: `Inkshot`, Description: `A spray of viscous, toxic ink jets from your fingertip to strike a target creature in the face. Make a spell attack roll against the target. On a hit, you deal 2d6 poison damage, plus you blast the target's eyes, making them @UUID[Compendium.pf2e.conditionitems.Item.Dazzled] for 1 round as the stinging ink blurs and distorts the creature's vision. On a critical hit, double the poison damage, and the target becomes dazzled for 1 minute by the foul ink.
The ink stain remains for 1 hour before fading, although vigorous cleansing (or magic such as a prestidigitation cantrip) can remove the ink before then.

Heightened (+1) Increase the base poison damage by 2d6.`},
	`inkshot_neural`: {Name: `Inkshot`, Description: `A spray of viscous, toxic ink jets from your fingertip to strike a target creature in the face. Make a spell attack roll against the target. On a hit, you deal 2d6 poison damage, plus you blast the target's eyes, making them @UUID[Compendium.pf2e.conditionitems.Item.Dazzled] for 1 round as the stinging ink blurs and distorts the creature's vision. On a critical hit, double the poison damage, and the target becomes dazzled for 1 minute by the foul ink.
The ink stain remains for 1 hour before fading, although vigorous cleansing (or magic such as a prestidigitation cantrip) can remove the ink before then.

Heightened (+1) Increase the base poison damage by 2d6.`},
	`inside_ropes_bio_synthetic`: {Name: `Combat Analysis Gland`, Description: `You activate a situational analysis gland that feeds you real-time combat data on the target's patterns.`},
	`inside_ropes_fanatic_doctrine`: {Name: `Doctrine's Combat Revelation`, Description: `The Doctrine reveals the target's weaknesses through ideological insight, granting real-time tactical data on their combat patterns.`},
	`inside_ropes_neural`: {Name: `Combat Data Feed`, Description: `You tap a real-time data feed about the target's tactical tendencies, gaining insight that helps you predict and counter their moves.`},
	`instant_pottery_bio_synthetic`: {Name: `Instant Pottery`, Description: `You pull earthen material out of the environment, then shape it into one or more earthenware objects that, in combination, can be up to light Bulk. Alternatively, you can cast this spell on objects previously created with this spell, extending their duration. No object can have intricate artistry or complex moving parts, can fulfill a cost or the like, or is made of anything more than clay or earth. Each object is obviously the product of temporary magic and thus can't be sold or passed off as a valuable item.

Heightened (2nd) You can create objects of up to 1 Bulk. They last 8 hours.
Heightened (3rd) You can create objects of up to 2 Bulk. They last 24 hours.`},
	`instant_pottery_technical`: {Name: `Rapid Material Former`, Description: `Use a focused micro-printer to shape clay or soft materials into useful forms instantly.`},
	`interposing_earth_bio_synthetic`: {Name: `Interposing Earth`, Description: `Trigger You are the target of a Strike or would attempt a Reflex save against a damaging area effect.

You raise a flimsy barrier of earth to shield you from harm. This barrier is 1 inch thick, 5 feet long, 5 feet high, and must be placed on the border between two squares. This barrier appears between you and the source of the triggering effect and grants you standard cover against the triggering effect. If you would be damaged by the triggering effect despite this barrier, the barrier is destroyed, and the damage dealt to you is reduced by 2. The barrier remains in place for 3 rounds (or until destroyed). It has AC 5, 2 Hardness, and 5 Hit Points.

Heightened (4th) The damage reduced increases to 8, the barrier's hardness increases to 8, and the barrier's Hit Points increases to 20.`},
	`interposing_earth_technical`: {Name: `Terrain Shield Actuator`, Description: `Trigger a ground-displacement actuator that raises a barrier of earth between attacker and target.`},
	`invisible_item_neural`: {Name: `Invisible Item`, Description: `You make the object @UUID[Compendium.pf2e.conditionitems.Item.Invisible]. This makes it @UUID[Compendium.pf2e.conditionitems.Item.Undetected] to all creatures, though the creatures can attempt to find the target, making it @UUID[Compendium.pf2e.conditionitems.Item.Hidden] to them instead if they succeed. If the item is used as part of a hostile action, the spell ends after that hostile action is completed. Making a weapon invisible typically doesn't give any advantage to the attack, except that an invisible thrown weapon or piece of ammunition can be used for an attack without necessarily giving information about the attacker's hiding place unless the weapon returns to the attacker.Heightened (3rd) The duration is until the next time you make your daily preparations.
Heightened (7th) The duration is unlimited.`},
	`invisible_item_technical`: {Name: `Optical Cloaking Wrap`, Description: `Apply a light-bending metamaterial wrap that renders an item effectively invisible.`},
	`invoke_true_name_bio_synthetic`: {Name: `Biological Signature Lock`, Description: `You identify a target's unique biological signature, establishing a chemical resonance that makes your compounds more effective against them.`},
	`invoke_true_name_fanatic_doctrine`: {Name: `True Designation`, Description: `You speak a target's true identifier in the Doctrine's language, establishing a resonance lock that makes your protocols more effective.`},
	`invoke_true_name_neural`: {Name: `Designation Protocol`, Description: `You speak the target's unique cognitive identifier, establishing a resonance link that makes your protocols more effective against them.`},
	`item_facade_neural`: {Name: `Item Facade`, Description: `You make the target object look and feel as though it were in much better or worse physical condition. When you cast this spell, decide whether you want to make the object look decrepit or perfect. An item made to look decrepit appears @UUID[Compendium.pf2e.conditionitems.Item.Broken] and shoddy. An intact item made to look better appears as though it's brand new and highly polished or well maintained. A Broken item appears to be intact and functional. Destroyed items can't be affected by this spell. A creature that Interacts with the item can attempt to disbelieve the illusion.

Heightened (2nd) The duration is 24 hours.
Heightened (3rd) The duration is unlimited.`},
	`item_facade_technical`: {Name: `Object Holographic Skin`, Description: `Project a holographic overlay onto an object to disguise its true nature.`},
	`join_pasts`: {Name: `Memory Bridge`, Description: `You establish a direct neural link between two targets, letting them share and experience each other's memories.`},
	`jump_bio_synthetic`: {Name: `Jump`, Description: `Your legs surge with strength, ready to leap high and far. You jump 30 feet in any direction without touching the ground. You must land on a space of solid ground within 30 feet of you, or else you fall after using your next action.

Heightened (3rd) The range becomes touch, the target changes to one touched creature, and the duration becomes 1 minute, allowing the target to jump as described whenever it takes the @UUID[Compendium.pf2e.actionspf2e.Item.Leap] action.`},
	`jump_technical`: {Name: `Jump Jet Assist Pack`, Description: `Trigger a brief jump-jet burst for dramatically enhanced vertical or horizontal leap.`},
	`juvenile_companion`: {Name: `Juvenile Companion`, Description: `You transform your companion into its juvenile form, such as a cub, foal, kitten, puppy, or piglet, making the target appear harmless. It becomes Tiny (if it was larger), and its reach is reduced to 0 feet. All of its Speeds are halved (to a minimum Speed of 5 feet), and it gains weakness 5 to physical damage. In all other ways, its abilities and statistics are unchanged.
If your companion uses a hostile action, juvenile companion ends. This spell has no effect on a companion that doesn't have a juvenile form.
@UUID[Compendium.pf2e.spell-effects.Item.Spell Effect: Juvenile Companion]

Heightened (2nd) The duration increases to 1 hour.`},
	`kinetic_ram_neural`: {Name: `Kinetic Ram`, Description: `Gathering kinetic energy, you either focus it in a straight line or disperse it as an encircling ripple. Any creature targeted by this spell must succeed at a Fortitude saving throw or be pushed 10 feet away from you (or 20 feet on a critical failure).
The spell's area or range and how many creatures it affects is based on how many actions you spend when Casting the Spell.1 The spell targets one creature within 15 feet.
2 The spell targets one creature within 30 feet. The distance the target is pushed if it fails is doubled, and on a critical failure, the target is also knocked @UUID[Compendium.pf2e.conditionitems.Item.Prone] and takes @Damage[1d6[bludgeoning]] damage.
3 The spell targets all creatures in a @Template[emanation|distance:5].`},
	`kinetic_ram_technical`: {Name: `Kinetic Impact Projector`, Description: `Fire a dense kinetic impactor at a target, delivering massive concussive force.`},
	`know_location_bio_synthetic`: {Name: `Know Location`, Description: `You create an invisible anchor at a location within range (even if it's outside your line of sight or line of effect), as long as you can identify the location by its appearance (or other identifying features). You innately know the direction towards that location, including relative depth, but not the distance. Incorrect knowledge of the location's appearance usually causes the spell to fail, but it could instead lead to an unwanted location or some other unusual mishap determined by the GM. This spell doesn't help you find a suitable route to the location nor assist you in overcoming obstacles on the way there.

Heightened (3rd) The range is 10 miles.
Heightened (5th) The range is 100 miles.
Heightened (7th) The range is planetary and you can create an anchor at a location you've viewed with scrying or similar effects.`},
	`know_location_fanatic_doctrine`: {Name: `Know Location`, Description: `You create an invisible anchor at a location within range (even if it's outside your line of sight or line of effect), as long as you can identify the location by its appearance (or other identifying features). You innately know the direction towards that location, including relative depth, but not the distance. Incorrect knowledge of the location's appearance usually causes the spell to fail, but it could instead lead to an unwanted location or some other unusual mishap determined by the GM. This spell doesn't help you find a suitable route to the location nor assist you in overcoming obstacles on the way there.

Heightened (3rd) The range is 10 miles.
Heightened (5th) The range is 100 miles.
Heightened (7th) The range is planetary and you can create an anchor at a location you've viewed with scrying or similar effects.`},
	`know_location_neural`: {Name: `Know Location`, Description: `You create an invisible anchor at a location within range (even if it's outside your line of sight or line of effect), as long as you can identify the location by its appearance (or other identifying features). You innately know the direction towards that location, including relative depth, but not the distance. Incorrect knowledge of the location's appearance usually causes the spell to fail, but it could instead lead to an unwanted location or some other unusual mishap determined by the GM. This spell doesn't help you find a suitable route to the location nor assist you in overcoming obstacles on the way there.

Heightened (3rd) The range is 10 miles.
Heightened (5th) The range is 100 miles.
Heightened (7th) The range is planetary and you can create an anchor at a location you've viewed with scrying or similar effects.`},
	`know_the_way_bio_synthetic`: {Name: `Biological Compass Calibration`, Description: `You activate your internal biological compass, immediately orienting to your precise position and direction.`},
	`know_the_way_fanatic_doctrine`: {Name: `Doctrine's Direction`, Description: `You invoke the Doctrine's guidance, instantly knowing your precise position and orientation.`},
	`know_the_way_neural`: {Name: `Internal Navigation System`, Description: `You trigger an internal compass recalibration, immediately knowing your precise orientation and direction.`},
	`leaden_steps_bio_synthetic`: {Name: `Leaden Steps`, Description: `You partially transform a foe's feet into unwieldy slabs of metal, slowing their steps. The target attempts a Fortitude saving throw.Critical Success The target is unaffected.
Success The target is @UUID[Compendium.pf2e.conditionitems.Item.Encumbered] and has weakness 2 to electricity until the end of your next turn. The spell can't be sustained.
Failure The target is encumbered and has weakness 2 to electricity.
Critical Failure The target is encumbered and has weakness 3 to electricity.
@UUID[Compendium.pf2e.spell-effects.Item.Spell Effect: Leaden Steps]Heightened (+1) The weakness increases by 1.`},
	`leaden_steps_technical`: {Name: `Magnetic Anchor System`, Description: `Activate electromagnetic floor anchors that lock a target's feet in place.`},
	`liberating_command`: {Name: `Liberating Command`, Description: `You call out a liberating cry, urging an ally to break free of an effect that holds them in place. If the target is @UUID[Compendium.pf2e.conditionitems.Item.Grabbed], @UUID[Compendium.pf2e.conditionitems.Item.Immobilized], or @UUID[Compendium.pf2e.conditionitems.Item.Restrained], it can immediately use a reaction to attempt to @UUID[Compendium.pf2e.actionspf2e.Item.Escape].`},
	`light_bio_synthetic`: {Name: `Bioluminescent Orb`, Description: `You excrete a sustained bioluminescent compound that forms a floating orb, illuminating a wide area for the duration.`},
	`light_fanatic_doctrine`: {Name: `Doctrine's Light`, Description: `You manifest a sustained orb of the Doctrine's sacred light, illuminating a wide area for the faithful.`},
	`light_neural`: {Name: `Photon Emitter`, Description: `You project a sustained orb of bright photonic light that illuminates a wide area for the duration.`},
	`live_wire_bio_synthetic`: {Name: `Electro-Filament Deploy`, Description: `You extrude a length of conductive bio-filament charged with electricity, dealing damage to anything that touches it.`},
	`lock_fanatic_doctrine`: {Name: `Lock`, Description: `The target's latch mechanism clinks shut, held fast by unseen magical wards. When you magically lock a target, you set an Athletics and Thievery DC to open it equal to your spell DC or the base lock DC with a +4 status bonus, whichever is higher. Any key or combination that once opened a lock affected by this spell does not do so for the duration of the spell, though the key or combination does grant a +4 circumstance bonus to checks to open the door. If the target is opened, the spell ends. Assuming the target is not barred or locked in some additional way, you can unlock and open it with an Interact action during which you touch the target. This does not end the spell. You can Dismiss this spell at any time and from any distance.

Heightened (2nd) The duration is unlimited, but you must expend 6 gp worth of precious metals as an additional cost.`},
	`lock_neural`: {Name: `Lock`, Description: `The target's latch mechanism clinks shut, held fast by unseen magical wards. When you magically lock a target, you set an Athletics and Thievery DC to open it equal to your spell DC or the base lock DC with a +4 status bonus, whichever is higher. Any key or combination that once opened a lock affected by this spell does not do so for the duration of the spell, though the key or combination does grant a +4 circumstance bonus to checks to open the door. If the target is opened, the spell ends. Assuming the target is not barred or locked in some additional way, you can unlock and open it with an Interact action during which you touch the target. This does not end the spell. You can Dismiss this spell at any time and from any distance.

Heightened (2nd) The duration is unlimited, but you must expend 6 gp worth of precious metals as an additional cost.`},
	`lock_technical`: {Name: `Electronic Deadbolt Override`, Description: `Remotely engage electronic deadbolts and security locks through radio command.`},
	`lose_the_path_bio_synthetic`: {Name: `Lose the Path`, Description: `Trigger A creature in range Strides

You surround a moving creature with lifelike illusions, shifting their perception of the terrain to subtly lead them off course. The target must attempt a Will save. Regardless of the result, the creature is immune to lose the path for 1 hour.

Success The creature is unaffected.
Failure The creature treats all squares as difficult terrain for its Stride.
Critical Failure As failure, except that you determine where the target moves during the Stride, though you can't move it into hazardous terrain or to a place it can't stand.`},
	`lose_the_path_neural`: {Name: `Lose the Path`, Description: `Trigger A creature in range Strides

You surround a moving creature with lifelike illusions, shifting their perception of the terrain to subtly lead them off course. The target must attempt a Will save. Regardless of the result, the creature is immune to lose the path for 1 hour.

Success The creature is unaffected.
Failure The creature treats all squares as difficult terrain for its Stride.
Critical Failure As failure, except that you determine where the target moves during the Stride, though you can't move it into hazardous terrain or to a place it can't stand.`},
	`magic_stone_bio_synthetic`: {Name: `Magic Stone`, Description: `You pour vitality energy into ordinary stones, granting them temporary magical properties. You can target 1 non-magical stone or sling bullet for every action you use Casting this Spell. The stones must be unattended or carried by you or a willing ally. The stones become +1 striking disrupting @UUID[Compendium.pf2e.equipment-srd.Item.Sling Bullets]. Each stone can be used only once, after which it crumbles to dust.
@UUID[Compendium.pf2e.spell-effects.Item.Spell Effect: Magic Stone]`},
	`magic_stone_fanatic_doctrine`: {Name: `Magic Stone`, Description: `You pour vitality energy into ordinary stones, granting them temporary magical properties. You can target 1 non-magical stone or sling bullet for every action you use Casting this Spell. The stones must be unattended or carried by you or a willing ally. The stones become +1 striking disrupting @UUID[Compendium.pf2e.equipment-srd.Item.Sling Bullets]. Each stone can be used only once, after which it crumbles to dust.
@UUID[Compendium.pf2e.spell-effects.Item.Spell Effect: Magic Stone]`},
	`malediction`: {Name: `Malediction`, Description: `You incite distress in the minds of your enemies, making it more difficult for them to defend themselves. Enemies in the area must succeed at a Will save or take a –1 status penalty to AC as long as they're in the area.
Once per round on subsequent turns, you can Sustain the spell to increase the emanation's radius by 10 feet and force enemies in the area that weren't yet affected to attempt a saving throw.
Malediction can counteract benediction.
@UUID[Compendium.pf2e.spell-effects.Item.Spell Effect: Malediction]`},
	`mending_bio_synthetic`: {Name: `Mending`, Description: `You repair the target item. You restore 5 Hit Points per spell rank to the target, potentially removing the @UUID[Compendium.pf2e.conditionitems.Item.Broken] condition if this repairs it past the item's Broken Threshold. You can't replace lost pieces or repair an object that's been completely destroyed.

Heightened (2nd) You can target a non-magical object of 1 Bulk or less.
Heightened (3rd) You can target a non-magical object of 2 Bulk or less, or a magical object of 1 Bulk or less.`},
	`mending_fanatic_doctrine`: {Name: `Mending`, Description: `You repair the target item. You restore 5 Hit Points per spell rank to the target, potentially removing the @UUID[Compendium.pf2e.conditionitems.Item.Broken] condition if this repairs it past the item's Broken Threshold. You can't replace lost pieces or repair an object that's been completely destroyed.

Heightened (2nd) You can target a non-magical object of 1 Bulk or less.
Heightened (3rd) You can target a non-magical object of 2 Bulk or less, or a magical object of 1 Bulk or less.`},
	`mending_neural`: {Name: `Mending`, Description: `You repair the target item. You restore 5 Hit Points per spell rank to the target, potentially removing the @UUID[Compendium.pf2e.conditionitems.Item.Broken] condition if this repairs it past the item's Broken Threshold. You can't replace lost pieces or repair an object that's been completely destroyed.

Heightened (2nd) You can target a non-magical object of 1 Bulk or less.
Heightened (3rd) You can target a non-magical object of 2 Bulk or less, or a magical object of 1 Bulk or less.`},
	`mending_technical`: {Name: `Nano-Repair Injector`, Description: `Apply a nano-repair compound that rapidly closes wounds and restores structural integrity.`},
	`message_fanatic_doctrine`: {Name: `Doctrine's Word`, Description: `You transmit a private message in the Doctrine's name directly to the target's mind, bypassing all distance.`},
	`message_neural`: {Name: `Subvocal Transmission`, Description: `You transmit a whispered message directly to the auditory cortex of a target anywhere in range — bypassing distance as a silent direct-line communication.`},
	`message_rune_neural`: {Name: `Message Rune`, Description: `You record a message up to 5 minutes long and inscribe a special rune on any flat unattended surface or small object within reach. The nature of the rune's appearance is up to you, but it's visible to everyone, and it must be no smaller than 2 inches in diameter. You also specify a trigger that creatures must meet to activate the rune.
For the duration of the spell, creatures that meet the criteria of the trigger can touch the rune to hear the recorded message in their head as though you were speaking to them telepathically. You know when someone is listening to the message, but you don't know who's listening to it. You can Dismiss the spell.Heightened (+2) The duration increases for every 2 ranks, becoming 1 week, 1 month, 1 year, or unlimited respectively.`},
	`message_rune_technical`: {Name: `Embedded Comm Chip`, Description: `Implant a micro-transmitter in a surface that broadcasts a stored message when triggered.`},
	`mind_spike`: {Name: `Mind Spike`, Description: `A focused neural disruption that scrambles a target's cognition.`},
	`mindlink_neural`: {Name: `Mindlink`, Description: `You link your mind to the target's mind and mentally impart to that target an amount of information in an instant that could otherwise be communicated in 10 minutes.`},
	`mindlink_technical`: {Name: `Neural Direct-Link Bridge`, Description: `Establish a direct neural communication channel between two linked augmentation systems.`},
	`moisture_reclaim`: {Name: `Moisture Reclaim`, Description: `Atmospheric condensation filters extract potable water from ambient humidity.`},
	`mud_pit_bio_synthetic`: {Name: `Mud Pit`, Description: `Thick, clinging mud covers the ground, 1 foot deep. The mud is difficult terrain.`},
	`mud_pit_technical`: {Name: `Terrain Destabilizer Charge`, Description: `Detonate a water-injection charge that turns solid ground into a treacherous mud pit.`},
	`musical_accompaniment_neural`: {Name: `Sonic Rhythm Interface`, Description: `You project an ambient musical rhythm through nearby receivers, granting bonuses to performance-related actions.`},
	`mystic_armor_bio_synthetic`: {Name: `Mystic Armor`, Description: `You ward yourself with shimmering magical energy, gaining a +1 item bonus to AC and a maximum Dexterity modifier of +5. While wearing mystic armor, you use your unarmored proficiency to calculate your AC.
@UUID[Compendium.pf2e.spell-effects.Item.Spell Effect: Mystic Armor]

Heightened (4th) You gain a +1 item bonus to saving throws.
Heightened (6th) The item bonus to AC increases to +2, and you gain a +1 item bonus to saving throws.
Heightened (8th) The item bonus to AC increases to +2, and you gain a +2 item bonus to saving throws.
Heightened (10th) The item bonus to AC increases to +3, and you gain a +3 item bonus to saving throws.`},
	`mystic_armor_fanatic_doctrine`: {Name: `Mystic Armor`, Description: `You ward yourself with shimmering magical energy, gaining a +1 item bonus to AC and a maximum Dexterity modifier of +5. While wearing mystic armor, you use your unarmored proficiency to calculate your AC.
@UUID[Compendium.pf2e.spell-effects.Item.Spell Effect: Mystic Armor]

Heightened (4th) You gain a +1 item bonus to saving throws.
Heightened (6th) The item bonus to AC increases to +2, and you gain a +1 item bonus to saving throws.
Heightened (8th) The item bonus to AC increases to +2, and you gain a +2 item bonus to saving throws.
Heightened (10th) The item bonus to AC increases to +3, and you gain a +3 item bonus to saving throws.`},
	`mystic_armor_neural`: {Name: `Mystic Armor`, Description: `You ward yourself with shimmering magical energy, gaining a +1 item bonus to AC and a maximum Dexterity modifier of +5. While wearing mystic armor, you use your unarmored proficiency to calculate your AC.
@UUID[Compendium.pf2e.spell-effects.Item.Spell Effect: Mystic Armor]

Heightened (4th) You gain a +1 item bonus to saving throws.
Heightened (6th) The item bonus to AC increases to +2, and you gain a +1 item bonus to saving throws.
Heightened (8th) The item bonus to AC increases to +2, and you gain a +2 item bonus to saving throws.
Heightened (10th) The item bonus to AC increases to +3, and you gain a +3 item bonus to saving throws.`},
	`mystic_armor_technical`: {Name: `Force Field Emitter`, Description: `Project a personal force field that absorbs incoming kinetic and energy damage.`},
	`nanite_infusion`: {Name: `Nanite Infusion`, Description: `Releases a cloud of salvaged medical nanites that accelerate tissue repair in a touched target.`},
	`necromancers_generosity_fanatic_doctrine`: {Name: `Necromancer's Generosity`, Description: `You channel void energy through your magical connection to your undead minion to strengthen the creature. The target regains 1d8+4 Hit Points when you Cast the Spell, and it gains a +2 status bonus to saves against vitality effects for the duration.
@UUID[Compendium.pf2e.spell-effects.Item.Spell Effect: Necromancer's Generosity]

Heightened (+1) The amount of healing increases by 1d8+4.`},
	`necromancers_generosity_technical`: {Name: `Biohazard Transfer Agent`, Description: `Apply a compound that transfers a debilitating condition from one subject to another.`},
	`needle_darts_bio_synthetic`: {Name: `Spine Launcher`, Description: `You extrude and launch three bio-synthetic spine projectiles simultaneously at targets in range.`},
	`needle_darts_fanatic_doctrine`: {Name: `Doctrine's Judgment Darts`, Description: `You manifest three needles of compressed doctrinal energy and launch them at targets within range.`},
	`needle_darts_neural`: {Name: `Needle Volley`, Description: `You shape three razor-sharp flechettes from available metal and launch them simultaneously at targets within range.`},
	`negate_aroma_bio_synthetic`: {Name: `Negate Aroma`, Description: `The target loses its odor, preventing creatures from passively noticing its presence via smell alone, even if the creatures have precise or imprecise scent. A creature attempting a Perception check to Seek with scent and other senses might notice the lack of natural scent. If the target has any abilities that result from its smell, such as an overpowering scent, those abilities are also negated.

Heightened (5th) The range increases to 30 feet, and you can target up to 10 creatures.`},
	`negate_aroma_technical`: {Name: `Scent Neutralization Spray`, Description: `Apply a chemical neutralizer that eliminates all detectable odors from the target.`},
	`nettleskin`: {Name: `Nettleskin`, Description: `Thorns sprout from your body; they pass through and don't damage any clothing or armor you wear.
Adjacent creatures that hit you with a melee or unarmed attack take 1d4 piercing damage as the nettles jab them and break off. Each time a creature takes damage in this way, nettleskin's duration decreases by 1 round.

Heightened (+1) The damage increases by 1d4.`},
	`neural_static`: {Name: `Neural Static`, Description: `Floods a target's sensory nerves with dissonant white-noise, slowing their reactions.`},
	`noxious_vapors_bio_synthetic`: {Name: `Noxious Vapors`, Description: `You emit a cloud of toxic smoke that temporarily obscures you from sight. Each creature except you in the area when you Cast the Spell takes 1d6 poison damage (basic Fortitude save). A creature that critically fails the saving throw also becomes Sickened 1. All creatures in the area become @UUID[Compendium.pf2e.conditionitems.Item.Concealed], and all creatures outside the smoke become concealed to creatures within it. This smoke can be dispersed by a strong wind.Heightened (+1) The damage increases by 1d6`},
	`noxious_vapors_technical`: {Name: `Toxic Gas Dispersal Canister`, Description: `Deploy a canister that releases a cloud of noxious gas in the target area.`},
	`nudge_the_odds_fanatic_doctrine`: {Name: `Nudge the Odds`, Description: `You bestow yourself supernaturally good luck at cards, dice, and other games of chance. You gain a +1 status bonus to Games Lore checks to gamble, and if you roll a critical failure on such a check, you get a failure instead; however, the spell is too short-lived to use for Earn Income checks from gambling.
When you're under the effect of nudge the odds, one facial feature, such as a lock of hair or the iris of an eye, transforms to a distinctive golden color; the GM chooses which feature when you cast the spell. This change resists all magical efforts to conceal it, though it can be hidden or covered by mundane means. A creature noticing the feature can identify the spell using Recall Knowledge. Because it prevents losing big, gamblers consider nudge the odds a repugnant form of cheating. If you're caught using the spell, you are likely to suffer serious consequences, depending on the nature of the gamblers you cheated.
@UUID[Compendium.pf2e.spell-effects.Item.Spell Effect: Nudge the Odds]

Heightened (5th) The status bonus increases to +2, and the duration increases to last until the next time you make your daily preparations. If you continue spending your spell slot to keep the duration active, this allows you to apply the effect to a downtime check to Earn Income.`},
	`nudge_the_odds_neural`: {Name: `Nudge the Odds`, Description: `You bestow yourself supernaturally good luck at cards, dice, and other games of chance. You gain a +1 status bonus to Games Lore checks to gamble, and if you roll a critical failure on such a check, you get a failure instead; however, the spell is too short-lived to use for Earn Income checks from gambling.
When you're under the effect of nudge the odds, one facial feature, such as a lock of hair or the iris of an eye, transforms to a distinctive golden color; the GM chooses which feature when you cast the spell. This change resists all magical efforts to conceal it, though it can be hidden or covered by mundane means. A creature noticing the feature can identify the spell using Recall Knowledge. Because it prevents losing big, gamblers consider nudge the odds a repugnant form of cheating. If you're caught using the spell, you are likely to suffer serious consequences, depending on the nature of the gamblers you cheated.
@UUID[Compendium.pf2e.spell-effects.Item.Spell Effect: Nudge the Odds]

Heightened (5th) The status bonus increases to +2, and the duration increases to last until the next time you make your daily preparations. If you continue spending your spell slot to keep the duration active, this allows you to apply the effect to a downtime check to Earn Income.`},
	`nudge_the_odds_technical`: {Name: `Luck Algorithm Adjustment`, Description: `Run a probability-tweak subroutine that biases one upcoming random outcome in your favor.`},
	`object_reading`: {Name: `Object Reading`, Description: `You place a hand on an object to learn a piece of information about an emotional event that occurred involving the object within the past week, determined by the GM. If you cast object reading on the same item multiple times, you can either concentrate on a single event to gain additional pieces of information about that event, or you can gain a piece of information about another emotional event in the applicable time frame.

Heightened (2nd) You can learn about an event that occurred within the last month.
Heightened (4th) You can learn about an event that occurred within the last year.
Heightened (6th) You can learn about an event that occurred within the last decade.
Heightened (8th) You can learn about an event that occurred within the last century.
Heightened (9th) You can learn about an event that occurred within the entirety of the object's history.`},
	`overselling_flourish_neural`: {Name: `Overselling Flourish`, Description: `Trigger A creature damages you.
You make a grand spectacle out of getting hit. Enhanced by magic, this spectacle features sprays of blood, anguished screams, or other theatrics that appear to result from your foe's attack. The triggering creature must attempt a Will saving throw.

Critical Success The creature is unaffected.
Success The creature is thrown off by your display. The creature becomes @UUID[Compendium.pf2e.conditionitems.Item.Dazzled] until the start of your turn.
Failure The creature fully believes your performance, leaving itself open. The creature becomes dazzled and @UUID[Compendium.pf2e.conditionitems.Item.Off-Guard] until the start of your turn.
Critical Failure The creature is enraptured by the display. The creature uses its remaining actions to watch you in awe. It then remains dazzled and off-guard until the start of your turn.`},
	`overselling_flourish_technical`: {Name: `Dramatic Overclock Effect`, Description: `Trigger a showy overclock sequence that dazzles observers and establishes technical dominance.`},
	`penumbral_shroud_neural`: {Name: `Penumbral Shroud`, Description: `You envelop the target in a shroud of shadow. The target perceives light as one step lower than it actually is (bright light becomes dim light, for example), affecting their ability to sense creatures and objects accordingly.
The shroud also provides the target a +1 status bonus to saving throws against light effects. This effect is helpful to creatures sensitive to light, and a creature can willingly choose to be subject to the failure effect of the spell.Critical Success The target is unaffected.
Success The effect lasts for 1 round.
Failure The effect lasts its normal duration.`},
	`penumbral_shroud_technical`: {Name: `Darkfield Shroud`, Description: `Deploy a portable darkfield emitter that absorbs ambient light around the target.`},
	`personal_rain_cloud_bio_synthetic`: {Name: `Personal Rain Cloud`, Description: `You conjure a 5-foot-wide rain cloud that follows the target wherever it goes. It stays roughly an arm's length overhead, unless it must drift lower to fit under a ceiling. The cloud rains constantly on the target, keeping it wet and dampening the ground in the wake of any movement. The rain extinguishes non-magical flames. The target gains fire resistance 2. Creatures with weakness to water take damage equal to their weakness at the end of each of their turns. Creatures can attempt a Reflex save to avoid the cloud.

Heightened (+1) The amount of fire resistance increases by 2.`},
	`personal_rain_cloud_technical`: {Name: `Targeted Precipitation Unit`, Description: `Position a micro-sprayer unit overhead that produces a localized rain effect.`},
	`pest_form_bio_synthetic`: {Name: `Pest Form`, Description: `You transform into the battle form of a Tiny animal, such as a cat, insect, lizard, or rat. You can decide the specific type of animal (such as a rat or praying mantis), but this has no effect on the form's Size or statistics. While in this form, you gain the animal trait and you can't make Strikes. You can Dismiss the spell.
You gain the following statistics and abilities:

AC = 15 + your level. Ignore your armor's check penalty and Speed reduction.
Speed 20 feet.
Weakness 5 to physical damage. (If you take physical damage in this form, you take 5 additional damage.)
Low-light vision and imprecise scent 30 feet.
Acrobatics and Stealth modifiers of +10, unless your own modifier is higher; Athletics modifier -4.


Heightened (4th) You can turn into a flying creature, such as a bird, which grants you a fly Speed of 20 feet.
@UUID[Compendium.pf2e.spell-effects.Item.Spell Effect: Pest Form]`},
	`pest_form_technical`: {Name: `Micro-Drone Disguise Shell`, Description: `Enter a compact drone chassis disguised as a small creature for infiltration purposes.`},
	`pet_cache_bio_synthetic`: {Name: `Pet Cache`, Description: `You open your cloak or create a gap with your hands, drawing the target into a pocket dimension just large enough for its basic comfort. No other creature can enter this extradimensional space, and the target can bring along objects only if they were designed to be worn by a creature of its kind. The space has enough air, food, and water to sustain the target for the duration.
You can @UUID[Compendium.pf2e.actionspf2e.Item.Dismiss] the spell. The spell also ends if you die or enter an extradimensional space. When the spell ends, the target reappears in the nearest unoccupied space (outside of any extradimensional space you may have entered).`},
	`pet_cache_fanatic_doctrine`: {Name: `Pet Cache`, Description: `You open your cloak or create a gap with your hands, drawing the target into a pocket dimension just large enough for its basic comfort. No other creature can enter this extradimensional space, and the target can bring along objects only if they were designed to be worn by a creature of its kind. The space has enough air, food, and water to sustain the target for the duration.
You can @UUID[Compendium.pf2e.actionspf2e.Item.Dismiss] the spell. The spell also ends if you die or enter an extradimensional space. When the spell ends, the target reappears in the nearest unoccupied space (outside of any extradimensional space you may have entered).`},
	`pet_cache_neural`: {Name: `Pet Cache`, Description: `You open your cloak or create a gap with your hands, drawing the target into a pocket dimension just large enough for its basic comfort. No other creature can enter this extradimensional space, and the target can bring along objects only if they were designed to be worn by a creature of its kind. The space has enough air, food, and water to sustain the target for the duration.
You can @UUID[Compendium.pf2e.actionspf2e.Item.Dismiss] the spell. The spell also ends if you die or enter an extradimensional space. When the spell ends, the target reappears in the nearest unoccupied space (outside of any extradimensional space you may have entered).`},
	`pet_cache_technical`: {Name: `Subspace Storage Unit`, Description: `Access a compressed-space storage module for deploying or retrieving a companion unit.`},
	`phantasmal_minion_neural`: {Name: `Phantasmal Minion`, Description: `You summon a @UUID[Compendium.pf2e.pathfinder-bestiary.Actor.Phantasmal Minion]. The minion is roughly the shape of a humanoid. You can choose to have it be @UUID[Compendium.pf2e.conditionitems.Item.Invisible] or have an ephemeral appearance, but it's obviously a magical effect, not a real creature.`},
	`phantasmal_minion_technical`: {Name: `Autonomous Holographic Decoy`, Description: `Deploy an autonomous holographic construct that mimics a combat-capable unit.`},
	`phantom_pain`: {Name: `Phantom Pain`, Description: `Illusory pain wracks the target, dealing 2d4 mental damage and @Damage[(@item.level)d4[persistent,mental]] damage. The target must attempt a Will save.Critical Success The target is unaffected.
Success The target takes full initial damage but no persistent damage, and the spell ends immediately.
Failure The target takes full initial and persistent damage, and the target is Sickened 1. If the target recovers from being Sickened, the persistent damage ends and the spell ends.
Critical Failure As failure, but the target is Sickened 2.Heightened (+1) The damage increases by 2d4 and the persistent damage by 1d4.`},
	`phase_bolt_neural`: {Name: `Phase Strike`, Description: `You project a bolt of focused psionic energy that passes through non-living material to strike its target.`},
	`pocket_library_neural`: {Name: `Pocket Library`, Description: `Like Vil Seral, you collect information from all around you and store it in book form in an extradimensional library. When you Cast this Spell, choose any skill in which you are at least trained that has the Recall Knowledge action.
During the duration of this spell, you can call forth a tome from the extradimensional library when attempting a Recall Knowledge check using your chosen skill. This is part of the action to Recall Knowledge. You must have a hand free to do so. The tome appears in your hand, open to an appropriate page. This grants you a +1 status bonus to the Recall Knowledge check. If you roll a critical failure on this check, you get a failure instead. If the roll is successful and the subject is a creature, you gain additional information or context about the creature. Once you reference a book from your pocket library, the spell ends.
@UUID[Compendium.pf2e.spell-effects.Item.Spell Effect: Pocket Library]Heightened (3rd) The status bonus increases to +2 and you can reference your pocket library twice before the spell ends.
Heightened (6th) The status bonus increases to +3 and you can reference your pocket library three times before the spell ends.
Heightened (9th) The status bonus increases to +4 and you can reference your pocket library four times before the spell ends.`},
	`pocket_library_technical`: {Name: `Portable Data Archive`, Description: `Access a compressed personal data archive containing extensive reference material.`},
	`pressure_burst`: {Name: `Pressure Burst`, Description: `A pneumatic compression rig vents a focused blast that shoves targets and shatters brittle obstacles.`},
	`prestidigitation_bio_synthetic`: {Name: `Minor Bio-Utility`, Description: `You perform minor low-level biological manipulations — cleaning, minor temperature regulation, or cosmetic micro-modifications.`},
	`prestidigitation_fanatic_doctrine`: {Name: `Minor Doctrine Cantrip`, Description: `You perform minor doctrinal manipulations — small cleanings, warmings, or cosmetic adjustments blessed by the Doctrine.`},
	`prestidigitation_neural`: {Name: `Minor Tech Cantrip`, Description: `You perform simple micro-level technological manipulations — minor cleaning, lighting, warming, or cosmetic effects.`},
	`protect_companion_bio_synthetic`: {Name: `Shared Bio-Field Extension`, Description: `You extend your bio-synthetic protective field to cover a nearby ally, granting them temporary AC protection.`},
	`protect_companion_fanatic_doctrine`: {Name: `Shared Doctrine Shield`, Description: `You extend the Doctrine's protective blessing over a nearby ally, granting them temporary AC protection.`},
	`protect_companion_neural`: {Name: `Shared Barrier Protocol`, Description: `You extend your personal force barrier to cover a nearby ally, granting them a temporary AC bonus.`},
	`protection_fanatic_doctrine`: {Name: `Protection`, Description: `You ward a creature against harm. The target gains a +1 status bonus to Armor Class and saving throws.

Heightened (3rd) You can choose to have the benefits also affect all your allies in a @Template[emanation|distance:10] around the target.
@UUID[Compendium.pf2e.spell-effects.Item.Spell Effect: Protection]`},
	`protection_neural`: {Name: `Protection`, Description: `You ward a creature against harm. The target gains a +1 status bonus to Armor Class and saving throws.

Heightened (3rd) You can choose to have the benefits also affect all your allies in a @Template[emanation|distance:10] around the target.
@UUID[Compendium.pf2e.spell-effects.Item.Spell Effect: Protection]`},
	`protector_tree`: {Name: `Protector Tree`, Description: `A Medium tree suddenly grows in an unoccupied square within range. The tree has AC 10 and 10 Hit Points. Whenever an ally adjacent to the tree is hit by a Strike, the tree interposes its branches and takes the damage first. Any additional damage beyond what it takes to reduce the tree to 0 Hit Points is dealt to the original target. The tree isn't large enough to impede movement through its square. If the tree is in soil and survives to the end of the spell's duration, it remains as an ordinary, non-magical tree and continues to grow and thrive. The GM might determine that the tree disappears immediately in certain inhospitable situations.Heightened (+1) The tree has an additional 10 Hit Points.`},
	`puff_of_poison_bio_synthetic`: {Name: `Puff of Poison`, Description: `You exhale a shimmering cloud of toxic breath at an enemy's face. The target takes 1d4 poison damage and @Damage[(ceil(@item.level/2))d4[persistent,poison]], depending on its Fortitude save.Critical Success The creature is unaffected.
Success The target takes half initial damage and no persistent damage.
Failure The target takes full initial and persistent damage.
Critical Failure The target takes double initial and persistent damage.Heightened (+2) The initial poison damage increases by 1d4, and the persistent poison damage increases by 1d4.`},
	`pummeling_rubble_bio_synthetic`: {Name: `Pummeling Rubble`, Description: `A spray of heavy rocks flies through the air in front of you. The rubble deals 2d4 bludgeoning damage to each creature in the area. Each creature must attempt a Reflex save.

Critical Success The creature is unaffected.
Success The creature takes half damage.
Failure The creature takes full damage and is pushed 5 feet away from you.
Critical Failure The creature takes double damage and is pushed 10 feet away from you.

Heightened (+1) Increase the damage by 2d4.`},
	`pummeling_rubble_technical`: {Name: `Debris Launch Barrage`, Description: `Magnetically accelerate loose debris at a target in a rapid barrage.`},
	`purifying_icicle_bio_synthetic`: {Name: `Purifying Icicle`, Description: `You evoke life essence into the form of water and freeze it, then launch the icicle at a foe. Make a spell attack roll. On a success, the icicle deals 2d6 piercing damage and 1d6 cold damage, and if the target is undead, the icicle deals an additional 1d4 vitality damage. On a critical success, the target takes double damage and takes a –10-foot circumstance penalty to its Speeds for 1 round as the icicle lodges inside them before melting away.

Heightened (+1) The piercing damage and cold damage each increase by 1d6. The vitality damage increases by 1d4.`},
	`purifying_icicle_fanatic_doctrine`: {Name: `Purifying Icicle`, Description: `You evoke life essence into the form of water and freeze it, then launch the icicle at a foe. Make a spell attack roll. On a success, the icicle deals 2d6 piercing damage and 1d6 cold damage, and if the target is undead, the icicle deals an additional 1d4 vitality damage. On a critical success, the target takes double damage and takes a –10-foot circumstance penalty to its Speeds for 1 round as the icicle lodges inside them before melting away.

Heightened (+1) The piercing damage and cold damage each increase by 1d6. The vitality damage increases by 1d4.`},
	`putrefy_food_and_drink_bio_synthetic`: {Name: `Putrefy Food and Drink`, Description: `You cause otherwise edible food to rot and spoil instantly, and water and other liquids to become brackish and undrinkable. @UUID[Compendium.pf2e.equipment-srd.Item.Holy Water], @UUID[Compendium.pf2e.equipment-srd.Item.Unholy Water], and similar food and drink of significance are spoiled by this spell, unless they are associated with a deity of decay or putrefaction, but it has no effect on creatures of any type, potions, or alchemical elixirs. One cubic foot of liquid is roughly 8 gallons.

Heightened (2nd) You can target an alchemical elixir with this spell, attempting a counteract check against it. If you succeed, the elixir spoils and becomes a mundane item.
Heightened (3rd) You can target a potion or alchemical elixir with this spell, attempting a counteract check against it. If you succeed, the elixir or potion spoils and becomes a mundane item.`},
	`putrefy_food_and_drink_fanatic_doctrine`: {Name: `Putrefy Food and Drink`, Description: `You cause otherwise edible food to rot and spoil instantly, and water and other liquids to become brackish and undrinkable. @UUID[Compendium.pf2e.equipment-srd.Item.Holy Water], @UUID[Compendium.pf2e.equipment-srd.Item.Unholy Water], and similar food and drink of significance are spoiled by this spell, unless they are associated with a deity of decay or putrefaction, but it has no effect on creatures of any type, potions, or alchemical elixirs. One cubic foot of liquid is roughly 8 gallons.

Heightened (2nd) You can target an alchemical elixir with this spell, attempting a counteract check against it. If you succeed, the elixir spoils and becomes a mundane item.
Heightened (3rd) You can target a potion or alchemical elixir with this spell, attempting a counteract check against it. If you succeed, the elixir or potion spoils and becomes a mundane item.`},
	`quick_sort_bio_synthetic`: {Name: `Quick Sort`, Description: `You magically sort a group of objects into neat stacks or piles. You can sort the objects in two different ways. The first option is to separate them into different piles depending on an easily observed factor, such as color or shape. Alternatively, you can sort the objects into ordered stacks depending on a clearly indicated notation, such as a page number, title, or date. The objects sort themselves throughout the duration, though it takes less time per object to sort a smaller number of objects, down to a single round for 30 or fewer objects.

Heightened (3rd) The spell can sort up to 400 objects in a minute, or 60 objects in a round.
Heightened (5th) The spell can sort up to 800 objects in a minute, or 120 objects in a round.`},
	`quick_sort_fanatic_doctrine`: {Name: `Quick Sort`, Description: `You magically sort a group of objects into neat stacks or piles. You can sort the objects in two different ways. The first option is to separate them into different piles depending on an easily observed factor, such as color or shape. Alternatively, you can sort the objects into ordered stacks depending on a clearly indicated notation, such as a page number, title, or date. The objects sort themselves throughout the duration, though it takes less time per object to sort a smaller number of objects, down to a single round for 30 or fewer objects.

Heightened (3rd) The spell can sort up to 400 objects in a minute, or 60 objects in a round.
Heightened (5th) The spell can sort up to 800 objects in a minute, or 120 objects in a round.`},
	`quick_sort_neural`: {Name: `Quick Sort`, Description: `You magically sort a group of objects into neat stacks or piles. You can sort the objects in two different ways. The first option is to separate them into different piles depending on an easily observed factor, such as color or shape. Alternatively, you can sort the objects into ordered stacks depending on a clearly indicated notation, such as a page number, title, or date. The objects sort themselves throughout the duration, though it takes less time per object to sort a smaller number of objects, down to a single round for 30 or fewer objects.

Heightened (3rd) The spell can sort up to 400 objects in a minute, or 60 objects in a round.
Heightened (5th) The spell can sort up to 800 objects in a minute, or 120 objects in a round.`},
	`quick_sort_technical`: {Name: `Automated Sorting Algorithm`, Description: `Run an automated sorting routine to rapidly organize items or data.`},
	`rainbows_end_bio_synthetic`: {Name: `Rainbow's End`, Description: `You reach upward to wrest down a rainbow and harness its power to connect this world to the heavens. Each creature in the area takes 1d4 spirit damage with a basic Fortitude save. Any creature that fails this save is additionally @UUID[Compendium.pf2e.conditionitems.Item.Dazzled] for 1 round. For the spell's duration, an ally who's adjacent to you can Interact and be instantly teleported to an unoccupied space in the spell's area, as long as they don't travel more than 60 feet. This effect has the teleportation trait.Heightened (+2) The damage increases by 2d4, the duration of the dazzled condition on a failed save increases by 1 round, and the maximum distance an ally can use the rainbow to teleport increases by 10 feet.`},
	`rainbows_end_fanatic_doctrine`: {Name: `Rainbow's End`, Description: `You reach upward to wrest down a rainbow and harness its power to connect this world to the heavens. Each creature in the area takes 1d4 spirit damage with a basic Fortitude save. Any creature that fails this save is additionally @UUID[Compendium.pf2e.conditionitems.Item.Dazzled] for 1 round. For the spell's duration, an ally who's adjacent to you can Interact and be instantly teleported to an unoccupied space in the spell's area, as long as they don't travel more than 60 feet. This effect has the teleportation trait.Heightened (+2) The damage increases by 2d4, the duration of the dazzled condition on a failed save increases by 1 round, and the maximum distance an ally can use the rainbow to teleport increases by 10 feet.`},
	`rainbows_end_neural`: {Name: `Rainbow's End`, Description: `You reach upward to wrest down a rainbow and harness its power to connect this world to the heavens. Each creature in the area takes 1d4 spirit damage with a basic Fortitude save. Any creature that fails this save is additionally @UUID[Compendium.pf2e.conditionitems.Item.Dazzled] for 1 round. For the spell's duration, an ally who's adjacent to you can Interact and be instantly teleported to an unoccupied space in the spell's area, as long as they don't travel more than 60 feet. This effect has the teleportation trait.Heightened (+2) The damage increases by 2d4, the duration of the dazzled condition on a failed save increases by 1 round, and the maximum distance an ally can use the rainbow to teleport increases by 10 feet.`},
	`rainbows_end_technical`: {Name: `Prismatic Beam Array`, Description: `Fire a multi-spectrum beam array that hits with dazzling prismatic energy.`},
	`ray_of_frost_bio_synthetic`: {Name: `Cryo-Beam Secretion`, Description: `You secrete a focused beam of cryo-compound, dealing cold damage and potentially slowing the target's movement.`},
	`read_aura_bio_synthetic`: {Name: `Bio-Signature Scanner`, Description: `You scan an object or creature with bio-chemical sensors, identifying general bio-type or organic origin.`},
	`read_aura_fanatic_doctrine`: {Name: `Doctrine's Aura Reading`, Description: `You scan an object or creature with the Doctrine's discerning eye, identifying its ideological alignment and power level.`},
	`read_aura_neural`: {Name: `Signature Reading`, Description: `You focus your psionic senses on an object, reading its residual energy signature and identifying its general tech or origin type.`},
	`read_the_air_fanatic_doctrine`: {Name: `Social Doctrine Analysis`, Description: `You read a social situation through the Doctrine's wisdom, identifying power dynamics and ideological loyalties.`},
	`read_the_air_neural`: {Name: `Social Dynamics Scan`, Description: `You perform a rapid behavioral analysis of a social encounter, gaining insight into the key players and their intentions.`},
	`reed_whistle_bio_synthetic`: {Name: `Reed Whistle`, Description: `You enchant a blade of grass that you can easily hold in your mouth without inhibiting your speech or other actions. As a reaction, you can reduce the spell's remaining duration by 1 hour to Point Out a creature you detect as you sharply whistle through the reed. You and your allies also gain a +2 circumstance bonus to Perception checks to @UUID[Compendium.pf2e.actionspf2e.Item.Seek] the creature for [[/r 1d4 #rounds]]{1d4 rounds}.Heightened (3rd) The spell's duration becomes 4 hours.`},
	`reed_whistle_neural`: {Name: `Reed Whistle`, Description: `You enchant a blade of grass that you can easily hold in your mouth without inhibiting your speech or other actions. As a reaction, you can reduce the spell's remaining duration by 1 hour to Point Out a creature you detect as you sharply whistle through the reed. You and your allies also gain a +2 circumstance bonus to Perception checks to @UUID[Compendium.pf2e.actionspf2e.Item.Seek] the creature for [[/r 1d4 #rounds]]{1d4 rounds}.Heightened (3rd) The spell's duration becomes 4 hours.`},
	`reed_whistle_technical`: {Name: `Signal Tone Generator`, Description: `Produce a precise signal tone used for communication or system activation.`},
	`restyle_bio_synthetic`: {Name: `Restyle`, Description: `You permanently change the appearance of one piece of clothing currently worn by you or an ally to better fit your aesthetic sensibilities. You can change its color, texture, pattern, and other minor parts of its design, but the changes can't alter the clothing's overall shape, size, or purpose. The changes can't increase the quality of the craftsmanship or artistry of the piece of clothing, but particularly gauche choices for the new color and pattern might decrease its aesthetic appeal. This spell transforms existing materials into the desired appearance and never alters the material or creates more material than what's originally part of the object. The object's statistics also remain unchanged.`},
	`restyle_fanatic_doctrine`: {Name: `Restyle`, Description: `You permanently change the appearance of one piece of clothing currently worn by you or an ally to better fit your aesthetic sensibilities. You can change its color, texture, pattern, and other minor parts of its design, but the changes can't alter the clothing's overall shape, size, or purpose. The changes can't increase the quality of the craftsmanship or artistry of the piece of clothing, but particularly gauche choices for the new color and pattern might decrease its aesthetic appeal. This spell transforms existing materials into the desired appearance and never alters the material or creates more material than what's originally part of the object. The object's statistics also remain unchanged.`},
	`restyle_neural`: {Name: `Restyle`, Description: `You permanently change the appearance of one piece of clothing currently worn by you or an ally to better fit your aesthetic sensibilities. You can change its color, texture, pattern, and other minor parts of its design, but the changes can't alter the clothing's overall shape, size, or purpose. The changes can't increase the quality of the craftsmanship or artistry of the piece of clothing, but particularly gauche choices for the new color and pattern might decrease its aesthetic appeal. This spell transforms existing materials into the desired appearance and never alters the material or creates more material than what's originally part of the object. The object's statistics also remain unchanged.`},
	`restyle_technical`: {Name: `Garment Nano-Actuator`, Description: `Use garment nano-actuators to instantly alter the cut and appearance of worn clothing.`},
	`root_reading_bio_synthetic`: {Name: `Organic Memory Scan`, Description: `You extract chemical memory from plant or soil material, reading recent environmental history encoded in organic matter.`},
	`rousing_splash_bio_synthetic`: {Name: `Stimulant Splash Compound`, Description: `You splash the target with a bio-synthesized stimulant compound, reviving unconscious or stunned allies.`},
	`rousing_splash_fanatic_doctrine`: {Name: `Revival Blessing`, Description: `You splash the target with the Doctrine's sanctified water, reviving unconscious or stunned allies.`},
	`runic_body_bio_synthetic`: {Name: `Runic Body`, Description: `Glowing runes appear on the target's body. All its unarmed attacks become +1 striking unarmed attacks, gaining a +1 item bonus to attack rolls and increasing the number of damage dice to two.

Heightened (6th) The unarmed attacks are +2 greater striking.
Heightened (9th) The unarmed attacks are +3 major striking.
@UUID[Compendium.pf2e.spell-effects.Item.Spell Effect: Runic Body]`},
	`runic_body_fanatic_doctrine`: {Name: `Runic Body`, Description: `Glowing runes appear on the target's body. All its unarmed attacks become +1 striking unarmed attacks, gaining a +1 item bonus to attack rolls and increasing the number of damage dice to two.

Heightened (6th) The unarmed attacks are +2 greater striking.
Heightened (9th) The unarmed attacks are +3 major striking.
@UUID[Compendium.pf2e.spell-effects.Item.Spell Effect: Runic Body]`},
	`runic_body_neural`: {Name: `Runic Body`, Description: `Glowing runes appear on the target's body. All its unarmed attacks become +1 striking unarmed attacks, gaining a +1 item bonus to attack rolls and increasing the number of damage dice to two.

Heightened (6th) The unarmed attacks are +2 greater striking.
Heightened (9th) The unarmed attacks are +3 major striking.
@UUID[Compendium.pf2e.spell-effects.Item.Spell Effect: Runic Body]`},
	`runic_body_technical`: {Name: `Combat Augmentation Layer`, Description: `Activate a set of combat-optimized augmentation overlays on the body.`},
	`runic_weapon_bio_synthetic`: {Name: `Runic Weapon`, Description: `The weapon glimmers with magic as temporary runes carve down its length. The target becomes a +1 striking weapon, gaining a +1 item bonus to attack rolls and increasing the number of weapon damage dice to two.

Heightened (6th) The weapon is +2 greater striking.
Heightened (9th) The weapon is +3 major striking.
@UUID[Compendium.pf2e.spell-effects.Item.Spell Effect: Runic Weapon]`},
	`runic_weapon_fanatic_doctrine`: {Name: `Runic Weapon`, Description: `The weapon glimmers with magic as temporary runes carve down its length. The target becomes a +1 striking weapon, gaining a +1 item bonus to attack rolls and increasing the number of weapon damage dice to two.

Heightened (6th) The weapon is +2 greater striking.
Heightened (9th) The weapon is +3 major striking.
@UUID[Compendium.pf2e.spell-effects.Item.Spell Effect: Runic Weapon]`},
	`runic_weapon_neural`: {Name: `Runic Weapon`, Description: `The weapon glimmers with magic as temporary runes carve down its length. The target becomes a +1 striking weapon, gaining a +1 item bonus to attack rolls and increasing the number of weapon damage dice to two.

Heightened (6th) The weapon is +2 greater striking.
Heightened (9th) The weapon is +3 major striking.
@UUID[Compendium.pf2e.spell-effects.Item.Spell Effect: Runic Weapon]`},
	`runic_weapon_technical`: {Name: `Weapon Enhancement Module`, Description: `Attach a weapon enhancement module that temporarily upgrades damage output.`},
	`sacred_beasts_bio_synthetic`: {Name: `Sacred Beasts`, Description: `Requirements You worship a deity.

You call out to the creatures of the wild favored by your deity. You quickly summon your deity's sacred animal (or a small swarm of them if the animal is usually Tiny). For example, you would call forth a lion if you worship Iomedae or a swarm of spiders if you worship Norgorber. If your deity doesn't have a known sacred animal, work with the GM to find a thematic one. The animal or swarm assaults all creatures in the area, dealing 2d6 damage. The damage is either bludgeoning, piercing, or slashing based on the animal that was conjured, as determined by the GM. After their attacks, the animals return to your deity's plane.

Heightened (+1) The damage increases by 2d6.`},
	`sacred_beasts_fanatic_doctrine`: {Name: `Sacred Beasts`, Description: `Requirements You worship a deity.

You call out to the creatures of the wild favored by your deity. You quickly summon your deity's sacred animal (or a small swarm of them if the animal is usually Tiny). For example, you would call forth a lion if you worship Iomedae or a swarm of spiders if you worship Norgorber. If your deity doesn't have a known sacred animal, work with the GM to find a thematic one. The animal or swarm assaults all creatures in the area, dealing 2d6 damage. The damage is either bludgeoning, piercing, or slashing based on the animal that was conjured, as determined by the GM. After their attacks, the animals return to your deity's plane.

Heightened (+1) The damage increases by 2d6.`},
	`sanctuary_fanatic_doctrine`: {Name: `Sanctuary`, Description: `You ward a creature with protective energy that deters attacks. Creatures attempting to attack the target must attempt a Will save each time. If the target uses a hostile action, the spell ends.

Critical Success Sanctuary ends.
Success The creature can attempt its attack and any other attacks against the target this turn.
Failure The creature can't attack the target and wastes the action. It can't attempt further attacks against the target this turn.
Critical Failure The creature wastes the action and can't attempt to attack the target for the rest of sanctuary's duration.`},
	`sanctuary_neural`: {Name: `Sanctuary`, Description: `You ward a creature with protective energy that deters attacks. Creatures attempting to attack the target must attempt a Will save each time. If the target uses a hostile action, the spell ends.

Critical Success Sanctuary ends.
Success The creature can attempt its attack and any other attacks against the target this turn.
Failure The creature can't attack the target and wastes the action. It can't attempt further attacks against the target this turn.
Critical Failure The creature wastes the action and can't attempt to attack the target for the rest of sanctuary's duration.`},
	`scatter_scree_bio_synthetic`: {Name: `Debris Launch`, Description: `You collect and launch fragments of stone or bone at high velocity, dealing piercing damage to targets in the area.`},
	`schadenfreude_fanatic_doctrine`: {Name: `Schadenfreude`, Description: `Trigger You critically fail a saving throw against a foe's effect.You distract your enemy with their feeling of smug pleasure when you fail catastrophically. They must attempt a Will save.Critical Success The creature is unaffected.
Success The creature is distracted by its amusement and takes a -1 status penalty on Perception checks and Will saves for 1 round. @UUID[Compendium.pf2e.spell-effects.Item.Spell Effect: Schadenfreude (Success)]
Failure The creature is overcome by its amusement and is Stupefied 1 for 1 round.
Critical Failure The creature is lost in its amusement and is Stupefied 2 for 1 round and Stunned 1.`},
	`schadenfreude_neural`: {Name: `Schadenfreude`, Description: `Trigger You critically fail a saving throw against a foe's effect.You distract your enemy with their feeling of smug pleasure when you fail catastrophically. They must attempt a Will save.Critical Success The creature is unaffected.
Success The creature is distracted by its amusement and takes a -1 status penalty on Perception checks and Will saves for 1 round. @UUID[Compendium.pf2e.spell-effects.Item.Spell Effect: Schadenfreude (Success)]
Failure The creature is overcome by its amusement and is Stupefied 1 for 1 round.
Critical Failure The creature is lost in its amusement and is Stupefied 2 for 1 round and Stunned 1.`},
	`schadenfreude_technical`: {Name: `Adversity Feedback Loop`, Description: `Trigger a neural feedback loop that converts suffered damage into a resolve bonus.`},
	`scorching_blast_bio_synthetic`: {Name: `Scorching Blast`, Description: `You evoke a mass of fire into the air around your outstretched fist. For the remainder of your turn, you can blast targets within 30 feet with this fire by spending a single action which has the attack and concentrate traits. When you do so, attempt a ranged spell attack roll. If you hit, you inflict 2d8 fire damage. On a critical hit, the target takes @Damage[(1d6+((@item.rank -1)*2))[persistent,fire]] damage.Heightened (+1) The base damage increases by 1d8 and the persistent fire damage on a critical hit increases by 2.`},
	`scorching_blast_neural`: {Name: `Scorching Blast`, Description: `You evoke a mass of fire into the air around your outstretched fist. For the remainder of your turn, you can blast targets within 30 feet with this fire by spending a single action which has the attack and concentrate traits. When you do so, attempt a ranged spell attack roll. If you hit, you inflict 2d8 fire damage. On a critical hit, the target takes @Damage[(1d6+((@item.rank -1)*2))[persistent,fire]] damage.Heightened (+1) The base damage increases by 1d8 and the persistent fire damage on a critical hit increases by 2.`},
	`scorching_blast_technical`: {Name: `Thermal Plasma Fist`, Description: `Discharge a high-energy plasma accumulator through a strike for explosive thermal damage.`},
	`scouring_sand_bio_synthetic`: {Name: `Scouring Sand`, Description: `You blast the area with grit that scours away soil and gets into creatures' eyes. For the duration of the spell, any plant-based difficult terrain smaller than a tree becomes loose, allowing each 5-foot square of it to be cleared with a single Interact action. In addition, scouring sand attempts to counteract @UUID[Compendium.pf2e.spells-srd.Item.Entangling Flora] and other effects that create or manipulate plant-based terrain in its area. Successfully counteracting an effect removes only the portion of its area that overlaps with scouring sand's area. After one such attempt, the effect is temporarily immune to scouring sand's counteract for 24 hours. Each creature in the area when you Cast this Spell or that ends its turn in the area must attempt a Reflex save.

Success The creature is unaffected.
Failure The creature is @UUID[Compendium.pf2e.conditionitems.Item.Dazzled] for 1 minute or until it uses an Interact action to get the sand out of its eyes.
Critical Failure As failure, but the creature is also @UUID[Compendium.pf2e.conditionitems.Item.Blinded] for its next action.

Heightened (3rd) Once per round when you Sustain the Spell, you can move the center of the burst to a spot within range.
Heightened (6th) As the 3rd-rank version, except the range is 120 feet and the area is a 20-foot burst.`},
	`scouring_sand_technical`: {Name: `Abrasive Particle Cannon`, Description: `Fire a stream of high-velocity abrasive particles that strip armor and blind targets.`},
	`seashell_of_stolen_sound_bio_synthetic`: {Name: `Seashell of Stolen Sound`, Description: `Trigger A creature within range begins to make a sound.
You store a sound in a seashell to use as you will: the last words of a loved one, a dragon's mighty roar, the compromising conversation between two powerful diplomats, or even more strange and secret. As part of Casting this Spell, you must present an unbroken seashell. When you Cast the Spell, magic swirls around the triggering creature, copying the sounds they make, as well as any background noise, for the next minute and storing them in the seashell.
You or another creature can then play the sound back from the seashell during the spell's duration by Interacting with the seashell, but once the sounds have been played back, the seashell shatters and the spell ends.
As normal for spells with a duration until your next daily preparations, you can choose to continue expending the spell slot to prolong the duration of an existing seashell of stolen sound for another day. While the spell faithfully copies the sounds around the target, it doesn't reproduce any special auditory or sonic effects of the sound.`},
	`seashell_of_stolen_sound_neural`: {Name: `Seashell of Stolen Sound`, Description: `Trigger A creature within range begins to make a sound.
You store a sound in a seashell to use as you will: the last words of a loved one, a dragon's mighty roar, the compromising conversation between two powerful diplomats, or even more strange and secret. As part of Casting this Spell, you must present an unbroken seashell. When you Cast the Spell, magic swirls around the triggering creature, copying the sounds they make, as well as any background noise, for the next minute and storing them in the seashell.
You or another creature can then play the sound back from the seashell during the spell's duration by Interacting with the seashell, but once the sounds have been played back, the seashell shatters and the spell ends.
As normal for spells with a duration until your next daily preparations, you can choose to continue expending the spell slot to prolong the duration of an existing seashell of stolen sound for another day. While the spell faithfully copies the sounds around the target, it doesn't reproduce any special auditory or sonic effects of the sound.`},
	`seashell_of_stolen_sound_technical`: {Name: `Sound Capture Device`, Description: `Deploy a directional microphone array that captures and stores specific sounds for replay.`},
	`seismic_sense`: {Name: `Seismic Sense`, Description: `Bone-conduction implants detect ground vibrations, revealing the movement of creatures through floors and walls.`},
	`share_lore_neural`: {Name: `Share Lore`, Description: `You share your knowledge with the touched creatures. Choose one Lore skill in which you're trained. The targets become trained in that Lore skill for the duration of the spell.Heightened (3rd) The duration of the spell is 1 hour, and you can target up to five creatures.
Heightened (5th) The duration of the spell is 8 hours, you can target up to five creatures, and you can share up to two Lore skills in which you're trained.`},
	`share_lore_technical`: {Name: `Data Package Transfer`, Description: `Beam a compressed data package of knowledge directly to a target's neural interface.`},
	`shattering_gem_bio_synthetic`: {Name: `Shattering Gem`, Description: `A large gem floats around the target in an erratic pattern. The gem has 5 Hit Points. Each time a creature Strikes the target, the target attempts a @Check[flat|dc:11|showDC:all]. On a success, the gem blocks the attack, so the attack first damages the gem and then applies any remaining damage to the target. If the gem is reduced to 0 Hit Points, it shatters, immediately dealing 1d8 slashing damage (basic Reflex save) to the creature that destroyed it, as long as that creature is within 10 feet of the target.Heightened (+1) The gem has 5 additional Hit Points, and the damage dealt by its detonation increases by 1d8.`},
	`shattering_gem_technical`: {Name: `Resonance Shatter Charge`, Description: `Deploy a resonance emitter tuned to the crystalline frequency of a target material.`},
	`shield_fanatic_doctrine`: {Name: `Doctrine's Shield of Faith`, Description: `You manifest a doctrinal energy shield that can intercept incoming damage. Triggers as a reaction to block blows.`},
	`shield_neural`: {Name: `Neural Barrier`, Description: `You project a reinforced kinetic barrier that intercepts incoming damage. Can be used as a reaction to block blows.`},
	`shielded_arm_bio_synthetic`: {Name: `Shielded Arm`, Description: `Reinforcing veins of ore run through the target's arm, letting it ward off blows with its bare skin. It can use the Raise a Shield action to instead raise its arm, gaining a +2 circumstance bonus to AC. It can Shield Block with its Raised arm as well; when it does, the target reduces the damage as if it had a shield with Hardness 4 and 15 Hit Points. This shield doesn't have a Broken Threshold, and the spell ends if the shield's Hit Points are expended.
This spell doesn't modify the target's unarmed attacks and can't be used to make a shield bash Strike. Casting or coming under the effects of this spell also counts as using a metallic item with regards to anathema.

Heightened (+2) The Hardness increases by 4, and the Hit Points increase by 15.`},
	`shielded_arm_fanatic_doctrine`: {Name: `Shielded Arm`, Description: `Reinforcing veins of ore run through the target's arm, letting it ward off blows with its bare skin. It can use the Raise a Shield action to instead raise its arm, gaining a +2 circumstance bonus to AC. It can Shield Block with its Raised arm as well; when it does, the target reduces the damage as if it had a shield with Hardness 4 and 15 Hit Points. This shield doesn't have a Broken Threshold, and the spell ends if the shield's Hit Points are expended.
This spell doesn't modify the target's unarmed attacks and can't be used to make a shield bash Strike. Casting or coming under the effects of this spell also counts as using a metallic item with regards to anathema.

Heightened (+2) The Hardness increases by 4, and the Hit Points increase by 15.`},
	`shielded_arm_technical`: {Name: `Forearm Shield Extrusion`, Description: `Extrude a compact hardened shield from a forearm-mounted deployment system.`},
	`shillelagh`: {Name: `Shillelagh`, Description: `The target grows vines and leaves, brimming with primal energy. The target becomes a +1 striking weapon while in your hands, gaining a +1 item bonus to attack rolls and increasing the number of weapon damage dice to two. Additionally, as long as you are on your home plane, attacks you make with the target against aberrations, extraplanar creatures, and undead increase the number of weapon damage dice to three.
@UUID[Compendium.pf2e.spell-effects.Item.Spell Effect: Shillelagh]`},
	`shocking_grasp_bio_synthetic`: {Name: `Shocking Grasp`, Description: `You shroud your hands in a crackling field of lightning. Make a melee spell attack roll. On a hit, the target takes 2d12 electricity damage. If the target is wearing metal armor or is made of metal, you gain a +1 circumstance bonus to your attack roll with shocking grasp, and the target also takes @Damage[(1d4+((@item.level)-1))[persistent,electricity]] damage on a hit. On a critical hit, double the initial damage, but not the persistent damage.Heightened (+1) The damage increases by 1d12, and the persistent electricity damage increases by 1.`},
	`shocking_grasp_technical`: {Name: `High-Voltage Contact Discharge`, Description: `Deliver a high-voltage electrical discharge on contact with a grounded target.`},
	`shockwave_bio_synthetic`: {Name: `Shockwave`, Description: `You create a wave of energy that ripples through the earth. Terrestrial creatures in the affected area must attempt a Reflex save to avoid stumbling as the shockwave shakes the ground.Critical Success The creature is unaffected.
Success The creature is @UUID[Compendium.pf2e.conditionitems.Item.Off-Guard] until the start of its next turn.
Failure The creature falls @UUID[Compendium.pf2e.conditionitems.Item.Prone].
Critical Failure As failure, plus the creature takes 1d6 bludgeoning damage.Heightened (+1) The area increases by 5 feet (to a 20-foot cone at 2nd rank, and so on).`},
	`shockwave_technical`: {Name: `Seismic Pulse Generator`, Description: `Trigger a ground-mounted seismic pulse that knocks down targets in the surrounding area.`},
	`sigil_bio_synthetic`: {Name: `Bio-Chemical Identifier Tag`, Description: `You mark an object or location with an invisible chemical identifier, readable only by those who know the scent signature.`},
	`sigil_fanatic_doctrine`: {Name: `Doctrine's Mark`, Description: `You invisibly mark an object or location with the Doctrine's unique identifier — detectable only to the faithful who know the sign.`},
	`sigil_neural`: {Name: `Personal Signature Tag`, Description: `You invisibly tag an object or location with your unique psionic identifier — detectable only to those who know what to scan for.`},
	`signal_skyrocket_bio_synthetic`: {Name: `Signal Skyrocket`, Description: `With a pinch of metallic powder and gunpowder, you call forth blistering red energy that shoots straight upward into the air and explodes, unleashing a crackling boom. Over time, you might even customize your own pattern and color for the skyrocket as you refine the spell.
You can't change the direction or distance of the rocket-it must go straight up, continuing up to the maximum range if possible. If the rocket explodes at its maximum height, the bright light can be seen up to 10 miles away, and the sound of the explosion can be heard up to 1 mile away under clear weather conditions.
If the rocket explodes in an enclosed space smaller than the full size of the burst, each creature in the area takes 1d10 sonic damage depending on the result of its Reflex save.

Critical Success The creature is unaffected.
Success The creature takes half damage.
Failure The creature takes full damage and is @UUID[Compendium.pf2e.conditionitems.Item.Dazzled] for 1 round.
Critical Failure The creature takes double damage and is @UUID[Compendium.pf2e.conditionitems.Item.Blinded] for 1 round.

Heightened (+1) The sonic damage increases by 1d10.`},
	`signal_skyrocket_neural`: {Name: `Signal Skyrocket`, Description: `With a pinch of metallic powder and gunpowder, you call forth blistering red energy that shoots straight upward into the air and explodes, unleashing a crackling boom. Over time, you might even customize your own pattern and color for the skyrocket as you refine the spell.
You can't change the direction or distance of the rocket-it must go straight up, continuing up to the maximum range if possible. If the rocket explodes at its maximum height, the bright light can be seen up to 10 miles away, and the sound of the explosion can be heard up to 1 mile away under clear weather conditions.
If the rocket explodes in an enclosed space smaller than the full size of the burst, each creature in the area takes 1d10 sonic damage depending on the result of its Reflex save.

Critical Success The creature is unaffected.
Success The creature takes half damage.
Failure The creature takes full damage and is @UUID[Compendium.pf2e.conditionitems.Item.Dazzled] for 1 round.
Critical Failure The creature takes double damage and is @UUID[Compendium.pf2e.conditionitems.Item.Blinded] for 1 round.

Heightened (+1) The sonic damage increases by 1d10.`},
	`signal_skyrocket_technical`: {Name: `Flare Launcher`, Description: `Fire a bright signal flare from a compact launcher for signaling or distraction.`},
	`slashing_gust_bio_synthetic`: {Name: `Blade Wind Exhale`, Description: `You exhale a cutting burst of air reinforced with bio-synthetic particles, dealing slashing damage in a cone.`},
	`sleep_neural`: {Name: `Sleep`, Description: `Each creature in the area becomes drowsy, possibly nodding off. A creature that falls @UUID[Compendium.pf2e.conditionitems.Item.Unconscious] from this spell doesn't fall @UUID[Compendium.pf2e.conditionitems.Item.Prone] or release what it's holding. This spell doesn't prevent creatures from waking up due to a successful Perception check, limiting its utility in combat.Critical Success The creature is unaffected.
Success The creature takes a –1 status penalty to Perception checks for 1 round.
Failure The creature falls unconscious. If it's still unconscious after 1 minute, it wakes up automatically.
Critical Failure The creature falls unconscious. If it's still unconscious after 1 hour, it wakes up automatically.Heightened (4th) The creatures fall unconscious for 1 round on a failure or 1 minute on a critical failure. They fall prone and release what they're holding, and they can't attempt Perception checks to wake up. When the duration ends, the creature is sleeping normally instead of automatically waking up.`},
	`sleep_technical`: {Name: `Soporific Aerosol Grenade`, Description: `Detonate a grenade releasing a fast-acting soporific aerosol that induces unconsciousness.`},
	`snowball_bio_synthetic`: {Name: `Snowball`, Description: `You throw a magically propelled and chilled ball of dense snow. The target takes 2d4 cold damage and potentially other effects, depending on the result of your spell attack roll.

Critical Success The target takes double damage and a –10-foot status penalty to its Speeds for 1 round.
Success The target takes full damage and a –5-foot status penalty to its Speeds for 1 round.
Failure No effect.

Heightened (+1) The damage increases by 2d4.`},
	`snowball_technical`: {Name: `Cryo-Gel Projectile`, Description: `Fire a capsule of cryogenic gel that encases the target in fast-hardening ice.`},
	`soothe`: {Name: `Soothe`, Description: `You grace the target's mind, boosting its mental defenses and healing its wounds. The target regains 1d10+4 Hit Points when you Cast the Spell and gains a +2 status bonus to saves against mental effects for the duration.
@UUID[Compendium.pf2e.spell-effects.Item.Spell Effect: Soothe]

Heightened (+1) The amount of healing increases by 1d10+4.`},
	`spider_sting_bio_synthetic`: {Name: `Spider Sting`, Description: `You magically duplicate a spider's venomous sting. You deal 1d4 piercing damage to the touched creature and afflict it with spider venom. The target must attempt a Fortitude save.

Critical Success The target is unaffected.
Success The target takes @Damage[1d4[poison]] damage.
Failure The target is afflicted with spider venom at stage 1.
Critical Failure The target is afflicted with spider venom at stage 2.

Spider Venom (poison)
Level 1
Maximum Duration 4 rounds
Stage 1 1d4 poison damage and Enfeebled 1 (1 round)
Stage 2 1d4 poison damage and Enfeebled 2 (1 round)`},
	`spider_sting_technical`: {Name: `Paralytic Micro-Dart`, Description: `Fire a micro-dart loaded with a fast-acting paralytic compound.`},
	`spirit_link_fanatic_doctrine`: {Name: `Spirit Link`, Description: `You form a spiritual link with another creature, taking in its pain. When you Cast this Spell and at the start of each of your turns, if the target is below maximum Hit Points, it regains 2 Hit Points (or the difference between its current and maximum Hit Points, if that's lower). You lose as many Hit Points as the target regained.
This is a spiritual transfer, so no effects apply that would increase the Hit Points the target regains or decrease the Hit Points you lose. This transfer also ignores any temporary Hit Points you or the target have. Since this effect doesn't involve vitality or void energy, spirit link works even if you or the target is undead. While the duration persists, you gain no benefit from regeneration or fast healing. You can Dismiss this spell, and if you're ever at 0 Hit Points, spirit link ends automatically.

Heightened (+1) The number of Hit Points transferred each time increases by 2.`},
	`spirit_link_neural`: {Name: `Spirit Link`, Description: `You form a spiritual link with another creature, taking in its pain. When you Cast this Spell and at the start of each of your turns, if the target is below maximum Hit Points, it regains 2 Hit Points (or the difference between its current and maximum Hit Points, if that's lower). You lose as many Hit Points as the target regained.
This is a spiritual transfer, so no effects apply that would increase the Hit Points the target regains or decrease the Hit Points you lose. This transfer also ignores any temporary Hit Points you or the target have. Since this effect doesn't involve vitality or void energy, spirit link works even if you or the target is undead. While the duration persists, you gain no benefit from regeneration or fast healing. You can Dismiss this spell, and if you're ever at 0 Hit Points, spirit link ends automatically.

Heightened (+1) The number of Hit Points transferred each time increases by 2.`},
	`spirit_ward_fanatic_doctrine`: {Name: `Spirit Ward`, Description: `You draw on nearby spiritual energy or on echoes of the spirits you've invoked throughout your life to temporarily ward living flesh against dangerous spirits. You grant the target a +1 status bonus to saving throws against spells and effects caused by creatures that have the spirit trait and haunts. The number of actions you spend when Casting this Spell determines its targets, range, area, and other parameters.
1 The spell has a range of touch.
2 (concentrate) The spell has a range of 30 feet. If you target a living creature, the bonus increases to +2.
3 (concentrate) You create a ward in a @Template[type:emanation|distance:30]. This targets you and all your allies in the burst.
@UUID[Compendium.pf2e.spell-effects.Item.Spell Effect: Spirit Ward]`},
	`spirit_ward_neural`: {Name: `Spirit Ward`, Description: `You draw on nearby spiritual energy or on echoes of the spirits you've invoked throughout your life to temporarily ward living flesh against dangerous spirits. You grant the target a +1 status bonus to saving throws against spells and effects caused by creatures that have the spirit trait and haunts. The number of actions you spend when Casting this Spell determines its targets, range, area, and other parameters.
1 The spell has a range of touch.
2 (concentrate) The spell has a range of 30 feet. If you target a living creature, the bonus increases to +2.
3 (concentrate) You create a ward in a @Template[type:emanation|distance:30]. This targets you and all your allies in the burst.
@UUID[Compendium.pf2e.spell-effects.Item.Spell Effect: Spirit Ward]`},
	`spout_bio_synthetic`: {Name: `High-Pressure Bio-Fluid Jet`, Description: `You project a high-pressure biological fluid jet that deals bludgeoning damage and may knock targets prone.`},
	`stabilize_bio_synthetic`: {Name: `Stabilize`, Description: `Life energy shuts death's door. The target loses the @UUID[Compendium.pf2e.conditionitems.Item.Dying] condition, though it remains @UUID[Compendium.pf2e.conditionitems.Item.Unconscious] at 0 Hit Points.`},
	`stabilize_fanatic_doctrine`: {Name: `Emergency Doctrine Stabilization`, Description: `You invoke the Doctrine's healing mercy, stabilizing a dying creature and preventing their immediate death.`},
	`summon_animal_bio_synthetic`: {Name: `Summon Animal`, Description: `You summon a creature that has the animal trait and whose level is –1 to fight for you.Heightened As listed in the summon trait.`},
	`summon_animal_technical`: {Name: `Trained Animal Deployment`, Description: `Release a trained animal from a compact transport unit for scouting or combat support.`},
	`summon_construct`: {Name: `Combat Drone Deploy`, Description: `Release a pre-loaded combat drone from a transport capsule for autonomous combat support.`},
	`summon_fey_bio_synthetic`: {Name: `Summon Fey`, Description: `You summon a creature that has the fey trait and whose level is –1 to fight for you.

Heightened As listed in the summon trait.`},
	`summon_fey_neural`: {Name: `Summon Fey`, Description: `You summon a creature that has the fey trait and whose level is –1 to fight for you.

Heightened As listed in the summon trait.`},
	`summon_instrument_fanatic_doctrine`: {Name: `Doctrine's Instrument Summon`, Description: `You invoke the Doctrine's provision, summoning a musical instrument or ritual tool to your hands.`},
	`summon_instrument_neural`: {Name: `Equipment Teleport`, Description: `You telekinetically summon a stored instrument or tool from your inventory cache to your hands.`},
	`summon_lesser_servitor`: {Name: `Summon Lesser Servitor`, Description: `While deities jealously guard their most powerful servants from the summoning spells of those who aren't steeped in the faith, this spell allows you to conjure an inhabitant of the Outer Sphere with or without the deity's permission. You summon a common celestial, fiend, or monitor of level –1. You can choose to instead summon a common animal of level –1 that hails from the Outer Sphere; you can choose for this animal to gain the celestial and holy traits, the fiend and unholy traits, or the monitor trait. It's anathema to summon a servitor if it has a holy or unholy trait that isn't allowed for your deity's sanctification. For example, Sarenrae's sanctification is "can choose holy," so you couldn't summon an unholy creature, and Pharasma's is "none," so you couldn't summon a holy or unholy creature. The GM might determine that your deity restricts specific types of creatures further, making it anathema to summon them as well.Heightened (2nd) The creature can be level 1 or lower.
Heightened (3rd) The creature can be level 2 or lower.
Heightened (4th) The creature can be level 3 or lower.`},
	`summon_plant_or_fungus`: {Name: `Summon Plant or Fungus`, Description: `You summon a creature that has the plant or fungus trait and whose level is -1 to fight for you.

Heightened As listed in the summon trait.`},
	`summon_undead_fanatic_doctrine`: {Name: `Summon Undead`, Description: `You summon a creature that has the undead trait and whose level is -1 to fight for you.

Heightened As listed in the summon trait.`},
	`summon_undead_neural`: {Name: `Summon Undead`, Description: `You summon a creature that has the undead trait and whose level is -1 to fight for you.

Heightened As listed in the summon trait.`},
	`summon_undead_technical`: {Name: `Reanimation Protocol`, Description: `Trigger a neural reactivation sequence in a recently deceased target.`},
	`sure_strike_neural`: {Name: `Sure Strike`, Description: `The next time you make an attack roll before the end of your turn, roll it twice and use the better result. The attack ignores circumstance penalties to the attack roll and any flat check required due to the target being @UUID[Compendium.pf2e.conditionitems.Item.Concealed] or @UUID[Compendium.pf2e.conditionitems.Item.Hidden]. You are then temporarily immune to sure strike for 10 minutes.
@UUID[Compendium.pf2e.spell-effects.Item.Spell Effect: Sure Strike]`},
	`sure_strike_technical`: {Name: `Targeting Precision Lock`, Description: `Engage a targeting precision subroutine that guarantees the next strike lands true.`},
	`swampcall`: {Name: `Swampcall`, Description: `You call upon the spirits of the soil to twist and churn, transforming the terrain in the targeted area into a sodden mess. The area becomes difficult terrain. Creatures in the area when you Cast this Spell must attempt a Reflex saving throw.

Success The creature is unaffected.
Failure The creature sinks partially into the mud. The creature takes a –10-foot circumstance penalty to its Speeds (except for its swim Speed, if any) and becomes @UUID[Compendium.pf2e.conditionitems.Item.Off-Guard]. These effects last until the creature leaves the area or until the end of its next turn, whichever comes first.
Critical Failure As failure, but the penalty to Speeds (except Swim speed) is -15 feet.
@UUID[Compendium.pf2e.spell-effects.Item.Spell Effect: Swampcall]

Heightened (3rd) The range increases to 60 feet, and the area increases to a 20-foot burst.`},
	`synaptic_surge`: {Name: `Synaptic Surge`, Description: `Overwhelms a target's nervous system with a burst of pain signals.`},
	`synchronize_bio_synthetic`: {Name: `Synchronize`, Description: `You harmlessly place your unique magic sigil, which is about 1 square inch in size, on your targets. When you Cast the Spell, you set the duration by choosing a time at which point the sigil flashes dimly three times. After that point, the spell ends. Even though spell durations aren't normally exact, the effects of synchronize are precise to the second. The timer is based on the place where the spell was cast, so entering a plane or area where time flows differently changes how the time elapses.

Heightened (2nd) The spell can target up to 20 willing creatures.`},
	`synchronize_fanatic_doctrine`: {Name: `Synchronize`, Description: `You harmlessly place your unique magic sigil, which is about 1 square inch in size, on your targets. When you Cast the Spell, you set the duration by choosing a time at which point the sigil flashes dimly three times. After that point, the spell ends. Even though spell durations aren't normally exact, the effects of synchronize are precise to the second. The timer is based on the place where the spell was cast, so entering a plane or area where time flows differently changes how the time elapses.

Heightened (2nd) The spell can target up to 20 willing creatures.`},
	`synchronize_neural`: {Name: `Synchronize`, Description: `You harmlessly place your unique magic sigil, which is about 1 square inch in size, on your targets. When you Cast the Spell, you set the duration by choosing a time at which point the sigil flashes dimly three times. After that point, the spell ends. Even though spell durations aren't normally exact, the effects of synchronize are precise to the second. The timer is based on the place where the spell was cast, so entering a plane or area where time flows differently changes how the time elapses.

Heightened (2nd) The spell can target up to 20 willing creatures.`},
	`synchronize_steps_neural`: {Name: `Synchronize Steps`, Description: `You link the minds of two targets, enabling them to move in tandem. When one of the targets Steps, the other target can use a reaction to Step. When one of the targets Strides, the other target can use a reaction to Stride.

Heightened (5th) The range increases to 60 feet, and you can target up to 10 willing creatures.`},
	`synchronize_steps_technical`: {Name: `Movement Sync Protocol`, Description: `Establish a movement synchronization link between two units for coordinated maneuvers.`},
	`synchronize_technical`: {Name: `Beacon Frequency Sync`, Description: `Sync tactical beacons across multiple units to a shared communication frequency.`},
	`tailwind_bio_synthetic`: {Name: `Tailwind`, Description: `The wind at your back pushes you to find new horizons. You gain a +10-foot status bonus to your Speed.
@UUID[Compendium.pf2e.spell-effects.Item.Spell Effect: Tailwind]

Heightened (2nd) The duration increases to 8 hours.
@UUID[Compendium.pf2e.spell-effects.Item.Spell Effect: Tailwind (8 hours)]`},
	`tailwind_technical`: {Name: `Slipstream Actuator`, Description: `Engage a rear-mounted micro-thruster that generates a tailwind boosting movement speed.`},
	`take_root_bio_synthetic`: {Name: `Root Anchor System`, Description: `You grow rapid anchor roots into the terrain, stabilizing yourself or an ally against knockback and forced movement.`},
	`tame_bio_synthetic`: {Name: `Biochemical Calming Agent`, Description: `You release a targeted biochemical calming compound that suppresses the aggression response in a wild creature.`},
	`tame_neural`: {Name: `Behavioral Override`, Description: `You broadcast a calming behavioral override signal to a wild creature, suppressing its aggression.`},
	`tangle_vine_bio_synthetic`: {Name: `Entanglement Vine Deploy`, Description: `You deploy a bundle of tangle-vines at the target, rapidly growing to entangle their limbs and restrict movement.`},
	`telekinetic_hand_neural`: {Name: `Telekinetic Manipulator`, Description: `You project a telekinetic field that acts as a precise invisible hand, manipulating objects at range.`},
	`telekinetic_projectile_neural`: {Name: `Telekinetic Launch`, Description: `You seize a loose object telekinetically and launch it as a high-velocity projectile at a target.`},
	`temporary_tool`: {Name: `Rapid Fabrication Print`, Description: `Use a compact field printer to produce a temporary-use tool or component on demand.`},
	`terror_broadcast`: {Name: `Terror Broadcast`, Description: `A subdermal transmitter floods nearby targets with a fear-inducing neural frequency.`},
	`tether_bio_synthetic`: {Name: `Tether`, Description: `You use magical chains, vines, or other tethers to bind your target to you. The creature can still try to @UUID[Compendium.pf2e.actionspf2e.Item.Escape], and it or others can break the tethers by attacking them (the tethers have AC 15 and 10 Hit Points). You must stay within 30 feet of the target while it's tethered; moving more than 30 feet away from your target ends the spell. The target must attempt a Reflex save. You can Dismiss the spell.Critical Success The target is unaffected.
Success The target takes a –5-foot circumstance penalty to its Speed as long as it's within 30 feet of you.
Failure The target takes a –10-foot circumstance penalty to its Speed and can't move more than 30 feet away from you until it Escapes or the spell ends.
Critical Failure The target is @UUID[Compendium.pf2e.conditionitems.Item.Immobilized] until it Escapes or the spell ends.Heightened (+1) The tethers' AC increases by 3, and their Hit Points increase by 10.`},
	`tether_technical`: {Name: `Electrostatic Tether Line`, Description: `Fire an electrostatic tether line that restrains a target at a fixed distance.`},
	`thicket_of_knives_neural`: {Name: `Thicket of Knives`, Description: `You create numerous phantom copies of your weapon arm, hiding your true movements and rendering your attacks unpredictable. You gain a +2 status bonus to Deception checks. If you're untrained in Deception, you can use the Feint action anyway, and add your level as your proficiency bonus despite being untrained.
@UUID[Compendium.pf2e.spell-effects.Item.Spell Effect: Thicket of Knives]`},
	`thicket_of_knives_technical`: {Name: `Blade Array Projector`, Description: `Deploy a rotating array of hard-light blades that punish movement through the area.`},
	`thoughtful_gift_fanatic_doctrine`: {Name: `Thoughtful Gift`, Description: `You teleport one object of light or negligible Bulk held in your hand to the target. The object appears instantly in the target's hand if they have a free hand, or at their feet if they don't. The target knows what object you're attempting to send them. If the target is @UUID[Compendium.pf2e.conditionitems.Item.Unconscious] or refuses to accept your gift, or if the spell would teleport a creature (even if the creature is inside an extradimensional container), the spell fails.Heightened (3rd) The spell's range increases to 500 feet.
Heightened (5th) As 3rd level, and the object's maximum Bulk increases to 1. You can Cast the Spell with 3 actions instead of 1; doing so increases the range to 1 mile, and you don't need line of sight to the target, but you must be extremely familiar with the target.`},
	`thoughtful_gift_neural`: {Name: `Thoughtful Gift`, Description: `You teleport one object of light or negligible Bulk held in your hand to the target. The object appears instantly in the target's hand if they have a free hand, or at their feet if they don't. The target knows what object you're attempting to send them. If the target is @UUID[Compendium.pf2e.conditionitems.Item.Unconscious] or refuses to accept your gift, or if the spell would teleport a creature (even if the creature is inside an extradimensional container), the spell fails.Heightened (3rd) The spell's range increases to 500 feet.
Heightened (5th) As 3rd level, and the object's maximum Bulk increases to 1. You can Cast the Spell with 3 actions instead of 1; doing so increases the range to 1 mile, and you don't need line of sight to the target, but you must be extremely familiar with the target.`},
	`thoughtful_gift_technical`: {Name: `Object Transfer Launcher`, Description: `Fire a compact delivery capsule to transfer an object to a target at range.`},
	`threefold_limb_bio_synthetic`: {Name: `Threefold Limb`, Description: `You temporarily transform one of your limbs into water, taking the form of ice, liquid water, or steam as you desire. Make a melee spell attack roll. On a hit, the target takes 2d6 damage; the type of damage dealt and any additional effect depends on the form you choose. On a critical hit, double the damage.Ice The limb deals cold damage, and the target takes a –10-foot status penalty to its Speeds until the start of your next turn. This spell gains the cold trait.
@UUID[Compendium.pf2e.spell-effects.Item.Spell Effect: Threefold Limb (Ice)]Liquid Water The limb deals bludgeoning damage and you can @UUID[Compendium.pf2e.actionspf2e.Item.Reposition] the target up to 10 feet.Steam The limb deals fire damage and steam clings to the target, making all creatures @UUID[Compendium.pf2e.conditionitems.Item.Concealed] to them until the start of your next turn or they perform an Interact action to wave the steam away. This spell gains the fire trait.Heightened (+1) The damage increases by 2d6.`},
	`threefold_limb_technical`: {Name: `Triple Strike Actuator`, Description: `Activate a rapid-cycle arm actuator for a three-hit mechanical strike sequence.`},
	`thunderstrike_bio_synthetic`: {Name: `Thunderstrike`, Description: `You call down a tendril of lightning that cracks with thunder, dealing 1d12 electricity damage and 1d4 sonic damage to the target with a basic Reflex save. A target wearing metal armor or made of metal takes a –1 circumstance bonus to its save, and if damaged by the spell is Clumsy 1 for 1 round.

Heightened (+1) The damage increases by 1d12 electricity and 1d4 sonic.`},
	`thunderstrike_technical`: {Name: `EMP Thunder Discharge`, Description: `Deliver a combined electromagnetic pulse and sonic discharge, disrupting electronics and balance.`},
	`timber_bio_synthetic`: {Name: `Timber`, Description: `You create a small dead tree in your space that falls over on anyone in its path, then immediately decomposes. Any creature in the area takes 2d4 bludgeoning damage, with a basic Reflex saving throw. A creature that critically fails its save is knocked for a loop, making it @UUID[Compendium.pf2e.conditionitems.Item.Dazzled] until the end of its next turn.

Heightened (+1) The initial damage increases by 1d4.`},
	`time_sense_neural`: {Name: `Internal Chronometer`, Description: `You achieve perfect time synchronization, knowing the exact time and duration of any interval with precision.`},
	`torturous_trauma_fanatic_doctrine`: {Name: `Doctrine's Punishment`, Description: `You invoke the Doctrine's punishing judgment, amplifying the target's pain and imposing conditions based on injury severity.`},
	`tremor_signs_bio_synthetic`: {Name: `Ground Vibration Signal`, Description: `You transmit coded vibration pulses through the earth, sending a short message to those with seismic sensitivity.`},
	`tremor_signs_fanatic_doctrine`: {Name: `Doctrine Ground Signal`, Description: `You transmit coded doctrinal vibrations through the earth, sending a short message to those attuned to receive them.`},
	`tremor_signs_neural`: {Name: `Ground Wave Signal`, Description: `You send a series of coded vibrations through the ground, transmitting a short message to anyone who can feel it.`},
	`unbroken_panoply_neural`: {Name: `Unbroken Panoply`, Description: `As tools of violence undone by violence, broken weapons contain potent symbolic magic that faydhaans often call upon when forming alliances. Images of similar legendary weapons overlay the target, and the weapon's broken condition is suppressed for the duration. The weapon gains the nonlethal trait during this time. The weapon's wielder can apply the weapon's item bonus to attack rolls, if any, to their Diplomacy checks. If the weapon would be damaged or broken again, this spell ends.`},
	`unbroken_panoply_technical`: {Name: `Emergency Repair Override`, Description: `Trigger an emergency repair subroutine that restores armor integrity under combat conditions.`},
	`vanishing_tracks`: {Name: `Vanishing Tracks`, Description: `You obscure the tracks you leave behind. The DC of checks to Track you gains a +4 status bonus or is equal to your spell DC, whichever results in a higher DC.Heightened (2nd) The duration increases to 8 hours.
Heightened (4th) The duration increases to 8 hours. The spell has a range of 20 feet and targets up to 10 creatures.`},
	`ventriloquism_bio_synthetic`: {Name: `Ventriloquism`, Description: `Whenever you speak or make any other sound vocally, you can make your vocalization seem to originate from somewhere else within 60 feet, and you can change that apparent location freely as you vocalize. Any creature that hears the sound can attempt to disbelieve your illusion.

Heightened (2nd) The spell's duration increases to 1 hour, and you can also change the tone, quality, and other aspects of your voice. Before a creature can attempt to disbelieve your illusion, it must actively attempt a Perception check or otherwise use actions to interact with the sound.`},
	`ventriloquism_fanatic_doctrine`: {Name: `Ventriloquism`, Description: `Whenever you speak or make any other sound vocally, you can make your vocalization seem to originate from somewhere else within 60 feet, and you can change that apparent location freely as you vocalize. Any creature that hears the sound can attempt to disbelieve your illusion.

Heightened (2nd) The spell's duration increases to 1 hour, and you can also change the tone, quality, and other aspects of your voice. Before a creature can attempt to disbelieve your illusion, it must actively attempt a Perception check or otherwise use actions to interact with the sound.`},
	`ventriloquism_neural`: {Name: `Ventriloquism`, Description: `Whenever you speak or make any other sound vocally, you can make your vocalization seem to originate from somewhere else within 60 feet, and you can change that apparent location freely as you vocalize. Any creature that hears the sound can attempt to disbelieve your illusion.

Heightened (2nd) The spell's duration increases to 1 hour, and you can also change the tone, quality, and other aspects of your voice. Before a creature can attempt to disbelieve your illusion, it must actively attempt a Perception check or otherwise use actions to interact with the sound.`},
	`ventriloquism_technical`: {Name: `Voice Projection Speaker`, Description: `Route voice output through a remote speaker to project sound from any location.`},
	`verdant_sprout`: {Name: `Verdant Sprout`, Description: `You imbue a single ordinary, inexpensive plant seed with primal energy and throw it onto a surface, where it gradually sprouts into a Medium plant. After 10 minutes, the plant is sturdy enough to provide standard cover, and its space is difficult terrain. The plant is laden with nutritious nuts or fruit sufficient to feed one Medium creature for a day. The plant has AC 10, Hardness 5, and 20 Hit Points.

Heightened (+1) You throw an additional seed, which grows into an additional plant within range.`},
	`verminous_lure`: {Name: `Verminous Lure`, Description: `Upon casting, the target emits a musk that's captivating to certain animals. Tiny animals and animal swarms of any size within range must attempt a Will save. On a failure, non-hostile animals or animal swarms try to touch the target. If hostile, such creatures choose to attack the target instead of other foes, if able to do so without spending additional actions or exposing themselves to additional danger.
Verminous lure doesn't change animals' attitudes towards the target and is easily overridden by more direct control, such as the Command an Animal action. Animals with imprecise sense can use their scent as a precise sense against the target.`},
	`viscous_spray`: {Name: `Viscous Spray`, Description: `A bio-synthetic adhesive secretion coats a target's joints and limbs, restraining movement.`},
	`vitality_lash_bio_synthetic`: {Name: `Vitality Lash`, Description: `You demolish the target's corrupted essence with energy from Creation's Forge. You deal 2d6 vitality damage with a basic Fortitude save. If the creature critically fails the save, it is also Enfeebled 1 until the start of your next turn.

Heightened (+1) The damage increases by 1d6.`},
	`vitality_lash_fanatic_doctrine`: {Name: `Doctrine's Vitality Lash`, Description: `You lash the target with a whip of vital energy, dealing damage that can heal the faithful and harm the wicked.`},
	`void_warp_fanatic_doctrine`: {Name: `Doctrine's Void Judgment`, Description: `You briefly phase a target through the Doctrine's void, dealing void damage and disrupting their material cohesion.`},
	`void_warp_neural`: {Name: `Void Phase Shift`, Description: `You briefly phase a target partially out of existence, dealing void damage and disrupting their material cohesion.`},
	`wall_of_shrubs_bio_synthetic`: {Name: `Wall of Shrubs`, Description: `You call forth a line of bushes native to the region to spring from the ground. The wall of shrubs stands in a line 60 feet long, is less than 5 feet tall, and is a foot thick, providing lesser cover.

Heightened (3rd) The shrubs are 10 feet tall and 5 feet thick, provide standard cover, are difficult terrain, and have a Climb DC of 15. The duration increases to 10 minutes.
Heightened (5th) As 3rd rank, but the shrubs provide greater cover and the duration is 1 hour. You can choose to form a ring of shrubs with a diameter of up to 30 feet instead of a line.`},
	`wall_of_shrubs_technical`: {Name: `Deployable Terrain Wall`, Description: `Rapidly deploy a modular terrain barrier system for cover and area control.`},
	`warp_step_neural`: {Name: `Micro-Teleport`, Description: `You execute a short-range spatial teleport, stepping instantly from one position to another within range.`},
	`wash_your_luck_fanatic_doctrine`: {Name: `Doctrine's Luck Cleansing`, Description: `You invoke the Doctrine's authority to wash away bad luck from a target, allowing them to reroll a recent failure.`},
	`wash_your_luck_neural`: {Name: `Fortune Reset Protocol`, Description: `You broadcast a psionic fortune-reset signal, clearing accumulated bad luck from a target and allowing them to reroll a recent failure.`},
	`weaken_earth_bio_synthetic`: {Name: `Weaken Earth`, Description: `You weaken the bonds that hold earth and stone together. If your target has Hardness, you can affect one contiguous object, up to a 5-foot cube, or one creature, decreasing the Hardness by 5, to a minimum of 0. If the target lacks Hardness, it gains weakness 3 to physical damage. A target with a Fortitude modifier can attempt a Fortitude saving throw, negating the effect on a success.
@UUID[Compendium.pf2e.spell-effects.Item.Spell Effect: Weaken Earth]

Heightened (+2) Hardness decreases by 5, the size of a contiguous object increases by one 5-foot cube, and the weakness increases by 3.`},
	`weaken_earth_technical`: {Name: `Ground Destabilizer Charge`, Description: `Plant a charge that vibrates soil composition and turns solid ground unstable.`},
	`weave_wood_bio_synthetic`: {Name: `Weave Wood`, Description: `With a touch, you cause the target to break into fibrous strands that then weave themselves into a woven mundane object of the same Bulk or less, such as a basket, hat, shield, or mat. You can create up to 4 objects with one casting of this spell, providing their total Bulk doesn't exceed the Bulk of your target. The objects have Hardness 2 and 8 Hit Points.

Heightened (+1) Increase the maximum Bulk that you can target by 1 and the maximum number of objects you can create by 2.`},
	`weave_wood_technical`: {Name: `Polymer Shaper Tool`, Description: `Use a focused tool to rapidly shape polymer or wood composites into usable forms.`},
	`wooden_fists_bio_synthetic`: {Name: `Wooden Fists`, Description: `Your arms and hands swell with new growth, transforming into tree trunks twice as big as their current size. Your fists deal 1d6 bludgeoning damage, lose the nonlethal trait, and have reach.

Heightened (3rd) Your fists gain the magical trait and become a striking weapon, increasing the damage your fists deal to 2d6 bludgeoning.
Heightened (7th) Your fists gain the magical trait and become a greater striking weapon, increasing the damage your fists deal to 3d6 bludgeoning. The duration is 10 minutes.
Heightened (9th) Your fists gain the magical trait and become a major striking weapon, increasing the damage your fists deal to 4d6 bludgeoning. The duration is 1 hour.
@UUID[Compendium.pf2e.spell-effects.Item.Spell Effect: Wooden Fists]`},
	`wooden_fists_technical`: {Name: `Impact-Reinforced Gauntlets`, Description: `Activate hardened impact gauntlets that dramatically increase unarmed strike force.`},
}
