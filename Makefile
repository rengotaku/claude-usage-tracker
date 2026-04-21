.PHONY: build test lint clean install uninstall status db server

build:
	CGO_ENABLED=0 go build ./...

server:
	CGO_ENABLED=0 go run ./cmd/server

test:
	CGO_ENABLED=0 go test ./...

lint:
	golangci-lint run ./...

clean:
	go clean ./...

install:
	bash deploy/systemd/install.sh

uninstall:
	systemctl --user disable --now claude-usage-tracker.timer || true
	rm -f ~/.config/systemd/user/claude-usage-tracker.{service,timer}
	rm -f ~/.local/bin/claude-usage-tracker-{snapshot,current}
	systemctl --user daemon-reload

status:
	systemctl --user status claude-usage-tracker.timer claude-usage-tracker.service || true

DB_PATH ?= $(HOME)/.local/share/claude-usage-tracker/snapshots.db
db:
	sqlite3 $(DB_PATH) "SELECT datetime(taken_at,'+9 hours')||'+09:00' AS taken_at_jst, tokens_used, usage_ratio, weekly_tokens, weekly_sonnet_tokens FROM snapshots ORDER BY taken_at DESC LIMIT 10;"
