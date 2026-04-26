#!/bin/zsh
# Weekly Claude usage report — posts to Discussion #34
#
# Usage:
#   ./scripts/report_usage.sh [--dry-run] [--force]
#
#   --dry-run  Print report to stdout; skip posting
#   --force    Post even if report is identical to last post

CURRENT_BIN="${HOME}/.local/bin/claude-usage-tracker-current"
DB_PATH="${CLAUDE_USAGE_TRACKER_DB:-${HOME}/.local/share/claude-usage-tracker/snapshots.db}"
LOG_DIR="${CLAUDE_USAGE_TRACKER_LOG_DIR:-${HOME}/.claude/projects}"
DATA_DIR="${HOME}/.local/share/claude-usage-tracker"
LAST_REPORT_FILE="${DATA_DIR}/last-usage-report.txt"
DISCUSSION_NUM=34
REPO_OWNER="rengotaku"
REPO_NAME="claude-usage-tracker"

DRY_RUN=false
FORCE=false
for arg in "$@"; do
  case "$arg" in
    --dry-run) DRY_RUN=true ;;
    --force)   FORCE=true  ;;
  esac
done

# ── Helpers ────────────────────────────────────────────────────

die() { echo "Error: $*" >&2; exit 1 }

# Format token count as human-readable (k / M)
fmt_tok() {
  local n=$1
  if (( n >= 1000000 )); then
    awk "BEGIN { printf \"%.0fM\", $n / 1000000 }"
  elif (( n >= 1000 )); then
    awk "BEGIN { printf \"%.0fk\", $n / 1000 }"
  else
    printf "%d" "$n"
  fi
}

# Format float ratio as integer percentage
fmt_pct() { awk "BEGIN { printf \"%.0f%%\", $1 * 100 }" }

# ── Dependency checks ──────────────────────────────────────────

[[ -x "$CURRENT_BIN" ]] \
  || die "$CURRENT_BIN not found — run 'make install' first"
command -v jq >/dev/null 2>&1 || die "jq is required"
command -v gh  >/dev/null 2>&1 || die "gh CLI is required"

# ── Detect OS ─────────────────────────────────────────────────

OS_TYPE=linux
[[ "$(uname)" == Darwin ]] && OS_TYPE=mac

# ── Get current session/weekly usage ──────────────────────────

USAGE_JSON=$("$CURRENT_BIN" --json --no-cache 2>/dev/null) \
  || die "Failed to run $CURRENT_BIN"

jv() { printf '%s' "$USAGE_JSON" | jq -r "$1" }

SESSION_TOKENS=$(jv '.session.tokens_used')
SESSION_LIMIT=$(jv '.session.limit // 0')
SESSION_RATIO=$(jv '.session.ratio // 0')
SESSION_ENDS=$(jv '.session.ends_at // ""')
S_INPUT=$(jv '.session.breakdown.input')
S_OUTPUT=$(jv '.session.breakdown.output')
S_CACHE_CR=$(jv '.session.breakdown.cache_creation')
S_CACHE_RD=$(jv '.session.breakdown.cache_read')

WEEKLY_TOKENS=$(jv '.weekly.tokens_used')
WEEKLY_LIMIT=$(jv '.weekly.limit // 0')
WEEKLY_RATIO=$(jv '.weekly.ratio // 0')
WEEKLY_SONNET=$(jv '.weekly.sonnet_tokens_used')
WEEKLY_SONNET_LIMIT=$(jv '.weekly.sonnet_limit // 0')
WEEKLY_SONNET_RATIO=$(jv '.weekly.sonnet_ratio // 0')
WEEKLY_RESETS=$(jv '.weekly.resets_at')

# ── Recent blocks from SQLite ──────────────────────────────────

