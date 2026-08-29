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
	# 编译安装
	@echo "==> go install ($(VERSION) @ $(COMMIT))"
	cd server && go install -ldflags "$(LDFLAGS)"
	@echo "==> installed cube ($(VERSION) @ $(COMMIT), $(BUILD_TIME))"

	# 安装完成，确认生效：PATH 上的 cube 必须是刚构建的版本（输出含本次
	# BUILD_TIME），否则视为安装未生效（GOBIN 不在 PATH / 旧版本在前等），中止
	@cube version | grep -qF "$(BUILD_TIME)" || { echo "!! 安装校验失败：PATH 上的 cube 不是刚构建的版本（检查 GOBIN 是否在 PATH 且优先于旧安装）" >&2; exit 1; }

	# 关闭旧版本 server（服务未运行时 stop 会非零退出，- 忽略）
	-@cube server stop 2>/dev/null

	# 安装 zsh completion + shell 扩展（p / pz，见 scripts/zsh-append.sh）
	# completion 生成是覆盖写，重复 install 不会累积
	cube completion zsh > $(ZSH_COMPLETION_FILE)
	cat scripts/zsh-append.sh >> $(ZSH_COMPLETION_FILE)


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

last-proposal:
	@ls -d docs/proposals/*/ docs/proposals/archived/*/ docs/proposals/parked/*/ 2>/dev/null \
		| awk -F/ '/\/1[0-9]{3}-/ {print $$(NF-1)}' | sort | tail -1

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

dev-link-config:
	mkdir -p ./tmp/.config
	ln -s ~/.config/cube-dev ./tmp/.config/cube-dev
	ln -s ~/.config/cube ./tmp/.config/cube