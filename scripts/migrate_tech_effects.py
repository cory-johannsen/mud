#!/usr/bin/env python3
"""
Technology effects migration runner.

Addresses GitHub issues #98 (Foundry markdown in descriptions) and
#100 (missing effects implementations).

Usage:
    cd /home/cjohannsen/src/mud/scripts
    python migrate_tech_effects.py --tradition bio_synthetic --dry-run
    python migrate_tech_effects.py --tradition bio_synthetic --apply
    python migrate_tech_effects.py --all --apply
"""
import argparse
import sys
from pathlib import Path
import yaml

sys.path.insert(0, str(Path(__file__).parent))

from tech_migration.parser import parse_damage_annotations, parse_condition_annotations, parse_check_annotations
from tech_migration.effects_builder import build_effects
from tech_migration.description_rewriter import rewrite_description

TECH_ROOT = Path(__file__).parent.parent / "content" / "technologies"
TRADITIONS = ["bio_synthetic", "fanatic_doctrine", "neural", "technical", "innate"]


def has_foundry_markup(text: str) -> bool:
    return any(marker in text for marker in ["@Damage", "@UUID", "@Check", "@item.", "@actor."])


def is_placeholder_effects(effects: dict) -> bool:
    if not effects:
        return True
    for slot_effects in effects.values():
        for e in slot_effects:
            if e.get("type") not in ("utility", None):
                return False
    return True


def process_file(path: Path, apply: bool) -> dict:
    report = {
        "name": path.stem,
        "path": str(path.relative_to(TECH_ROOT.parent.parent)),
        "changed": False,
        "needs_review": False,
        "notes": [],
        "desc_changed": False,
        "effects_changed": False,
    }

    with open(path) as f:
        data = yaml.safe_load(f)

    if not isinstance(data, dict):
        report["notes"].append("not a dict — skipped")
        return report

    description = data.get("description", "")
    existing_effects = data.get("effects", {}) or {}
    resolution = data.get("resolution", "none")

    # Use original description for parsing (markup is informative)
    raw_description = description

    # --- Description cleanup ---
    if has_foundry_markup(description):
        new_description = rewrite_description(description)
        if new_description != description:
            report["desc_changed"] = True
            report["notes"].append("description: Foundry markup stripped")
            if apply:
                data["description"] = new_description

    # --- Effects population ---
    if is_placeholder_effects(existing_effects):
        damages = parse_damage_annotations(raw_description)
        conditions = parse_condition_annotations(raw_description)
        checks = parse_check_annotations(raw_description)

        if damages or conditions or checks:
            result = build_effects(
                resolution=resolution,
                existing_effects=existing_effects,
                damages=damages,
                conditions=conditions,
                checks=checks,
            )

            if result.effects and result.effects != existing_effects:
                report["effects_changed"] = True
                report["notes"].append(f"effects: populated {list(result.effects.keys())}")
                if result.skipped_conditions:
                    report["notes"].append(f"skipped conditions (no mapping): {result.skipped_conditions}")
                if result.needs_review:
                    report["needs_review"] = True
                    report["notes"].extend(result.notes)

                if apply:
                    data["effects"] = result.effects
                    if result.save_type and not data.get("save_type"):
                        data["save_type"] = result.save_type
                    if result.save_dc and not data.get("save_dc"):
                        data["save_dc"] = result.save_dc
            elif result.needs_review:
                report["needs_review"] = True
                report["notes"].extend(result.notes)
        else:
            report["needs_review"] = True
            report["notes"].append("no parseable mechanical data in description")

    report["changed"] = report["desc_changed"] or report["effects_changed"]

    if apply and report["changed"]:
        with open(path, "w") as f:
            yaml.dump(data, f, allow_unicode=True, default_flow_style=False, sort_keys=False, width=10000)

    return report


def run(traditions: list, apply: bool, report_only: bool):
    total = changed = needs_review = 0
    review_list = []

    for tradition in traditions:
        tradition_path = TECH_ROOT / tradition
        if not tradition_path.exists():
            print(f"WARNING: tradition path not found: {tradition_path}")
            continue

        files = sorted(tradition_path.glob("*.yaml"))
        tradition_changed = 0

        print(f"\n{'='*60}")
        print(f"Tradition: {tradition} ({len(files)} files)")
        print(f"{'='*60}")

        for path in files:
            report = process_file(path, apply=apply and not report_only)
            total += 1

            if report["changed"]:
                tradition_changed += 1
                changed += 1
                action = "APPLY" if apply else "DRY-RUN"
                print(f"  [{action}] {report['name']}")
                for note in report["notes"]:
                    print(f"         {note}")

            if report["needs_review"]:
                needs_review += 1
                review_list.append(report)

        print(f"  -> {tradition_changed} files changed in {tradition}")

    print(f"\n{'='*60}")
    print(f"SUMMARY")
    print(f"{'='*60}")
    print(f"  Total files processed: {total}")
    print(f"  Files changed:         {changed}")
    print(f"  Files needing review:  {needs_review}")

    if review_list and len(review_list) <= 50:
        print(f"\nFILES NEEDING MANUAL REVIEW ({len(review_list)}):")
        for r in review_list:
            print(f"  {r['path']}: {'; '.join(r['notes'])}")


def main():
    parser = argparse.ArgumentParser(description="Technology effects migration tool")
    group = parser.add_mutually_exclusive_group(required=True)
    group.add_argument("--tradition", choices=TRADITIONS, help="Process a single tradition")
    group.add_argument("--all", action="store_true", help="Process all traditions")
    parser.add_argument("--dry-run", action="store_true", help="Show changes without writing")
    parser.add_argument("--apply", action="store_true", help="Write changes to files")
    parser.add_argument("--report", action="store_true", help="Generate review report only")
    args = parser.parse_args()

    if not args.dry_run and not args.apply and not args.report:
        parser.error("specify one of --dry-run, --apply, or --report")

    traditions = TRADITIONS if args.all else [args.tradition]
    run(traditions=traditions, apply=args.apply, report_only=args.report)


if __name__ == "__main__":
    main()