BLOCKS_SECTION=""
if command -v sqlite3 >/dev/null 2>&1 && [[ -f "$DB_PATH" ]]; then
  BLOCKS_DATA=$(sqlite3 -separator "|" "$DB_PATH" \
    "SELECT datetime(block_started_at,'+9 hours'), \
            COALESCE(datetime(MAX(block_ended_at),'+9 hours'),'進行中'), \
            MAX(tokens_used)
     FROM snapshots
     WHERE block_started_at >= datetime('now','-7 days')
     GROUP BY block_started_at
     ORDER BY block_started_at DESC
     LIMIT 10;" 2>/dev/null || true)

  if [[ -n "$BLOCKS_DATA" ]]; then
    BLOCKS_SECTION="### 直近ブロック一覧 (直近7日)

| 開始 (JST) | 終了 (JST) | トークン |
|------------|------------|----------|
"
    while IFS="|" read -r started ended tokens; do
      BLOCKS_SECTION+="| ${started} | ${ended} | $(fmt_tok ${tokens}) |
"
    done <<< "$BLOCKS_DATA"
    BLOCKS_SECTION+="
"
  fi
fi

# ── Model breakdown from JSONL (weekly) ───────────────────────

MODEL_SECTION=""
typeset -a jsonl_files
jsonl_files=("${LOG_DIR}"/**/*.jsonl(N))

if (( ${#jsonl_files} > 0 )); then
  # Approximate weekly start: 7 days ago (actual config-based reset used for Sonnet/All above)
  WEEK_AGO=$(date -d "-7 days" '+%Y-%m-%dT%H:%M:%S' 2>/dev/null \
             || date -v-7d '+%Y-%m-%dT%H:%M:%S')

  MODEL_JSON=$(cat "${jsonl_files[@]}" | \
    jq -c 'select(.type == "assistant" and .message != null and .message.usage != null)' 2>/dev/null | \
    jq -s --arg since "$WEEK_AGO" '
      map(select(.timestamp >= $since))
      | group_by(
          if .message.model == null then "Other"
          elif (.message.model | ascii_downcase | test("sonnet")) then "Sonnet"
          elif (.message.model | ascii_downcase | test("opus"))   then "Opus"
          elif (.message.model | ascii_downcase | test("haiku"))  then "Haiku"
          else "Other"
          end)
      | map({
          model: (.[0] | if .message.model == null then "Other"
                          elif (.message.model | ascii_downcase | test("sonnet")) then "Sonnet"
                          elif (.message.model | ascii_downcase | test("opus"))   then "Opus"
                          elif (.message.model | ascii_downcase | test("haiku"))  then "Haiku"
                          else "Other" end),
          tokens: (map(.message.usage.input_tokens
                       + .message.usage.output_tokens
                       + (.message.usage.cache_creation_input_tokens // 0)
                       + (.message.usage.cache_read_input_tokens // 0))
                   | add // 0)
        })
      | sort_by(-.tokens)
    ' 2>/dev/null || echo '[]')

  if [[ "$MODEL_JSON" != "[]" && "$MODEL_JSON" != "null" ]]; then
    MODEL_SECTION="**週次モデル別内訳**

| モデル | トークン |
|--------|----------|
"
    while IFS= read -r row; do
      model=$(printf '%s' "$row" | jq -r '.model')
      tokens=$(printf '%s' "$row" | jq -r '.tokens')
      MODEL_SECTION+="| ${model} | $(fmt_tok ${tokens}) |
"
    done < <(printf '%s' "$MODEL_JSON" | jq -c '.[]')
    MODEL_SECTION+="
"
  fi
fi

# ── Previous month summary (first week of month only) ─────────

MONTHLY_SECTION=""
NOW_DAY=$(date +%d)
if (( 10#$NOW_DAY <= 7 )) && (( ${#jsonl_files} > 0 )); then
  # Previous month date range
  PREV_START=$(date -d "$(date +%Y-%m-01) -1 month" '+%Y-%m-01' 2>/dev/null \
               || date -v-1m -v1d '+%Y-%m-01')
  PREV_END=$(date '+%Y-%m-01')
  PREV_LABEL="${PREV_START:0:7}"

  MONTHLY_DATA=$(cat "${jsonl_files[@]}" | \
    jq -c 'select(.type == "assistant" and .message != null and .message.usage != null)' 2>/dev/null | \
    jq -s --arg start "${PREV_START}T00:00:00" --arg end "${PREV_END}T00:00:00" '
      map(select(.timestamp >= $start and .timestamp < $end))
      | {
          total:  (map(.message.usage.input_tokens + .message.usage.output_tokens + (.message.usage.cache_creation_input_tokens // 0) + (.message.usage.cache_read_input_tokens // 0)) | add // 0),
          sonnet: (map(select(.message.model != null and (.message.model | ascii_downcase | test("sonnet"))) | .message.usage.input_tokens + .message.usage.output_tokens + (.message.usage.cache_creation_input_tokens // 0) + (.message.usage.cache_read_input_tokens // 0)) | add // 0),
          opus:   (map(select(.message.model != null and (.message.model | ascii_downcase | test("opus")))   | .message.usage.input_tokens + .message.usage.output_tokens + (.message.usage.cache_creation_input_tokens // 0) + (.message.usage.cache_read_input_tokens // 0)) | add // 0),
          haiku:  (map(select(.message.model != null and (.message.model | ascii_downcase | test("haiku")))  | .message.usage.input_tokens + .message.usage.output_tokens + (.message.usage.cache_creation_input_tokens // 0) + (.message.usage.cache_read_input_tokens // 0)) | add // 0)
        }
    ' 2>/dev/null || echo '{"total":0,"sonnet":0,"opus":0,"haiku":0}')

  M_TOTAL=$(printf '%s' "$MONTHLY_DATA" | jq -r '.total')
  M_SONNET=$(printf '%s' "$MONTHLY_DATA" | jq -r '.sonnet')
  M_OPUS=$(printf '%s' "$MONTHLY_DATA"   | jq -r '.opus')
  M_HAIKU=$(printf '%s' "$MONTHLY_DATA"  | jq -r '.haiku')

  MONTHLY_SECTION="### 前月総括 (${PREV_LABEL})

**月間トータル**: $(fmt_tok $M_TOTAL)

**モデル別内訳**

| モデル | トークン |
|--------|----------|
| Sonnet | $(fmt_tok $M_SONNET) |
| Opus   | $(fmt_tok $M_OPUS) |
| Haiku  | $(fmt_tok $M_HAIKU) |

"
fi

# ── Build report body ─────────────────────────────────────────

NOW_STR=$(date '+%Y-%m-%d %H:%M %Z')

# Session line
if (( SESSION_LIMIT > 0 )); then
  SESSION_LINE="$(fmt_pct $SESSION_RATIO) ($(fmt_tok $SESSION_TOKENS) / $(fmt_tok $SESSION_LIMIT))"
else
  SESSION_LINE="$(fmt_tok $SESSION_TOKENS)"
fi
[[ -n "$SESSION_ENDS" ]] && SESSION_LINE+=" — ブロック終了: ${SESSION_ENDS}"

# Weekly lines
if (( WEEKLY_LIMIT > 0 )); then
  WEEKLY_ALL_LINE="$(fmt_pct $WEEKLY_RATIO) ($(fmt_tok $WEEKLY_TOKENS) / $(fmt_tok $WEEKLY_LIMIT))"
else
  WEEKLY_ALL_LINE="$(fmt_tok $WEEKLY_TOKENS)"
fi
if (( WEEKLY_SONNET_LIMIT > 0 )); then
  WEEKLY_SONNET_LINE="$(fmt_pct $WEEKLY_SONNET_RATIO) ($(fmt_tok $WEEKLY_SONNET) / $(fmt_tok $WEEKLY_SONNET_LIMIT))"
else
  WEEKLY_SONNET_LINE="$(fmt_tok $WEEKLY_SONNET)"
fi

REPORT="## 週次レポート — ${NOW_STR}

**実行ホスト**: ${OS_TYPE}

### セッション (現在の5h ブロック)

- 使用率: ${SESSION_LINE}

**トークン内訳**

| input | output | cache_creation | cache_read |
|-------|--------|----------------|------------|
| $(fmt_tok $S_INPUT) | $(fmt_tok $S_OUTPUT) | $(fmt_tok $S_CACHE_CR) | $(fmt_tok $S_CACHE_RD) |

### 週次使用量

- All: ${WEEKLY_ALL_LINE}
- Sonnet: ${WEEKLY_SONNET_LINE}
- リセット: ${WEEKLY_RESETS}

${MODEL_SECTION}${BLOCKS_SECTION}${MONTHLY_SECTION}"

# ── Duplicate check ───────────────────────────────────────────

mkdir -p "$DATA_DIR"
if [[ -f "$LAST_REPORT_FILE" ]] && ! $FORCE && ! $DRY_RUN; then
  if [[ "$(cat "$LAST_REPORT_FILE")" == "$REPORT" ]]; then
    echo "Report unchanged since last post. Use --force to override."
    exit 0
  fi
fi

# ── Dry-run: print and exit ───────────────────────────────────

if $DRY_RUN; then
  printf '%s\n' "$REPORT"
  echo
  echo "[dry-run] コメント投稿をスキップしました"
  exit 0
fi

# ── Post to Discussion #DISCUSSION_NUM ────────────────────────

DISCUSSION_ID=$(gh api graphql \
  -f query="query {
    repository(owner: \"${REPO_OWNER}\", name: \"${REPO_NAME}\") {
      discussion(number: ${DISCUSSION_NUM}) { id }
    }
  }" \
  --jq '.data.repository.discussion.id') \
  || die "Failed to query Discussion #${DISCUSSION_NUM}"

[[ -n "$DISCUSSION_ID" && "$DISCUSSION_ID" != "null" ]] \
  || die "Discussion #${DISCUSSION_NUM} not found in ${REPO_OWNER}/${REPO_NAME}"

COMMENT_URL=$(gh api graphql \
  -f query='mutation($id: ID!, $body: String!) {
    addDiscussionComment(input: {discussionId: $id, body: $body}) {
      comment { url }
    }
  }' \
  -f id="$DISCUSSION_ID" \
  -f body="$REPORT" \
  --jq '.data.addDiscussionComment.comment.url') \
  || die "Failed to post comment to Discussion #${DISCUSSION_NUM}"

printf '%s\n' "$REPORT" > "$LAST_REPORT_FILE"
echo "Posted: ${COMMENT_URL}"
