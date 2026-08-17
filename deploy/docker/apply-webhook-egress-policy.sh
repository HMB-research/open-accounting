#!/usr/bin/env bash
# Apply Docker host egress restrictions for the API container.
#
# The API also talks to PostgreSQL and optional private infrastructure, so this
# script requires explicit allowlists before denying special-use destinations.
# Webhook URL validation in the application remains the primary safeguard.

set -euo pipefail

API_CONTAINER=""
ALLOW_PRIVATE=()
DRY_RUN=false
REMOVE=false
CHAIN="OPEN_ACCOUNTING_WEBHOOK_EGRESS"

usage() {
    cat <<'EOF'
Usage: deploy/docker/apply-webhook-egress-policy.sh --api-container NAME [options]

Options:
  --api-container NAME       Docker API container to protect (required).
  --allow-private CIDR       Private destination required by the API (repeatable).
  --dry-run                  Print iptables commands without changing the host.
  --remove                   Remove this script's chain and jump rule.
  -h, --help                 Show this help.

The policy blocks Docker-forwarded traffic from the API container to loopback,
private, link-local, carrier-grade NAT, documentation, benchmark, multicast,
and future-reserved networks. Add the database and any required private service
addresses with --allow-private before applying it. Run as root on the Docker host.
EOF
}

fail() {
    echo "ERROR: $*" >&2
    exit 1
}

run() {
    if [ "$DRY_RUN" = true ]; then
        printf '+ '
        printf '%q ' "$@"
        printf '\n'
        return
    fi
    "$@"
}

while [ "$#" -gt 0 ]; do
    case "$1" in
        --api-container)
            [ "$#" -ge 2 ] || fail "--api-container requires a value"
            API_CONTAINER="$2"
            shift 2
            ;;
        --allow-private)
            [ "$#" -ge 2 ] || fail "--allow-private requires a CIDR"
            ALLOW_PRIVATE+=("$2")
            shift 2
            ;;
        --dry-run)
            DRY_RUN=true
            shift
            ;;
        --remove)
            REMOVE=true
            shift
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            fail "unknown option: $1"
            ;;
    esac
done

[ -n "$API_CONTAINER" ] || fail "--api-container is required"
command -v docker >/dev/null 2>&1 || fail "docker is required"
command -v iptables >/dev/null 2>&1 || fail "iptables is required"

API_IP="$(docker inspect --format '{{range .NetworkSettings.Networks}}{{.IPAddress}} {{end}}' "$API_CONTAINER" 2>/dev/null || true)"
API_IP="${API_IP%% *}"
[ -n "$API_IP" ] || fail "could not resolve a Docker IPv4 address for $API_CONTAINER"
API_IPV6="$(docker inspect --format '{{range .NetworkSettings.Networks}}{{.GlobalIPv6Address}} {{end}}' "$API_CONTAINER" 2>/dev/null || true)"
API_IPV6="${API_IPV6%% *}"

if [ "$REMOVE" = true ]; then
    run iptables -D DOCKER-USER -j "$CHAIN" 2>/dev/null || true
    run iptables -F "$CHAIN" 2>/dev/null || true
    run iptables -X "$CHAIN" 2>/dev/null || true
    if [ -n "$API_IPV6" ] && command -v ip6tables >/dev/null 2>&1; then
        run ip6tables -D DOCKER-USER -j "$CHAIN" 2>/dev/null || true
        run ip6tables -F "$CHAIN" 2>/dev/null || true
        run ip6tables -X "$CHAIN" 2>/dev/null || true
    fi
    exit 0
fi

if [ "${#ALLOW_PRIVATE[@]}" -eq 0 ]; then
    fail "at least one --allow-private destination is required (normally the PostgreSQL container IP)"
fi

if [ "$DRY_RUN" = false ] && [ "$(id -u)" -ne 0 ]; then
    fail "run as root or with sudo to apply host firewall rules"
fi

run iptables -N "$CHAIN" 2>/dev/null || true
run iptables -F "$CHAIN"

for destination in "${ALLOW_PRIVATE[@]}"; do
    if [[ "$destination" != *:* ]]; then
        run iptables -A "$CHAIN" -s "$API_IP" -d "$destination" -j RETURN
    fi
done

for destination in \
    0.0.0.0/8 \
    10.0.0.0/8 \
    100.64.0.0/10 \
    127.0.0.0/8 \
    169.254.0.0/16 \
    172.16.0.0/12 \
    192.0.0.0/24 \
    192.0.2.0/24 \
    192.168.0.0/16 \
    198.18.0.0/15 \
    198.51.100.0/24 \
    203.0.113.0/24 \
    224.0.0.0/4 \
    240.0.0.0/4; do
    run iptables -A "$CHAIN" -s "$API_IP" -d "$destination" -j REJECT
done
run iptables -A "$CHAIN" -j RETURN

if [ "$DRY_RUN" = true ]; then
    run iptables -I DOCKER-USER 1 -j "$CHAIN"
elif ! iptables -C DOCKER-USER -j "$CHAIN" 2>/dev/null; then
    iptables -I DOCKER-USER 1 -j "$CHAIN"
fi

if [ -n "$API_IPV6" ]; then
    command -v ip6tables >/dev/null 2>&1 || fail "ip6tables is required when the API container has IPv6 enabled"
    run ip6tables -N "$CHAIN" 2>/dev/null || true
    run ip6tables -F "$CHAIN"

    for destination in "${ALLOW_PRIVATE[@]}"; do
        if [[ "$destination" == *:* ]]; then
            run ip6tables -A "$CHAIN" -s "$API_IPV6" -d "$destination" -j RETURN
        fi
    done

    for destination in \
        ::/128 \
        ::1/128 \
        fc00::/7 \
        fe80::/10 \
        2001:db8::/32 \
        ff00::/8; do
        run ip6tables -A "$CHAIN" -s "$API_IPV6" -d "$destination" -j REJECT
    done
    run ip6tables -A "$CHAIN" -j RETURN

    if [ "$DRY_RUN" = true ]; then
        run ip6tables -I DOCKER-USER 1 -j "$CHAIN"
    elif ! ip6tables -C DOCKER-USER -j "$CHAIN" 2>/dev/null; then
        ip6tables -I DOCKER-USER 1 -j "$CHAIN"
    fi
fi

echo "Applied webhook egress policy for $API_CONTAINER ($API_IP${API_IPV6:+, $API_IPV6})"
