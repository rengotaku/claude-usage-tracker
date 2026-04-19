# claude-usage-tracker

Claudeのusage状態を定期的に記録するリポジトリ

## コマンド

| コマンド | 説明 |
|---|---|
| `claude-usage-tracker-current` | 現在の使用率を表示（`--json` で JSON 出力） |
| `claude-usage-tracker-snapshot` | 使用率を計測して SQLite に保存 |

## 環境変数

| 変数 | デフォルト | 説明 |
|---|---|---|
| `CLAUDE_USAGE_TRACKER_LOG_DIR` | `~/.claude/projects` | JSONL ログディレクトリ |
| `CLAUDE_USAGE_TRACKER_PLAN_LIMIT` | `200000000` | プランのトークン上限 |
| `CLAUDE_USAGE_TRACKER_DB` | `~/.local/share/claude-usage-tracker/snapshots.db` | SQLite DB パス |

## インストール（systemd timer）

```bash
make install
```

- バイナリを `~/.local/bin/` に配置
- user-level systemd timer を有効化（毎時実行）

### 確認

```bash
systemctl --user list-timers
# または
make status
```

### アンインストール

```bash
make uninstall
```
