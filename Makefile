# 画面（web/dist）をビルドしてから、それを埋め込んだ Go の実行ファイルを作る。
palmterm: web/node_modules $(shell find web/src -type f) web/index.html *.go
	cd web && npm run build
	go build -o palmterm .

web/node_modules: web/package.json
	cd web && npm install
	touch web/node_modules

.PHONY: dev
# 開発用：Go のサーバーを 7681 で動かし、Vite（5173）から中継する。
dev: palmterm
	./palmterm -allow-origin 'localhost:5173,*:5173' & cd web && npm run dev

.PHONY: test
# テスト：Go（tmux はテスト専用のサーバーを使う）、画面の単体テスト、実際のブラウザでの通しのテスト。
# 通しのテストは chromium を使う（場所は CHROMIUM で変えられる。既定は /usr/bin/chromium）。
test: palmterm
	go test ./...
	cd web && npm test && npm run test:e2e

BINDIR ?= $(HOME)/.local/bin
UNITDIR ?= $(HOME)/.config/systemd/user

.PHONY: install uninstall
# ~/.local/bin に実行ファイルを、~/.config/systemd/user にユーザーサービスを置く（有効化・起動はしない）。
install: palmterm
	install -Dm755 palmterm $(BINDIR)/palmterm
	install -Dm644 contrib/systemd/palmterm.service $(UNITDIR)/palmterm.service
	-systemctl --user daemon-reload
	@echo "Installed. Start it with:  systemctl --user enable --now palmterm"
	@echo "(If it was already running:  systemctl --user restart palmterm)"

uninstall:
	-systemctl --user disable --now palmterm
	rm -f $(BINDIR)/palmterm $(UNITDIR)/palmterm.service
	-systemctl --user daemon-reload
