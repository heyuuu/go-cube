.DEFAULT_GOAL := build

# 从 git 收集构建期信息（与 version/version.go 配合，通过 ldflags 注入）
VERSION    := $(shell git describe --tags --abbrev=0 2>/dev/null || git rev-parse --short HEAD)
COMMIT     := $(shell git rev-parse --short HEAD)
BUILD_TIME := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

VERSION_PKG := github.com/heyuuu/cube/version
LDFLAGS := \
  -X $(VERSION_PKG).Version=$(VERSION) \
  -X $(VERSION_PKG).Commit=$(COMMIT) \
  -X $(VERSION_PKG).BuildTime=$(BUILD_TIME)

OUTPUT ?= tmp/cube

.PHONY: build install

build: ## 构建到 OUTPUT（默认 tmp/cube）
	@echo "==> go build ($(VERSION) @ $(COMMIT))"
	go build -ldflags "$(LDFLAGS)" -o $(OUTPUT)
	@echo "==> built $(OUTPUT) ($(VERSION) @ $(COMMIT), $(BUILD_TIME))"

install: ## go install 到 GOBIN（默认 ~/go/bin）
	@echo "==> go install ($(VERSION) @ $(COMMIT))"
	go install -ldflags "$(LDFLAGS)"
	@echo "==> installed cube ($(VERSION) @ $(COMMIT), $(BUILD_TIME))"

install-zsh-completion:
	cube completion zsh > ~/.config/cube/zsh.sh

.PHONY: tag

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