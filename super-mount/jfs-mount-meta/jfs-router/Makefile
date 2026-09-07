# jfs-router 的构建脚本。
#
# 常用命令：
#   make build      本地构建（默认目标）
#   make run        构建后直接用示例配置跑起来（需要 sudo，会真的挂载）
#   make vet        go vet 静态检查
#   make fmt        gofmt 格式化
#   make test       跑单元测试（如果之后补充了 _test.go）
#   make tidy       go mod tidy 整理依赖
#   make cross      跨平台交叉编译出 linux/amd64、linux/arm64 两个二进制
#   make install    构建后安装到 /usr/local/bin（需要 sudo）
#   make clean      清理构建产物

MODULE      := jfs-router
BINARY      := jfs-router
PKG         := ./cmd/jfs-router
BUILD_DIR   := bin
INSTALL_DIR := /usr/local/bin

# 版本信息注入：优先用 git describe，拿不到就用 "dev"。
VERSION     := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
BUILD_TIME  := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS     := -s -w \
               -X 'main.version=$(VERSION)' \
               -X 'main.buildTime=$(BUILD_TIME)'

# go-fuse 是纯 Go 实现（直接对接 /dev/fuse 协议，不依赖 libfuse），
# 不需要 cgo，CGO_ENABLED=0 可以产出静态二进制，部署时不用担心目标
# 机器的动态库版本问题。
export CGO_ENABLED := 0
export GOFLAGS     :=

.PHONY: all build run vet fmt test tidy cross install uninstall clean

all: build

## 本地构建（默认用当前 GOOS/GOARCH）
build:
	@mkdir -p $(BUILD_DIR)
	go build -trimpath -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY) $(PKG)
	@echo "已构建: $(BUILD_DIR)/$(BINARY) (version=$(VERSION))"

## 构建后直接用文件元数据源的示例配置启动（会真的挂载 /data，需要 sudo）
run: build
	sudo ./$(BUILD_DIR)/$(BINARY) -config configs/config.example.file.json

## 静态检查
vet:
	go vet ./...

## 格式化代码，检查是否有未格式化的文件
fmt:
	@unformatted=$$(gofmt -l .); \
	if [ -n "$$unformatted" ]; then \
		echo "以下文件未格式化:"; echo "$$unformatted"; \
		gofmt -w .; \
		echo "已自动格式化，请重新 git diff 确认改动"; \
	else \
		echo "格式检查通过"; \
	fi

## 单元测试（预留；当前仓库还没有 _test.go，命令本身是幂等的）
test:
	go test ./...

## 整理 go.mod/go.sum
tidy:
	go mod tidy

## 交叉编译出 linux/amd64 与 linux/arm64 两个二进制。
## go-fuse 依赖 /dev/fuse，只支持类 Unix（本项目主要面向 Linux 生产环境，
## 因此这里只交叉编译 Linux 目标；如需 macOS 上跑 osxfuse，理论上也能编译，
## 但未经测试，不在默认交叉编译列表里）。
cross:
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "$(LDFLAGS)" \
		-o $(BUILD_DIR)/$(BINARY)-linux-amd64 $(PKG)
	GOOS=linux GOARCH=arm64 go build -trimpath -ldflags "$(LDFLAGS)" \
		-o $(BUILD_DIR)/$(BINARY)-linux-arm64 $(PKG)
	@echo "已构建: $(BUILD_DIR)/$(BINARY)-linux-amd64, $(BUILD_DIR)/$(BINARY)-linux-arm64"

## 安装到 $(INSTALL_DIR)（默认 /usr/local/bin），需要有写权限（一般要 sudo）
install: build
	install -m 0755 $(BUILD_DIR)/$(BINARY) $(INSTALL_DIR)/$(BINARY)
	@echo "已安装到 $(INSTALL_DIR)/$(BINARY)"

uninstall:
	rm -f $(INSTALL_DIR)/$(BINARY)

## 清理构建产物
clean:
	rm -rf $(BUILD_DIR)
