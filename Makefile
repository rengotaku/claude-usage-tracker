.PHONY: build test lint clean install uninstall status

build:
	CGO_ENABLED=0 go build ./...

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
