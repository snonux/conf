#!/usr/bin/env bash
# End-to-end smoke test for a running bgtutor MCP server: authentication
# (bearer token and ?token=), the four MCP tools, and error handling.
#
#   BGTUTOR_TOKEN=... ./smoke-test.sh [BASE_URL] [--write]
#
# BASE_URL defaults to https://bgtutor-mcp.f3s.buetow.org. It can also be a
# port-forward, e.g. http://127.0.0.1:8080 (just port-forward 8080).
# --write also tests save_vocabulary, which adds (or bumps) one entry with
# term "smoketest-ябълка" in the real vocabulary notebook.
#
# Needs curl and jq. Exits non-zero if any check fails.
set -uo pipefail

BASE_URL="https://bgtutor-mcp.f3s.buetow.org"
WRITE=0
for arg in "$@"; do
    case "$arg" in
        --write) WRITE=1 ;;
        *) BASE_URL="${arg%/}" ;;
    esac
done
: "${BGTUTOR_TOKEN:?set BGTUTOR_TOKEN to the token from secret bgtutor-secret}"
MCP="$BASE_URL/mcp"
PASS=0
FAIL=0

ok()   { PASS=$((PASS + 1)); printf 'ok    %s\n' "$1"; }
fail() { FAIL=$((FAIL + 1)); printf 'FAIL  %s\n' "$1"; [ -n "${2:-}" ] && printf '      %s\n' "$2"; }
check() { if [ "$2" = "$3" ]; then ok "$1"; else fail "$1" "got '$2', want '$3'"; fi; }

# rpc_status METHOD PARAMS_JSON [curl auth args...] -> prints the HTTP status
rpc_status() {
    local method=$1 params=$2; shift 2
    curl -s -o /dev/null -w '%{http_code}' -X POST "$@" \
        -H 'Content-Type: application/json' -H 'Accept: application/json, text/event-stream' \
        -d "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"$method\",\"params\":$params}"
}

# rpc METHOD PARAMS_JSON -> prints the JSON-RPC response, authenticated by header
rpc() {
    curl -s -X POST "$MCP" -H "Authorization: Bearer $BGTUTOR_TOKEN" \
        -H 'Content-Type: application/json' -H 'Accept: application/json, text/event-stream' \
        -d "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"$1\",\"params\":$2}"
}

# call TOOL ARGS_JSON -> prints the tool result (structuredContent, isError, text)
call() { rpc tools/call "{\"name\":\"$1\",\"arguments\":$2}" | jq -c '.result'; }

LIST='{"name":"list_episodes","arguments":{}}'
echo "== bgtutor smoke test against $BASE_URL"

echo "-- authentication"
check "healthz is open"                     "$(curl -s -o /dev/null -w '%{http_code}' "$BASE_URL/healthz")" 200
check "no token -> 401"                     "$(rpc_status tools/call "$LIST" "$MCP")" 401
check "wrong bearer token -> 401"           "$(rpc_status tools/call "$LIST" "$MCP" -H 'Authorization: Bearer wrong')" 401
check "empty bearer token -> 401"           "$(rpc_status tools/call "$LIST" "$MCP" -H 'Authorization: Bearer ')" 401
check "token as basic auth -> 401"          "$(rpc_status tools/call "$LIST" "$MCP" -u "x:$BGTUTOR_TOKEN")" 401
check "wrong ?token= -> 401"                "$(rpc_status tools/call "$LIST" "$MCP?token=wrong")" 401
check "token prefix only -> 401"            "$(rpc_status tools/call "$LIST" "$MCP" -H "Authorization: Bearer ${BGTUTOR_TOKEN:0:8}")" 401
check "GET /mcp without token -> 401"       "$(curl -s -o /dev/null -w '%{http_code}' "$MCP")" 401
check "correct bearer token -> 200"         "$(rpc_status tools/call "$LIST" "$MCP" -H "Authorization: Bearer $BGTUTOR_TOKEN")" 200
check "lowercase 'bearer' scheme -> 200"    "$(rpc_status tools/call "$LIST" "$MCP" -H "authorization: bearer $BGTUTOR_TOKEN")" 200
check "correct ?token= -> 200"              "$(rpc_status tools/call "$LIST" "$MCP?token=$BGTUTOR_TOKEN")" 200
www=$(curl -s -D - -o /dev/null -X POST "$MCP" | tr -d '\r' | grep -i '^www-authenticate:' | cut -d' ' -f2)
check "401 carries WWW-Authenticate: Bearer" "$www" Bearer
if [ "$(rpc_status tools/call "$LIST" "$MCP" -H "Authorization: Bearer $BGTUTOR_TOKEN")" != 200 ]; then
    echo "== the server rejects BGTUTOR_TOKEN; stopping ($PASS passed, $FAIL failed)"
    exit 1
