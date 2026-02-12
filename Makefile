.PHONY: build-wasm dev-frontend build-all

# 編譯 Go 為 WebAssembly
build-wasm:
	GOOS=js GOARCH=wasm go build -o frontend/public/main.wasm cmd/wasm/main.go

# 啟動前端開發伺服器
dev-frontend: build-wasm
	cd frontend && npm run dev

# 構建專案 (準備部署到 GitHub Pages)
build-all: build-wasm
	cd frontend && npm run build
