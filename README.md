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
| `CLAUDE_USAGE_TRACKER_WEEKLY_LIMIT` | プラン自動検出（下表参照） | 週次 全モデル合算上限 |
| `CLAUDE_USAGE_TRACKER_WEEKLY_SONNET_LIMIT` | プラン自動検出（下表参照） | 週次 Sonnet 上限 |
| `CLAUDE_USAGE_TRACKER_DB` | `~/.local/share/claude-usage-tracker/snapshots.db` | SQLite DB パス |

## プラン自動検出

env 変数未設定時は `~/.claude/.credentials.json` の `rateLimitTier` からデフォルト値を適用する。

| tier | セッション (5h) | 週次 All | 週次 Sonnet |
|---|---|---|---|
| `default_claude_pro` | 19M | — | — |
| `default_claude_max_5x` | 45M | 833M | 695M |
| `default_claude_max_20x` | 220M | — | — |

- Max 5x の値は web `/usage` の `%` と照合済み (2026-04-22)。
- Pro / Max 20x のセッション値はコミュニティ実測で未検証、週次は未測定（env で明示指定する）。
- プラン変更後は `claude login` での再認証が必要（[claude-code#43639](https://github.com/anthropics/claude-code/issues/43639)）。
- env 変数は常に優先される。
- 検出結果は stderr に JSON ログ（`plan detected`）として出力される。

### Team / Enterprise プランの場合

Team・Enterprise プランは `claude auth login` によるブラウザ OAuth 認証（SSO）を使用するため、`~/.claude/.credentials.json` に `rateLimitTier` が存在しない。そのため自動検出は機能せず、`session_ratio` が常に `0` になる。

web の `/usage` ページで表示される `%` から上限を逆算し、env 変数で明示指定する：

```bash
# 例: web で「Session 16% / 9M used」と表示されている場合
# 上限 = 9,000,000 / 0.16 ≈ 56,000,000
export CLAUDE_USAGE_TRACKER_PLAN_LIMIT=56000000
export CLAUDE_USAGE_TRACKER_WEEKLY_LIMIT=754000000
export CLAUDE_USAGE_TRACKER_WEEKLY_SONNET_LIMIT=367000000
```

macOS（launchd）では `~/Library/LaunchAgents/com.user.claude-usage-tracker.plist` の `EnvironmentVariables` に追記する。

## インストール

`make install` を実行すると OS を自動判別してインストールする。

```bash
make install
```

- バイナリを `~/.local/bin/` に配置
- 定期実行エージェントを有効化（毎時0分）

### Linux（systemd timer）

```bash
make install   # systemd user timer を有効化
make status    # systemctl --user list-timers
make uninstall
```

### macOS（launchd）

```bash
make install   # ~/Library/LaunchAgents/ に plist を配置して launchctl load
make status    # launchctl list com.user.claude-usage-tracker
make uninstall
```

- ログ: `~/.local/share/claude-usage-tracker/launchd.log`
- エラーログ: `~/.local/share/claude-usage-tracker/launchd.error.log`
