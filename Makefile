.DEFAULT_GOAL := build
.PHONY: build-ui build install tag

# 输入参数
OUTPUT ?= tmp/cube

# 从 git 收集构建期信息（与 version/version.go 配合，通过 ldflags 注入）
VERSION    := $(shell git describe --tags --abbrev=0 2>/dev/null || git rev-parse --short HEAD)
COMMIT     := $(shell git rev-parse --short HEAD)
BUILD_TIME := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

VERSION_PKG := cube/version
LDFLAGS := \
  -X $(VERSION_PKG).version=$(VERSION) \
  -X $(VERSION_PKG).commit=$(COMMIT) \
  -X $(VERSION_PKG).buildTime=$(BUILD_TIME)

ZSH_COMPLETION_FILE := ~/.config/cube/zsh.sh

# go install 的落地目录：GOBIN 未设时退回 GOPATH/bin
GOBIN_DIR := $(shell go env GOBIN)
ifeq ($(GOBIN_DIR),)
GOBIN_DIR := $(shell go env GOPATH)/bin
endif

build-ui:
	rm -rf ./server/web/ui
	pnpm -C ./web build
	cp -r ./web/dist ./server/web/ui

build: build-ui
	@echo "==> go build ($(VERSION) @ $(COMMIT))"
	cd server && go build -ldflags "$(LDFLAGS)" -o ../$(OUTPUT)
	@echo "==> built $(OUTPUT) ($(VERSION) @ $(COMMIT), $(BUILD_TIME))"
	@$(OUTPUT) version

install: build-ui
	@echo "==> go install ($(VERSION) @ $(COMMIT))"
	cd server && go install -ldflags "$(LDFLAGS)"
	@echo "==> installed cube ($(VERSION) @ $(COMMIT), $(BUILD_TIME))"
	# cubex 是本目录模式 wrapper：cubex <args> == cube <args> --local（query 缺省以 cwd 定位项目）
	@printf '#!/bin/sh\nexec $(GOBIN_DIR)/cube "$$@" --local\n' > $(GOBIN_DIR)/cubex
	@chmod +x $(GOBIN_DIR)/cubex
	cube version
	# install zsh completion（末尾追加 compdef，让 cubex 复用 _cube 的补全）
	cube completion zsh > $(ZSH_COMPLETION_FILE)
	echo "compdef _cube cubex" >> $(ZSH_COMPLETION_FILE)

tag: ## 在当前位置打一个新版本 tag（上个版本末位 +1，如 v3.0.6 -> v3.0.7）
	@set -e; \
	export LC_ALL="${LC_ALL:-en_US.UTF-8}" LANG="${LANG:-en_US.UTF-8}"; \
	prev=$$(git describe --tags --abbrev=0 2>/dev/null); \
	if [ -z "$$prev" ]; then echo "==> 仓库还没有任何 tag" >&2; exit 1; fi; \
	prev_commit=$$(git rev-parse "$$prev^{commit}"); \
	curr_commit=$$(git rev-parse HEAD); \
	if [ "$$prev_commit" = "$$curr_commit" ]; then \
	  echo "==> 当前位置 $${curr_commit:0:7} 已是上个版本 $$prev, 无需打新 tag" >&2; exit 1; \
	fi; \
	major=$$(echo "$$prev" | sed -E 's/^v([0-9]+)\.([0-9]+)\.([0-9]+).*$$/\1/'); \
	minor=$$(echo "$$prev" | sed -E 's/^v([0-9]+)\.([0-9]+)\.([0-9]+).*$$/\2/'); \
	patch=$$(echo "$$prev" | sed -E 's/^v([0-9]+)\.([0-9]+)\.([0-9]+).*$$/\3/'); \
	patch=$$((patch + 1)); \
	new_tag="v$$major.$$minor.$$patch"; \
	echo "==> 上个版本: $$prev ($${prev_commit:0:7})"; \
	echo "==> 新版本  : $$new_tag ($${curr_commit:0:7})"; \
	git tag -a "$$new_tag" -m "release $$new_tag"; \
	echo "==> 已打 tag $$new_tag, 如需推送: git push origin $$new_tag"

# -------

# 起动前清掉 6001 上的残留监听：air 被异常退出后孤儿 server 会一直占着端口，
# 之后 air 每次热重载的新进程都因端口冲突起不来，表现为「改了代码不生效」。
# 注意必须 -sTCP:LISTEN 只杀监听进程——不带过滤会把连着 6001 的客户端
# （vite 代理、浏览器连接等）一起杀掉
dev-server:
	-@lsof -ti :6001 -sTCP:LISTEN | xargs kill 2>/dev/null || true
	cd server && air

dev-web:
	cd web && pnpm dev