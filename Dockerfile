FROM golang:1.26-bookworm AS builder

RUN apt-get update \
    && apt-get install -y --no-install-recommends clang llvm libbpf-dev linux-libc-dev \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .

RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/zero-copy-agent ./daemon
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/zero-copy-cni ./cmd/cni
RUN clang -O2 -g -target bpf -D__TARGET_ARCH_x86 -I/usr/include/x86_64-linux-gnu \
    -c ebpf/fast-path.c -o /out/fast-path.o

FROM debian:bookworm-slim

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates iproute2 \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder /out/zero-copy-agent /usr/local/bin/zero-copy-agent
COPY --from=builder /out/zero-copy-cni /usr/local/bin/zero-copy-cni
COPY --from=builder /out/fast-path.o /opt/zero-copy/fast-path.o
COPY entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod 0755 /usr/local/bin/entrypoint.sh

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]