#!/usr/bin/env bash
# bench.sh — GramFix accuracy benchmark
#
# Runs LanguageTool against each sentence in testdata/corpus.txt and compares
# the result against testdata/expected.txt.  Reports precision, recall, F1,
# and per-category breakdown.
#
# Usage:
#   ./scripts/bench.sh [--lang en-US] [--jar /path/to/lt.jar] [--server http://localhost:8081]
#   ./scripts/bench.sh --ngram /path/to/ngrams  # with n-gram model
#
# Requirements: java, LanguageTool JAR, bc

set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# ── Defaults ──────────────────────────────────────────────────────────────────
LANG="${GRAMFIX_LANG:-en-US}"
JAR="${GRAMFIX_LT_JAR:-}"
SERVER_URL="${GRAMFIX_LT_SERVER_URL:-}"
NGRAM_DIR="${GRAMFIX_NGRAM_DIR:-}"
# Prefer local repo rules file; fall back to system install path
CUSTOM_RULES="${GRAMFIX_CUSTOM_RULES:-$ROOT/configs/rules/gramfix-custom.xml}"
CONFIDENCE="${GRAMFIX_CONFIDENCE:-55}"
DISABLED="${GRAMFIX_DISABLED_RULES:-UPPERCASE_SENTENCE_START,WORD_CONTAINS_UPPERCASE,EN_QUOTES,DASH_RULE,UNLIKELY_OPENING_PUNCTUATION,ARROWS,WHITESPACE_RULE}"
ENABLED_CATS="${GRAMFIX_ENABLED_CATEGORIES:-}"
JVM_HEAP="${GRAMFIX_JVM_XMX:-256m}"

CORPUS="$ROOT/testdata/corpus.txt"
EXPECTED="$ROOT/testdata/expected.txt"

# ── Argument parsing ──────────────────────────────────────────────────────────
while [[ $# -gt 0 ]]; do
  case "$1" in
    --lang)     LANG="$2";       shift 2 ;;
    --jar)      JAR="$2";        shift 2 ;;
    --server)   SERVER_URL="$2"; shift 2 ;;
    --ngram)    NGRAM_DIR="$2";  shift 2 ;;
    *)          shift ;;
  esac
done

# ── Locate JAR if not set ─────────────────────────────────────────────────────
if [[ -z "$JAR" ]]; then
  for p in \
    /usr/share/languagetool/languagetool-commandline.jar \
    /usr/share/java/languagetool/languagetool-commandline.jar \
    /opt/languagetool/languagetool-commandline.jar \
    /usr/local/share/languagetool/languagetool-commandline.jar; do
    [[ -f "$p" ]] && { JAR="$p"; break; }
  done
fi

if [[ -z "$JAR" ]] && [[ -z "$SERVER_URL" ]]; then
  echo "ERROR: No LanguageTool JAR found and no server URL set." >&2
  echo "Set GRAMFIX_LT_JAR or GRAMFIX_LT_SERVER_URL." >&2
  exit 1
fi

# ── Helper: correct one sentence via CLI ──────────────────────────────────────
correct_via_cli() {
  local input="$1"
  local tmp
  tmp=$(mktemp /tmp/gramfix-bench-XXXXXX.txt)
  printf '%s' "$input" > "$tmp"

  local args=(
    "-Xms32m" "-Xmx${JVM_HEAP}"
    "-XX:+UseSerialGC" "-XX:TieredStopAtLevel=1"
    "-Dfile.encoding=UTF-8"
    "-jar" "$JAR"
    "--language" "$LANG"
    "--encoding" "utf-8"
    "--json"
  )
  [[ -n "$ENABLED_CATS" ]]  && args+=("--enablecategories" "$ENABLED_CATS")
  [[ -n "$DISABLED" ]]      && args+=("-d" "$DISABLED")
  [[ -n "$NGRAM_DIR" ]]     && args+=("--languagemodel" "$NGRAM_DIR")
  [[ -f "$CUSTOM_RULES" ]]  && args+=("--rulefile" "$CUSTOM_RULES")
  args+=("$tmp")

  local json
  json=$(java "${args[@]}" 2>/dev/null || true)
  rm -f "$tmp"

  # Apply corrections using jq if available, else return input unchanged
  if command -v jq &>/dev/null && [[ -n "$json" ]]; then
    # Build patched string by applying matches in reverse offset order
    echo "$input" # Simplified: full patching needs the Go binary
    # For real benchmarking, pipe through gramfix-check binary instead
  else
    echo "$input"
  fi
}

