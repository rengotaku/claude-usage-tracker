#!/usr/bin/env python3
"""Weekly Claude usage report — posts to Discussion #34.

Usage:
    ./scripts/report_usage.py [--dry-run] [--force]

    --dry-run  Print report to stdout; skip posting
    --force    Post even if report is identical to last post
"""

import argparse
import glob
import json
import os
import platform
import sqlite3
import subprocess
import sys
from datetime import datetime, timedelta, timezone
from pathlib import Path


# ── Configuration ─────────────────────────────────────────────

CURRENT_BIN = Path.home() / ".local/bin/claude-usage-tracker-current"
DB_PATH = Path(os.environ.get(
    "CLAUDE_USAGE_TRACKER_DB",
    Path.home() / ".local/share/claude-usage-tracker/snapshots.db",
))
LOG_DIR = Path(os.environ.get(
    "CLAUDE_USAGE_TRACKER_LOG_DIR",
    Path.home() / ".claude/projects",
))
DATA_DIR = Path.home() / ".local/share/claude-usage-tracker"
LAST_REPORT_FILE = DATA_DIR / "last-usage-report.txt"
DISCUSSION_NUM = 34
REPO_OWNER = "rengotaku"
REPO_NAME = "claude-usage-tracker"

JST = timezone(timedelta(hours=9))


# ── Helpers ───────────────────────────────────────────────────

def die(msg: str) -> None:
    print(f"Error: {msg}", file=sys.stderr)
    sys.exit(1)


def fmt_tok(n: int) -> str:
    if n >= 1_000_000:
        return f"{n / 1_000_000:.0f}M"
    if n >= 1_000:
        return f"{n / 1_000:.0f}k"
    return str(n)


def fmt_pct(ratio: float) -> str:
    return f"{ratio * 100:.0f}%"


def classify_model(model: str | None) -> str:
    if not model:
        return "Other"
    m = model.lower()
    if "sonnet" in m:
        return "Sonnet"
    if "opus" in m:
        return "Opus"
    if "haiku" in m:
        return "Haiku"
    return "Other"


def gh(*args: str) -> str:
    result = subprocess.run(["gh", *args], capture_output=True, text=True)
    if result.returncode != 0:
        die(f"gh {' '.join(args[:3])} failed: {result.stderr.strip()}")
    return result.stdout.strip()


# ── Dependency checks ─────────────────────────────────────────

def check_deps() -> None:
    if not CURRENT_BIN.is_file() or not os.access(CURRENT_BIN, os.X_OK):
        die(f"{CURRENT_BIN} not found — run 'make install' first")
    for cmd in ("gh",):
        if subprocess.run(["which", cmd], capture_output=True).returncode != 0:
            die(f"{cmd} is required but not found")


# ── Current usage (session + weekly) ─────────────────────────

def get_usage() -> dict:
    result = subprocess.run(
        [str(CURRENT_BIN), "--json", "--no-cache"],
        capture_output=True, text=True,
    )
    if result.returncode != 0:
        die(f"Failed to run {CURRENT_BIN}: {result.stderr.strip()}")
    return json.loads(result.stdout)


# ── Recent blocks from SQLite ─────────────────────────────────

def get_recent_blocks() -> list[dict]:
    if not DB_PATH.exists():
        return []
    try:
        con = sqlite3.connect(str(DB_PATH))
        cur = con.execute("""
            SELECT
              datetime(block_started_at, '+9 hours'),
              COALESCE(datetime(MAX(block_ended_at), '+9 hours'), '進行中'),
              MAX(tokens_used)
            FROM snapshots
            WHERE block_started_at >= datetime('now', '-7 days')
            GROUP BY block_started_at
            ORDER BY block_started_at DESC
            LIMIT 10
        """)
        rows = [{"started": r[0], "ended": r[1], "tokens": r[2]} for r in cur.fetchall()]
        con.close()
        return rows
    except Exception:
        return []


# ── JSONL parsing ─────────────────────────────────────────────

