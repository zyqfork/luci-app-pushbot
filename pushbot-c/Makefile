# Pushbot C - 无 Go 依赖，OpenWrt 下用 TARGET_CC 交叉编译
CC ?= gcc
CFLAGS ?= -O2 -Wall
LDFLAGS ?=

SRCS = main.c config.c device.c push.c ip.c soc.c util.c
OBJS = $(SRCS:.c=.o)
TARGET = pushbot

all: $(TARGET)

$(TARGET): $(OBJS)
	$(CC) $(LDFLAGS) -o $@ $^

%.o: %.c
	$(CC) $(CFLAGS) -c -o $@ $<

clean:
	rm -f $(OBJS) $(TARGET)

.PHONY: all clean
