IMAGE ?= zero-copy:dev
BPF_CLANG ?= clang
BPF_CFLAGS ?= -O2 -g -target bpf -D__TARGET_ARCH_x86

.PHONY: all build build-bpf build-go image clean

all: build

build: build-bpf build-go

build-bpf:
	$(BPF_CLANG) $(BPF_CFLAGS) -c ebpf/fast-path.c -o ebpf/fast-path.o

build-go:
	CGO_ENABLED=0 go build -trimpath -o bin/zero-copy-agent ./daemon
	CGO_ENABLED=0 go build -trimpath -o bin/zero-copy-cni ./cmd/cni

image:
	docker build -t $(IMAGE) .

clean:
	rm -rf bin ebpf/fast-path.o