fi

echo "-- MCP protocol"
init=$(rpc initialize '{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"smoke-test","version":"1"}}')
check "initialize returns server name"      "$(jq -r '.result.serverInfo.name' <<<"$init")" bulgarian-podcast-tutor
check "instructions mention get_paragraph"  "$(jq -r '.result.instructions | contains("get_paragraph")' <<<"$init")" true
tools=$(rpc tools/list '{}' | jq -r '[.result.tools[].name] | sort | join(",")')
check "tools/list has the four tools"       "$tools" get_paragraph,list_episodes,list_vocabulary,save_vocabulary

echo "-- episodes"
eps=$(call list_episodes '{}')
EPISODE=$(jq -r '[.structuredContent.episodes[] | select(.ready)][0].episode_id // empty' <<<"$eps")
TOTAL=$(jq -r '[.structuredContent.episodes[] | select(.ready)][0].paragraph_count // 0' <<<"$eps")
if [ -z "$EPISODE" ]; then
    fail "at least one ready episode" "$(jq -c '.structuredContent' <<<"$eps")"
else
    ok "ready episode found: $EPISODE ($TOTAL paragraphs)"
    p1=$(call get_paragraph "{\"episode_id\":\"$EPISODE\",\"index\":1}")
    check "paragraph 1 position"            "$(jq -r '.structuredContent.position' <<<"$p1")" "1 of $TOTAL"
    check "paragraph 1 has english"         "$(jq -r '.structuredContent.english | length > 0' <<<"$p1")" true
    check "paragraph 1 reference note"      "$(jq -r '.structuredContent.bulgarian_reference_note | startswith("Reference translation")' <<<"$p1")" true
    pn=$(call get_paragraph "{\"episode_id\":\"$EPISODE\",\"index\":$TOTAL}")
    check "last paragraph is_last"          "$(jq -r '.structuredContent.is_last' <<<"$pn")" true
    check "last paragraph next_index null"  "$(jq -r '.structuredContent.next_index' <<<"$pn")" null
    out=$(call get_paragraph "{\"episode_id\":\"$EPISODE\",\"index\":$((TOTAL + 1))}")
    check "index past the end is a tool error" "$(jq -r '.isError' <<<"$out")" true
    check "error names the valid range"     "$(jq -r '.content[0].text | contains("Valid indexes are 1 to '"$TOTAL"'")' <<<"$out")" true
fi
unknown=$(call get_paragraph '{"episode_id":"999-does-not-exist","index":1}')
check "unknown episode is a tool error"     "$(jq -r '.isError' <<<"$unknown")" true
trav=$(call get_paragraph '{"episode_id":"../../etc","index":1}')
check "path traversal id is rejected"       "$(jq -r '.content[0].text | startswith("Invalid episode id")' <<<"$trav")" true

echo "-- vocabulary"
check "list_vocabulary works"               "$(call list_vocabulary '{"limit":1}' | jq -r '.structuredContent.total_matching >= 0')" true
if [ "$WRITE" = 1 ]; then
    saved=$(call save_vocabulary "{\"term\":\"smoketest-ябълка\",\"translation\":\"apple\",\"note\":\"written by smoke-test.sh\",\"episode_id\":\"${EPISODE:-001-cooking-basics}\",\"paragraph_index\":1}")
    check "save_vocabulary stores the term" "$(jq -r '.structuredContent.status | test("saved")' <<<"$saved")" true
    found=$(call list_vocabulary '{"query":"smoketest"}' | jq -r '.structuredContent.total_matching')
    check "saved term is listed"            "$([ "${found:-0}" -ge 1 ] && echo yes)" yes
    bad=$(call save_vocabulary '{"term":"x","kind":"bogus"}')
    check "invalid kind is a tool error"    "$(jq -r '.isError' <<<"$bad")" true
else
    echo "skip  save_vocabulary (pass --write to test it; adds one test entry)"
fi

echo "== $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