def parse_jsonl_entries(since: datetime | None = None, until: datetime | None = None) -> list[dict]:
    """Return assistant entries from all JSONL files, optionally filtered by time range."""
    entries = []
    seen: set[str] = set()

    for path in glob.glob(str(LOG_DIR / "**/*.jsonl"), recursive=True):
        try:
            with open(path, encoding="utf-8") as f:
                for line in f:
                    line = line.strip()
                    if not line:
                        continue
                    try:
                        obj = json.loads(line)
                    except json.JSONDecodeError:
                        continue
                    if obj.get("type") != "assistant":
                        continue
                    msg = obj.get("message")
                    if not msg or not msg.get("usage"):
                        continue
                    uid = obj.get("uuid") or obj.get("message", {}).get("id", "")
                    if uid and uid in seen:
                        continue
                    if uid:
                        seen.add(uid)
                    ts_raw = obj.get("timestamp", "")
                    try:
                        ts = datetime.fromisoformat(ts_raw.replace("Z", "+00:00"))
                    except ValueError:
                        continue
                    if since and ts < since:
                        continue
                    if until and ts >= until:
                        continue
                    entries.append({"ts": ts, "model": msg.get("model"), "usage": msg["usage"]})
        except OSError:
            continue

    return entries


def sum_tokens(entries: list[dict]) -> int:
    total = 0
    for e in entries:
        u = e["usage"]
        total += (u.get("input_tokens", 0)
                  + u.get("output_tokens", 0)
                  + u.get("cache_creation_input_tokens", 0)
                  + u.get("cache_read_input_tokens", 0))
    return total


def model_breakdown(entries: list[dict]) -> list[dict]:
    buckets: dict[str, int] = {}
    for e in entries:
        label = classify_model(e["model"])
        u = e["usage"]
        buckets[label] = buckets.get(label, 0) + (
            u.get("input_tokens", 0)
            + u.get("output_tokens", 0)
            + u.get("cache_creation_input_tokens", 0)
            + u.get("cache_read_input_tokens", 0)
        )
    return sorted(
        [{"model": k, "tokens": v} for k, v in buckets.items()],
        key=lambda x: -x["tokens"],
    )


# ── Report sections ───────────────────────────────────────────

def section_session(usage: dict) -> str:
    s = usage["session"]
    tokens = s["tokens_used"]
    limit = s.get("limit") or 0
    ratio = s.get("ratio") or 0.0
    ends = s.get("ends_at") or ""
    bd = s.get("breakdown", {})

    if limit:
        usage_line = f"{fmt_pct(ratio)} ({fmt_tok(tokens)} / {fmt_tok(limit)})"
    else:
        usage_line = fmt_tok(tokens)
    if ends:
        usage_line += f" — ブロック終了: {ends}"

    return f"""### セッション (現在の5h ブロック)

- 使用率: {usage_line}

**トークン内訳**

| input | output | cache_creation | cache_read |
|-------|--------|----------------|------------|
| {fmt_tok(bd.get('input', 0))} | {fmt_tok(bd.get('output', 0))} | {fmt_tok(bd.get('cache_creation', 0))} | {fmt_tok(bd.get('cache_read', 0))} |
"""


def section_weekly(usage: dict, model_rows: list[dict]) -> str:
    w = usage["weekly"]
    tokens = w["tokens_used"]
    limit = w.get("limit") or 0
    ratio = w.get("ratio") or 0.0
    sonnet = w["sonnet_tokens_used"]
    sonnet_limit = w.get("sonnet_limit") or 0
    sonnet_ratio = w.get("sonnet_ratio") or 0.0
    resets = w.get("resets_at", "")

    all_line = (f"{fmt_pct(ratio)} ({fmt_tok(tokens)} / {fmt_tok(limit)})"
                if limit else fmt_tok(tokens))
    sonnet_line = (f"{fmt_pct(sonnet_ratio)} ({fmt_tok(sonnet)} / {fmt_tok(sonnet_limit)})"
                   if sonnet_limit else fmt_tok(sonnet))

    lines = [
        "### 週次使用量",
        "",
        f"- All: {all_line}",
        f"- Sonnet: {sonnet_line}",
        f"- リセット: {resets}",
        "",
    ]

    if model_rows:
        lines += [
            "**週次モデル別内訳**",
            "",
            "| モデル | トークン |",
            "|--------|----------|",
        ]
        for row in model_rows:
            lines.append(f"| {row['model']} | {fmt_tok(row['tokens'])} |")
        lines.append("")

    return "\n".join(lines)


