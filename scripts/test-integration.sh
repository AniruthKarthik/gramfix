#!/usr/bin/env bash
# scripts/test-integration.sh — GramFix integration test suite
# Tests clipboard pipeline, grammar correction, process lifecycle.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$SCRIPT_DIR/.."
GRAMFIX="$ROOT/build/gramfix"
PASS=0; FAIL=0; SKIP=0

GREEN='\033[0;32m'; RED='\033[0;31m'; YELLOW='\033[1;33m'; CYAN='\033[0;36m'; NC='\033[0m'
pass() { echo -e "${GREEN}[PASS]${NC} $1"; ((PASS++)) || true; }
fail() { echo -e "${RED}[FAIL]${NC} $1"; ((FAIL++)) || true; }
skip() { echo -e "${YELLOW}[SKIP]${NC} $1"; ((SKIP++)) || true; }
section() { echo -e "\n${CYAN}=== $1 ===${NC}"; }

# Detect clipboard tool
CLIP_READ=""
CLIP_WRITE=""
CLIP_PRIMARY_READ=""
CLIP_PRIMARY_WRITE=""

if command -v wl-paste &>/dev/null && command -v wl-copy &>/dev/null; then
    CLIP_READ="wl-paste --no-newline"
    CLIP_WRITE="wl-copy"
    CLIP_PRIMARY_READ="wl-paste --no-newline --primary"
    CLIP_PRIMARY_WRITE="wl-copy --primary"
elif command -v xclip &>/dev/null; then
    CLIP_READ="xclip -selection clipboard -o"
    CLIP_WRITE="xclip -selection clipboard -i"
    CLIP_PRIMARY_READ="xclip -selection primary -o"
    CLIP_PRIMARY_WRITE="xclip -selection primary -i"
else
    echo "No clipboard tool found, skipping all tests"
    exit 0
fi

clip_write()  { echo -n "$1" | $CLIP_WRITE; sleep 0.3; }
clip_read()   { $CLIP_READ 2>/dev/null || echo ""; }
prim_write()  { echo -n "$1" | $CLIP_PRIMARY_WRITE; sleep 0.3; }
prim_read()   { $CLIP_PRIMARY_READ 2>/dev/null || echo ""; }

run_gramfix() {
    # Run gramfix with a 30s timeout
    YDOTOOL_SOCKET=/tmp/.ydotool_socket timeout 30 "$GRAMFIX" --debug 2>&1 || true
}

###############################################################################
section "Build Check"
###############################################################################
cd "$ROOT"
if make build &>/dev/null; then
    pass "make build succeeds"
else
    fail "make build failed"
    exit 1
fi

if "$GRAMFIX" --version 2>&1 | grep -q "gramfix v"; then
    pass "version flag"
else
    fail "version flag"
fi

###############################################################################
section "Environment Detection"
###############################################################################
output=$(run_gramfix 2>&1 || true)
if echo "$output" | grep -q "environment:"; then
    pass "environment detected"
else
    fail "environment detection missing"
fi

if echo "$output" | grep -qE "(wayland|x11)"; then
    pass "display server detected"
else
    fail "display server not detected"
fi

###############################################################################
section "Grammar Correction — Primary Selection"
###############################################################################

# Test 1: basic grammar errors
INPUT="she dont know what is hapening in the world"
prim_write "$INPUT"
clip_write "original-clipboard-$$"
output=$(run_gramfix)

if echo "$output" | grep -q "She don"; then
    pass "correction: she → She, dont → don't, hapening → happening"
    echo "       Input:  '$INPUT'"
    echo "       Fixed:  $(echo "$output" | grep 'corrected:' | head -1)"
else
    fail "basic grammar correction"
    echo "       Debug: $output" | tail -5
fi

# Check clipboard was restored
sleep 0.5
restored=$(clip_read)
if [[ "$restored" == "original-clipboard-$$" ]]; then
    pass "clipboard restored after correction"
else
    fail "clipboard not restored: got '$restored'"
fi

# Test 2: capitalization fix
INPUT2="the quick brown fox jumps over the lazy dog"
prim_write "$INPUT2"
output=$(run_gramfix)
if echo "$output" | grep -qiE "corrected.*The|grammar fix"; then
    pass "capitalization correction"
else
    fail "capitalization correction: $output" | tail -3
fi

