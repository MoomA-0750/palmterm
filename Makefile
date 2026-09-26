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