def section_blocks(blocks: list[dict]) -> str:
    if not blocks:
        return ""
    rows = ["### 直近ブロック一覧 (直近7日)", "",
            "| 開始 (JST) | 終了 (JST) | トークン |",
            "|------------|------------|----------|"]
    for b in blocks:
        rows.append(f"| {b['started']} | {b['ended']} | {fmt_tok(b['tokens'])} |")
    rows.append("")
    return "\n".join(rows)


def section_monthly(prev_start: datetime, prev_end: datetime) -> str:
    entries = parse_jsonl_entries(
        since=prev_start.astimezone(timezone.utc),
        until=prev_end.astimezone(timezone.utc),
    )
    total = sum_tokens(entries)
    rows = model_breakdown(entries)
    label = prev_start.strftime("%Y-%m")

    model_table = "\n".join(
        [f"| {r['model']} | {fmt_tok(r['tokens'])} |" for r in rows]
    ) if rows else "| (データなし) | — |"

    return f"""### 前月総括 ({label})

**月間トータル**: {fmt_tok(total)}

**モデル別内訳**

| モデル | トークン |
|--------|----------|
{model_table}

"""


# ── GitHub Discussion posting ─────────────────────────────────

def get_discussion_id() -> str:
    query = (
        f'query {{ repository(owner: "{REPO_OWNER}", name: "{REPO_NAME}") '
        f'{{ discussion(number: {DISCUSSION_NUM}) {{ id }} }} }}'
    )
    raw = gh("api", "graphql", "-f", f"query={query}", "--jq",
             ".data.repository.discussion.id")
    if not raw or raw == "null":
        die(f"Discussion #{DISCUSSION_NUM} not found in {REPO_OWNER}/{REPO_NAME}")
    return raw


def post_comment(discussion_id: str, body: str) -> str:
    mutation = (
        "mutation($id: ID!, $body: String!) { "
        "addDiscussionComment(input: {discussionId: $id, body: $body}) "
        "{ comment { url } } }"
    )
    return gh("api", "graphql",
              "-f", f"query={mutation}",
              "-f", f"id={discussion_id}",
              "-f", f"body={body}",
              "--jq", ".data.addDiscussionComment.comment.url")


# ── Main ──────────────────────────────────────────────────────

def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__,
                                     formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--dry-run", action="store_true",
                        help="Print to stdout; skip posting")
    parser.add_argument("--force", action="store_true",
                        help="Post even if identical to last report")
    args = parser.parse_args()

    check_deps()

    os_type = "mac" if platform.system() == "Darwin" else "linux"
    usage = get_usage()

    # Weekly model breakdown from JSONL (approximate: past 7 days)
    week_ago = datetime.now(timezone.utc) - timedelta(days=7)
    weekly_entries = parse_jsonl_entries(since=week_ago)
    model_rows = model_breakdown(weekly_entries)

    blocks = get_recent_blocks()

    # Monthly summary only on first week of the month
    now = datetime.now()
    monthly_section = ""
    if now.day <= 7:
        first_of_month = now.replace(day=1, hour=0, minute=0, second=0, microsecond=0)
        prev_end = first_of_month
        # Previous month first day
        if first_of_month.month == 1:
            prev_start = first_of_month.replace(year=first_of_month.year - 1, month=12)
        else:
            prev_start = first_of_month.replace(month=first_of_month.month - 1)
        monthly_section = section_monthly(prev_start, prev_end)

    now_str = datetime.now(JST).strftime("%Y-%m-%d %H:%M JST")

    report = "\n".join([
        f"## 週次レポート — {now_str}",
        "",
        f"**実行ホスト**: {os_type}",
        "",
        section_session(usage),
        section_weekly(usage, model_rows),
        section_blocks(blocks),
        monthly_section,
    ]).rstrip() + "\n"

    # Duplicate check
    DATA_DIR.mkdir(parents=True, exist_ok=True)
    if not args.force and not args.dry_run and LAST_REPORT_FILE.exists():
        if LAST_REPORT_FILE.read_text() == report:
            print("Report unchanged since last post. Use --force to override.")
            return

    if args.dry_run:
        print(report)
        print("[dry-run] コメント投稿をスキップしました")
        return

    discussion_id = get_discussion_id()
    url = post_comment(discussion_id, report)
    LAST_REPORT_FILE.write_text(report)
    print(f"Posted: {url}")


if __name__ == "__main__":
    main()