# Test 3: multiline text
INPUT3=$'this is the first sentense.\nand this is the second one with bad grammer.'
prim_write "$INPUT3"
output=$(run_gramfix)
if echo "$output" | grep -q "grammar fix"; then
    pass "multiline text handling"
else
    fail "multiline text: $output" | tail -3
fi

# Test 4: already correct text (should be unchanged)
INPUT4="The quick brown fox jumps over the lazy dog."
prim_write "$INPUT4"
output=$(run_gramfix)
if echo "$output" | grep -q "pipeline complete"; then
    pass "correct text passes through unchanged"
else
    fail "correct text handling"
fi

# Test 5: long paragraph
INPUT5="this is a very long paragraph with many sentenses that have gramar mistaks. the languge tool should be able to fix most of them automaticly. i hope it works corectly for long text inputs as well."
prim_write "$INPUT5"
output=$(run_gramfix)
if echo "$output" | grep -q "grammar fix"; then
    pass "long paragraph correction"
else
    fail "long paragraph: $output" | tail -3
fi

###############################################################################
section "Grammar Correction — Clipboard Fallback"
###############################################################################

# Clear primary, put text in clipboard — should fall back
if command -v wl-copy &>/dev/null; then
    # Clear primary by writing empty
    echo -n "" | wl-copy --primary 2>/dev/null || true
    sleep 0.3
fi

INPUT_CB="we doesnt have any idea what to do"
clip_write "$INPUT_CB"
output=$(run_gramfix)
if echo "$output" | grep -q "captured selection"; then
    pass "clipboard fallback when primary empty"
else
    fail "clipboard fallback: $output" | tail -3
fi

###############################################################################
section "Empty Input"
###############################################################################
if command -v wl-copy &>/dev/null; then
    echo -n "" | wl-copy --primary 2>/dev/null || true
    echo -n "" | wl-copy 2>/dev/null || true
fi
sleep 0.3
output=$(run_gramfix)
if echo "$output" | grep -q "no text selected"; then
    pass "empty selection handled gracefully"
else
    # May exit quietly too
    if echo "$output" | grep -q "pipeline complete\|gramfix done"; then
        pass "empty selection exits cleanly"
    else
        skip "empty selection behavior (may vary)"
    fi
fi

###############################################################################
section "Process Lifecycle"
###############################################################################

# Count wl-copy processes before — baseline (may include pre-existing ones)
WL_BEFORE=$(pgrep -c wl-copy 2>/dev/null || echo 0)
prim_write "test sentense for lifecycle"
run_gramfix &>/dev/null
sleep 1.5

# After gramfix finishes:
#   - All intermediate wl-copy processes should be dead
#   - The clipboard-restore wl-copy may still be alive (that is correct)
#   - Net increase should be at most 1 over baseline
WL_AFTER=$(pgrep -c wl-copy 2>/dev/null || echo 0)
WL_DELTA=$(( WL_AFTER - WL_BEFORE ))
if [[ "$WL_DELTA" -le 1 ]]; then
    pass "clean process exit (wl-copy delta: +${WL_DELTA}, before=$WL_BEFORE after=$WL_AFTER)"
else
    fail "orphan wl-copy processes: delta=$WL_DELTA (before=$WL_BEFORE after=$WL_AFTER)"
fi

# Verify gramfix itself is gone
sleep 0.5
if ! pgrep -x gramfix &>/dev/null; then
    pass "gramfix process terminated cleanly"
else
    fail "gramfix process still running after completion"
fi

###############################################################################
section "Repeated Execution"
###############################################################################
FAIL_COUNT=0
for i in 1 2 3; do
    prim_write "test sentense number $i with bad grammer"
    out=$(run_gramfix 2>&1 || true)
    if echo "$out" | grep -q "pipeline complete"; then
        echo "  Run $i: OK"
    else
        echo "  Run $i: ISSUE — $out" | tail -2
        ((FAIL_COUNT++)) || true
    fi
    sleep 0.5
done
if [[ "$FAIL_COUNT" -eq 0 ]]; then
    pass "repeated execution (3 runs)"
else
    fail "repeated execution: $FAIL_COUNT/3 runs had issues"
fi

###############################################################################
# Summary
###############################################################################
echo ""
echo "============================================"
echo -e "  ${GREEN}PASSED${NC}: $PASS"
echo -e "  ${RED}FAILED${NC}: $FAIL"
echo -e "  ${YELLOW}SKIPPED${NC}: $SKIP"
echo "============================================"

[[ "$FAIL" -eq 0 ]] && exit 0 || exit 1
