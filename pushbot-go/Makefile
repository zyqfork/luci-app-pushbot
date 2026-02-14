# PushBot Go 版本 - OpenWrt 交叉编译
# 用法: make          # 当前系统
#       make arm      # linux/arm
#       make mips     # linux/mipsle (常见路由器)
#       make mips64   # linux/mips64le

BINARY=pushbot
VERSION=1.0.0
LDFLAGS=-ldflags "-s -w"

.PHONY: all arm mips mips64 clean

all:
	go build $(LDFLAGS) -o $(BINARY) .

# 常见 OpenWrt 架构
arm:
	GOOS=linux GOARCH=arm GOARM=7 CGO_ENABLED=0 go build $(LDFLAGS) -o $(BINARY)-arm .
mips:
	GOOS=linux GOARCH=mipsle GOMIPS=softfloat CGO_ENABLED=0 go build $(LDFLAGS) -o $(BINARY)-mipsle .
mips64:
	GOOS=linux GOARCH=mips64le CGO_ENABLED=0 go build $(LDFLAGS) -o $(BINARY)-mips64le .

clean:
	rm -f $(BINARY) $(BINARY)-*
