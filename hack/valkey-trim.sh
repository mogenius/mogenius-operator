#!/usr/bin/env bash
#
# valkey-trim.sh — one-shot cleanup of the operator's Valkey instance.
#
# Trims pod-stats / traffic-stats / node-stats / logs streams to the
# configured limit so memory is reclaimed immediately instead of waiting
# for the next write to each stream. Safe to re-run; XTRIM is idempotent.
#
# Usage:
#   hack/valkey-trim.sh                # uses defaults (1440 entries for stats, 500 for logs)
#   STATS_MAXLEN=2880 LOGS_MAXLEN=1000 hack/valkey-trim.sh
#   NAMESPACE=other-ns hack/valkey-trim.sh
#   DRY_RUN=1 hack/valkey-trim.sh      # only prints what would be done

set -euo pipefail

NAMESPACE="${NAMESPACE:-mogenius}"
SECRET="${SECRET:-mogenius-operator-valkey}"
SECRET_KEY="${SECRET_KEY:-valkey-password}"
POD_SELECTOR="${POD_SELECTOR:-app=mogenius-operator-valkey}"

STATS_MAXLEN="${STATS_MAXLEN:-1440}"  # 24h @ 1m write cadence
LOGS_MAXLEN="${LOGS_MAXLEN:-500}"
DRY_RUN="${DRY_RUN:-0}"

POD=$(kubectl -n "$NAMESPACE" get pod -l "$POD_SELECTOR" -o jsonpath='{.items[0].metadata.name}')
if [[ -z "$POD" ]]; then
  echo "could not find valkey pod with selector '$POD_SELECTOR' in namespace '$NAMESPACE'" >&2
  exit 1
fi
PW=$(kubectl -n "$NAMESPACE" get secret "$SECRET" -o jsonpath="{.data.$SECRET_KEY}" | base64 -d)

rc() { kubectl -n "$NAMESPACE" exec -i "$POD" -- redis-cli -a "$PW" --no-auth-warning "$@"; }
rc_stdin() { kubectl -n "$NAMESPACE" exec -i "$POD" -- redis-cli -a "$PW" --no-auth-warning; }

echo "=== Valkey pod: $POD (ns: $NAMESPACE) ==="
echo "=== Before ==="
rc INFO memory | grep -E '^(used_memory_human|used_memory_dataset):'
BEFORE_DBSIZE=$(rc DBSIZE | tr -d '[:space:]')
echo "keys: $BEFORE_DBSIZE"
echo

trim_pattern() {
  local pattern="$1"
  local maxlen="$2"
  local label="$3"

  local tmp
  tmp=$(mktemp)
  rc --scan --pattern "$pattern" --count 1000 > "$tmp" 2>/dev/null || true
  local count
  count=$(wc -l < "$tmp" | tr -d '[:space:]')
  echo "[$label] $count streams matching $pattern -> trim to MAXLEN ~ $maxlen"

  if [[ "$count" -eq 0 ]]; then
    rm -f "$tmp"
    return
  fi

  if [[ "$DRY_RUN" == "1" ]]; then
    echo "  (dry-run, no changes)"
    rm -f "$tmp"
    return
  fi

  # Build XTRIM commands and pipe through a single redis-cli session.
  awk -v n="$maxlen" '{printf "XTRIM %s MAXLEN ~ %d\n", $0, n}' "$tmp" \
    | rc_stdin > /dev/null
  rm -f "$tmp"
}

trim_pattern 'pod-stats:*'     "$STATS_MAXLEN" "pod-stats"
trim_pattern 'traffic-stats:*' "$STATS_MAXLEN" "traffic-stats"
trim_pattern 'node-stats:*'    "$STATS_MAXLEN" "node-stats"
trim_pattern 'logs:*'          "$LOGS_MAXLEN"  "logs"

echo
echo "=== After ==="
rc INFO memory | grep -E '^(used_memory_human|used_memory_dataset):'
echo "keys: $(rc DBSIZE | tr -d '[:space:]')"
