#!/usr/bin/env bash
set -euo pipefail

BEHAVIOR="implementer"
OWNER="cory-johannsen"
PROJECT_NUMBER="1"
INTERVAL="${WATCH_INTERVAL:-600}"
LOG="/tmp/mud-watch-${BEHAVIOR}-$$.log"

echo "# watch-implementer pid=$$ started $(date -u +%Y-%m-%dT%H:%M:%SZ) log=$LOG interval=${INTERVAL}s" >&2
echo "# watch-implementer pid=$$ started $(date -u +%Y-%m-%dT%H:%M:%SZ)" >> "$LOG"

declare -A SEEN

emit() {
  local event="$1" num="$2" title="$3"
  local ts
  ts=$(date -u +%Y-%m-%dT%H:%M:%SZ)
  printf '%s\t%s\t%s\t%s\t%s\n' "$ts" "$BEHAVIOR" "$event" "$num" "$title" | tee -a "$LOG"
}

poll_once() {
  local json
  if ! json=$(gh project item-list "$PROJECT_NUMBER" --owner "$OWNER" --format json --limit 100 2>/dev/null); then
    echo "# $(date -u +%Y-%m-%dT%H:%M:%SZ) poll failed" >> "$LOG"
    return 0
  fi

  local current_keys=()
  local num title key

  # bug-open: Todo status AND label contains "bug"
  # Severity-rank in jq so sort|cut stays a simple 2-field strip.
  while IFS=$'\t' read -r num title; do
    [[ -z "$num" || "$num" == "0" ]] && continue
    key="bug-open:$num"
    current_keys+=("$key")
    if [[ -z "${SEEN[$key]:-}" ]]; then
      emit "bug-open" "$num" "$title"
      SEEN[$key]=1
    fi
  done < <(
    echo "$json" | jq -r '
      .items[]
      | select(.status == "Todo")
      | select((.labels // []) | index("bug"))
      | ((.labels // []) | map(select(startswith("severity: "))) | (.[0] // "severity: unknown")) as $sev
      | ( if   $sev == "severity: critical" then 0
          elif $sev == "severity: high"     then 1
          elif $sev == "severity: medium"   then 2
          elif $sev == "severity: low"      then 3
          else 4 end ) as $rank
      | [ $rank, (.content.number // 0), (.title // "") ]
      | @tsv
    ' | sort -k1,1n -k2,2n | cut -f2-
  )

  # planned-ready: Planned status
  while IFS=$'\t' read -r num title; do
    [[ -z "$num" || "$num" == "0" ]] && continue
    key="planned-ready:$num"
    current_keys+=("$key")
    if [[ -z "${SEEN[$key]:-}" ]]; then
      emit "planned-ready" "$num" "$title"
      SEEN[$key]=1
    fi
  done < <(
    echo "$json" | jq -r '
      .items[]
      | select(.status == "Planned")
      | [ (.content.number // 0), (.title // "") ]
      | @tsv
    '
  )

  # WATCH-7 dedupe reset: drop SEEN entries whose issue is no longer in the triggering state
  local active=" ${current_keys[*]} "
  for key in "${!SEEN[@]}"; do
    if [[ "$active" != *" $key "* ]]; then
      unset 'SEEN[$key]'
    fi
  done
}

trap 'echo "# watch-implementer pid=$$ stopped $(date -u +%Y-%m-%dT%H:%M:%SZ)" >> "$LOG"; exit 0' INT TERM

while true; do
  poll_once
  sleep "$INTERVAL"
done
