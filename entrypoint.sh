#!/bin/sh
set -eu

mkdir -p /host/opt/cni/bin
install -m 0755 /usr/local/bin/zero-copy-cni /host/opt/cni/bin/zero-copy-cni
exec /usr/local/bin/zero-copy-agent