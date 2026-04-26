# claude-usage-tracker

Claudeのusage状態を定期的に記録するリポジトリ

## コマンド

| コマンド | 説明 |
|---|---|
| `claude-usage-tracker-setup` | `claude.ai/usage` のテキストを貼り付けて設定ファイルを生成 |
| `claude-usage-tracker-current` | 現在の使用率を表示（`--json` で JSON 出力） |
| `claude-usage-tracker-snapshot` | 使用率を計測して SQLite に保存 |

## セットアップ

### 1. インストール

```bash
make install
```

- バイナリを `~/.local/bin/` に配置
- 定期実行エージェントを有効化（毎時0分）

### 2. 設定ファイルの生成

```bash
claude-usage-tracker-setup
```

[claude.ai/usage](https://claude.ai/usage) のページテキストを貼り付けて Ctrl+D を押すと、
`~/.config/claude-usage-tracker/config.yaml` が自動生成されます。

## 設定ファイル

`~/.config/claude-usage-tracker/config.yaml`

```yaml
# weekly リセット曜日・時刻（claude-usage-tracker-setup で自動設定）
weekly_reset_day: Thursday
weekly_reset_hour: 7

# 以下は任意設定
# log_dir: ~/.claude/projects
# db: ~/.local/share/claude-usage-tracker/snapshots.db
# plan_limit: 0          # 手動オーバーライド（0 = 自動検出）
# weekly_limit: 0        # 週次全モデル上限（0 = 自動検出）
# weekly_sonnet_limit: 0 # 週次 Sonnet 上限（0 = 自動検出）
# port: "8080"           # HTTP サーバーポート
```

設定ファイルパスは `CLAUDE_USAGE_TRACKER_CONFIG` 環境変数でオーバーライド可能。

## プラン自動検出

`plan_limit` 未設定時は `~/.claude/.credentials.json` の `rateLimitTier` からデフォルト値を適用する。

| tier | セッション (5h) | 週次 All | 週次 Sonnet |
|---|---|---|---|
| `default_claude_pro` | 19M | — | — |
| `default_claude_max_5x` | 90M | 833M | 695M |
| `default_claude_max_20x` | 220M | — | — |

- 値は web `/usage` の `%` 表示から実測（概算）。
- プラン変更後は `claude login` での再認証が必要（[claude-code#43639](https://github.com/anthropics/claude-code/issues/43639)）。
- 検出結果は stderr に JSON ログ（`plan detected`）として出力される。

### Team / Enterprise プランの場合

`~/.claude/.credentials.json` に `rateLimitTier` が存在しないため自動検出が機能しない。
web の `/usage` ページで表示される `%` から上限を逆算し、`config.yaml` に明示指定する：

```yaml
# 例: web で「Session 16% / 9M used」と表示されている場合
# 上限 = 9,000,000 / 0.16 ≈ 56,000,000
plan_limit: 56000000
weekly_limit: 754000000
weekly_sonnet_limit: 367000000
```

## インストール詳細

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

## 週次レポート自動投稿

`scripts/report_usage.sh` が毎週月曜 09:00 に [Discussion #34](https://github.com/rengotaku/claude-usage-tracker/discussions/34) へ使用量レポートを自動投稿する。

**レポート内容**

- 実行ホスト (linux / mac)
- 現在のセッション使用率・トークン内訳
- 週次使用量 (All / Sonnet)
- 週次モデル別内訳 (Sonnet / Opus / Haiku)
- 直近ブロック一覧 (直近7日)
- 月の第1週のみ: 前月総括 (月間トータル / モデル別)

**依存コマンド**: `jq`, `gh` CLI, `sqlite3` (オプション)

### セットアップ — Linux (systemd timer)

```bash
# 前提: make install 済みで claude-usage-tracker-current が ~/.local/bin/ にある
bash scripts/systemd/install.sh

# 動作確認
claude-usage-report --dry-run

# ログ確認
journalctl --user -u claude-usage-report
```

`GH_TOKEN` を使う場合は `~/.config/claude-usage-report/env` に記述する:

```
GH_TOKEN=ghp_xxxx
```

### セットアップ — macOS (launchd)

```bash
bash scripts/launchd/install.sh

# 動作確認
claude-usage-report --dry-run

# ログ確認
tail -f ~/.local/share/claude-usage-tracker/report.log
```

### 手動実行

```bash
# 投稿せず stdout に出力
claude-usage-report --dry-run

# 前回と同一内容でも強制投稿
claude-usage-report --force
```

### 環境変数

| 変数 | デフォルト | 説明 |
|------|-----------|------|
| `CLAUDE_USAGE_TRACKER_DB` | `~/.local/share/claude-usage-tracker/snapshots.db` | SQLite DB パス |
| `CLAUDE_USAGE_TRACKER_LOG_DIR` | `~/.claude/projects` | JSONL ログディレクトリ |
| `GH_TOKEN` | (gh auth login を使用) | GitHub API トークン |
