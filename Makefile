PRO_VERSION ?= 5.4.3
LITE_VERSION ?= 0.3.3
BINARY := clawpanel
EDITION ?= pro
APP_NAME ?= $(BINARY)
VERSION ?= $(if $(filter lite,$(EDITION)),$(LITE_VERSION),$(PRO_VERSION))
MODULE := github.com/zhaoxinyi02/ClawPanel
LDFLAGS := -s -w -X github.com/zhaoxinyi02/ClawPanel/internal/buildinfo.Version=$(VERSION) -X github.com/zhaoxinyi02/ClawPanel/internal/buildinfo.Edition=$(EDITION)
WINDOWS_LDFLAGS := -X github.com/zhaoxinyi02/ClawPanel/internal/buildinfo.Version=$(VERSION) -X github.com/zhaoxinyi02/ClawPanel/internal/buildinfo.Edition=$(EDITION)
GOFLAGS := -trimpath
EMBED_DIR := cmd/clawpanel/frontend/dist

# npm 国内镜像
NPM_REGISTRY := https://registry.npmmirror.com

.PHONY: all clean frontend backend build dev cross cross-all installer help build-lite build-pro backend-lite backend-pro package-lite-core package-lite-plugins package-lite-qq

all: build

# 构建前端
frontend:
	@echo "==> 构建前端..."
	cd web && npm install --registry=$(NPM_REGISTRY) && npx vite build
	@echo "==> 复制前端产物到 embed 目录..."
	rm -rf $(EMBED_DIR)
	mkdir -p $(EMBED_DIR)
	cp -r web/dist/* $(EMBED_DIR)/
	@echo "==> 前端构建完成"

# 构建后端（需要先构建前端）
backend: frontend
	@echo "==> 构建 Go 后端..."
	CGO_ENABLED=0 go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o bin/$(APP_NAME) ./cmd/clawpanel/
	@echo "==> 后端构建完成: bin/$(APP_NAME)"

# 完整构建
build: backend
	@echo "==> ClawPanel v$(VERSION) 构建完成!"
	@ls -lh bin/$(APP_NAME)

# 开发模式（前端热重载 + Go 后端）
dev:
	@echo "==> 启动开发模式..."
	@echo "  前端: cd web && npm run dev"
	@echo "  后端: go run ./cmd/clawpanel/"

# 仅构建后端（假设前端已构建）
backend-only:
	CGO_ENABLED=0 go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o bin/$(APP_NAME) ./cmd/clawpanel/

backend-lite: frontend
	$(MAKE) backend APP_NAME=clawpanel-lite EDITION=lite

backend-pro: frontend
	$(MAKE) backend APP_NAME=clawpanel EDITION=pro

build-lite: frontend
	$(MAKE) backend-only APP_NAME=clawpanel-lite EDITION=lite

build-pro: frontend
	$(MAKE) backend-only APP_NAME=clawpanel EDITION=pro

package-lite-plugins:
	bash scripts/package-lite-plugins.sh

package-lite-core: build-lite
	bash scripts/package-lite-core.sh $(VERSION)

package-lite-qq:
	bash scripts/package-lite-qq-bundle.sh $(VERSION)

# 交叉编译所有平台（文件名带版本号）
cross: frontend
	@echo "==> 交叉编译..."
	@mkdir -p bin
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o bin/$(BINARY)-v$(VERSION)-linux-amd64 ./cmd/clawpanel/
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o bin/$(BINARY)-v$(VERSION)-linux-arm64 ./cmd/clawpanel/
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o bin/$(BINARY)-v$(VERSION)-darwin-amd64 ./cmd/clawpanel/
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o bin/$(BINARY)-v$(VERSION)-darwin-arm64 ./cmd/clawpanel/
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build $(GOFLAGS) -ldflags "$(WINDOWS_LDFLAGS)" -o bin/$(BINARY)-v$(VERSION)-windows-amd64.exe ./cmd/clawpanel/
	@echo "==> 交叉编译完成"
	@ls -lh bin/

# 构建 Windows exe 安装包
installer: cross
	@echo "==> 构建 Windows 安装包..."
	@mkdir -p installer/payload
	cp bin/$(BINARY)-v$(VERSION)-windows-amd64.exe installer/payload/clawpanel.exe
	cd installer && GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build $(GOFLAGS) -o ../bin/ClawPanel-Setup-v$(VERSION).exe .
	rm -f installer/payload/clawpanel.exe
	@echo "==> Windows 安装包构建完成"
	@ls -lh bin/ClawPanel-Setup-v$(VERSION).exe

# 构建所有发布产物（交叉编译 + Windows 安装包）
release: installer
	cp bin/$(BINARY)-v$(VERSION)-linux-amd64 bin/$(BINARY)
	@echo "==> ClawPanel v$(VERSION) 全部发布产物:"
	@ls -lh bin/

# 清理
clean:
	rm -rf bin/ web/dist/ web/node_modules/ $(EMBED_DIR) installer/payload/clawpanel.exe
	@echo "==> 已清理"

help:
	@echo "ClawPanel v$(VERSION) 构建系统"
	@echo ""
	@echo "  make build        完整构建（前端 + 后端）"
	@echo "  make frontend     仅构建前端"
	@echo "  make backend      构建后端（含前端）"
	@echo "  make backend-only 仅构建后端（需前端已构建）"
	@echo "  make build-lite   构建 Lite Linux 版主程序"
	@echo "  make build-pro    构建 Pro 版主程序"
	@echo "  make package-lite-plugins 收集 Lite 预置插件"
	@echo "  make package-lite-core  打包 Lite Core"
	@echo "  make package-lite-qq    导出 Lite QQ Bundle"
	@echo "  make cross        交叉编译所有平台"
	@echo "  make installer    构建 Windows exe 安装包"
	@echo "  make release      构建全部发布产物"
	@echo "  make dev          开发模式说明"
	@echo "  make clean        清理构建产物"
