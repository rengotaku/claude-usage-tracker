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
| `CLAUDE_USAGE_TRACKER_PLAN_LIMIT` | プラン自動検出（下表参照） | 5h セッションのトークン上限 |
| `CLAUDE_USAGE_TRACKER_WEEKLY_LIMIT` | なし（0 表示） | 週次 全モデル合算上限 |
| `CLAUDE_USAGE_TRACKER_WEEKLY_SONNET_LIMIT` | なし（0 表示） | 週次 Sonnet 上限 |
| `CLAUDE_USAGE_TRACKER_DB` | `~/.local/share/claude-usage-tracker/snapshots.db` | SQLite DB パス |

## プラン自動検出

`CLAUDE_USAGE_TRACKER_PLAN_LIMIT` 未設定時は `~/.claude/.credentials.json` の `rateLimitTier` からデフォルト値を適用する。

| tier | セッション上限 (内蔵値) |
|---|---|
| `default_claude_pro` | 19M |
| `default_claude_max_5x` | 88M |
| `default_claude_max_20x` | 220M |

- 数値は Anthropic 非公開の概算値。プラン変更後は `claude login` での再認証が必要（[claude-code#43639](https://github.com/anthropics/claude-code/issues/43639)）。
- env 変数は常に優先される。
- 週次 limit はマップなし — env で明示指定する。
- 検出結果は stderr に JSON ログ（`plan detected`）として出力される。

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
