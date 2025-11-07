#!/usr/bin/env bash
set -euo pipefail

NETID="${1:-}"
UPLINK="${2:-eth0}"
ZTIF_HINT="${3:-}"
HOST_ONLY="${4:-}"

if [[ -z "${NETID}" ]]; then
  echo "usage: $0 <netid> <uplink> [ztif_override] [--host-only]" >&2
  exit 2
fi

# derive zt interface name (most distros use zt<netid>)
if [[ -z "${ZTIF_HINT}" ]]; then
  ZTIF="zt${NETID}"
  # fallback: pick the first zt* if exact doesn’t exist
  ip link show "${ZTIF}" >/dev/null 2>&1 || ZTIF="$(ip -o link | awk -F': ' '$2 ~ /^zt/{print $2; exit}')"
else
  ZTIF="${ZTIF_HINT}"
fi

if [[ -z "${ZTIF}" ]]; then
  echo "Could not determine ZeroTier interface" >&2
  exit 3
fi

BR="br-${NETID:0:12}"

# sysctls
mkdir -p /etc/sysctl.d
cat >/etc/sysctl.d/99-zerotier-bridge.conf <<EOF
net.ipv4.ip_forward=1
net.ipv6.conf.all.forwarding=1
net.bridge.bridge-nf-call-iptables=1
net.bridge.bridge-nf-call-ip6tables=1
EOF
modprobe br_netfilter || true
sysctl --system >/dev/null

if [[ "${HOST_ONLY:-}" == "--host-only" ]]; then
  echo "Sysctls applied; host-only requested. Exiting."
  exit 0
fi

# create bridge if not exists
ip link show "${BR}" >/dev/null 2>&1 || ip link add name "${BR}" type bridge
# enslave interfaces
ip link set "${ZTIF}" master "${BR}" || true
ip link set "${UPLINK}" master "${BR}" || true

# bring them up
ip link set "${BR}" up
ip link set "${ZTIF}" up
ip link set "${UPLINK}" up

echo "Bridge ${BR} up with ${ZTIF} <-> ${UPLINK}"