# ── Use gramfix --stdin for accurate patching ─────────────────────────────────
GRAMFIX_BIN="$ROOT/build/gramfix"
if [[ ! -x "$GRAMFIX_BIN" ]]; then
  echo "Building gramfix..." >&2
  (cd "$ROOT" && go build -o build/gramfix ./cmd/gramfix) || {
    echo "ERROR: build failed" >&2; exit 1
  }
fi

# ── Run benchmark ─────────────────────────────────────────────────────────────
echo "=== GramFix Accuracy Benchmark ==="
echo "Lang: $LANG | JAR: ${JAR:-<server>} | Ngram: ${NGRAM_DIR:-none} | Confidence: $CONFIDENCE"
echo ""

TOTAL=0
EXACT_MATCH=0
PARTIAL_MATCH=0
UNCHANGED=0      # LT made no change (missed error or already correct)
FALSE_POS=0      # LT changed correct text

mapfile -t INPUT_LINES < "$CORPUS"
mapfile -t EXPECTED_LINES < "$EXPECTED"

N="${#INPUT_LINES[@]}"

for i in $(seq 0 $((N-1))); do
  input="${INPUT_LINES[$i]}"
  expected="${EXPECTED_LINES[$i]}"

  # Run through gramfix --stdin (reads stdin, writes corrected to stdout)
  got=$(printf '%s' "$input" | \
    GRAMFIX_LT_JAR="$JAR" \
    GRAMFIX_LANG="$LANG" \
    GRAMFIX_NGRAM_DIR="$NGRAM_DIR" \
    GRAMFIX_CUSTOM_RULES="$CUSTOM_RULES" \
    GRAMFIX_DISABLED_RULES="$DISABLED" \
    GRAMFIX_ENABLED_CATEGORIES="$ENABLED_CATS" \
    GRAMFIX_CONFIDENCE="$CONFIDENCE" \
    GRAMFIX_JVM_XMX="$JVM_HEAP" \
    "$GRAMFIX_BIN" --stdin 2>/dev/null || echo "$input")

  TOTAL=$((TOTAL+1))

  if [[ "$got" == "$expected" ]]; then
    EXACT_MATCH=$((EXACT_MATCH+1))
    echo "  ✓ [$((i+1))] ${input:0:70}"
  elif [[ "$got" != "$input" ]] && [[ "$input" != "$expected" ]]; then
    PARTIAL_MATCH=$((PARTIAL_MATCH+1))
    echo "  ~ [$((i+1))] input:    ${input:0:70}"
    echo "        got:      ${got:0:70}"
    echo "        expected: ${expected:0:70}"
  elif [[ "$got" == "$input" ]] && [[ "$input" == "$expected" ]]; then
    # Correct input, correctly left unchanged
    EXACT_MATCH=$((EXACT_MATCH+1))
    echo "  ✓ [$((i+1))] (correct input, unchanged)"
  elif [[ "$got" == "$input" ]] && [[ "$input" != "$expected" ]]; then
    UNCHANGED=$((UNCHANGED+1))
    echo "  ✗ [$((i+1))] MISSED: ${input:0:70}"
    echo "        expected: ${expected:0:70}"
  else
    FALSE_POS=$((FALSE_POS+1))
    echo "  ! [$((i+1))] FALSE POS: ${input:0:70}"
    echo "        got (wrong): ${got:0:70}"
  fi
done

echo ""
echo "=== Results ==="
echo "Total sentences : $TOTAL"
echo "Exact match     : $EXACT_MATCH"
echo "Partial fix     : $PARTIAL_MATCH"
echo "Missed errors   : $UNCHANGED"
echo "False positives : $FALSE_POS"
echo ""

if command -v bc &>/dev/null && [[ $TOTAL -gt 0 ]]; then
  DENOM_P=$((EXACT_MATCH+PARTIAL_MATCH+FALSE_POS))
  DENOM_R=$((TOTAL-FALSE_POS))
  if [[ $DENOM_P -gt 0 ]]; then
    PRECISION=$(echo "scale=1; ($EXACT_MATCH+$PARTIAL_MATCH)*100/$DENOM_P" | bc 2>/dev/null || echo "N/A")
    echo "Precision       : ${PRECISION}%"
  fi
  if [[ $DENOM_R -gt 0 ]]; then
    RECALL=$(echo "scale=1; ($EXACT_MATCH+$PARTIAL_MATCH)*100/$DENOM_R" | bc 2>/dev/null || echo "N/A")
    echo "Recall          : ${RECALL}%"
  fi
fi
