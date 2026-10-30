#!/usr/bin/env bash
#
# Generate hosts.json for etcd-recovery's vmtools transport by discovering the
# control plane VMs in vCenter. Two kinds of clusters are supported:
#
#   supervisor  the Supervisor control plane VMs
#   vks         the control plane VMs of a VKS (workload) cluster
#
# For every VM found, one host entry is written:
#   name   the VM's name
#   host   the IPv4 address reported by VMware Tools
#   vm     the VM's inventory path
#
# Usage:
#   generate-hosts.sh [supervisor]                              (default)
#   generate-hosts.sh vks [<namespace> [<cluster-name>]]
#
# Discovery always uses the vCenter credentials GOVC_USERNAME and GOVC_PASSWORD.
# Edit the exports below, or set the variables in the environment (an already
# set variable wins):
#   GOVC_URL          vCenter endpoint
#   GOVC_USERNAME     vCenter user          (default: administrator@vsphere.local)
#   GOVC_PASSWORD     its password          (prompted when unset)
#   DATACENTER        the datacenter holding the VMs
#
# Guest OS credentials written for both kinds of clusters:
#   GUEST_USERNAME    guest OS user         (default: root for supervisor,
#                                            vmware-system-user for vks)
#   GUEST_PASSWORD    guest OS password     (prompted when unset)
#
# supervisor:
#   The VMs are those under /<DATACENTER>/vm named VM_NAME_PATTERN
#   (default: SupervisorControlPlaneVM*). The "vcenter_username" and
#   "vcenter_password" written to hosts.json are GOVC_USERNAME and GOVC_PASSWORD.
#
# vks:
#   NAMESPACE and CLUSTER_NAME (or the two arguments, or a prompt) select the
#   cluster. Its control plane VMs are the VMs under /<DATACENTER>/vm/Namespace/supervisor/<namespace>/<cluster-name>,
#   whose names start with "<cluster-name>-" and do not contain "node-pool" (those are the worker nodes).
#   The "vcenter_username" and "vcenter_password" written to hosts.json are:
#   VCENTER_USERNAME  (default: etcd-recovery@vsphere.local)
#   VCENTER_PASSWORD  (prompted when unset)
#
# Passwords are prompted for only when the script runs in a terminal.
#
# Optional variables:
#   BACKEDUP_MANIFEST  backedup_manifest value       (default: /root/etcd.yaml)
#   OUTPUT             file to write                 (default: hosts.json)
#   FORCE              set to 1 to overwrite OUTPUT
#
# The file contains passwords, so it is created with mode 600.
# Check the generated "host" values: a VM with several NICs may report an
# address that is not the one etcd listens on.

set -euo pipefail

die() { echo "error: $*" >&2; exit 1; }

usage() { sed -n '2,/^set -euo/p' "$0" | sed '$d' | sed 's/^# \{0,1\}//'; exit 1; }

MODE=${1:-supervisor}
case "$MODE" in
  supervisor) [ $# -le 1 ] || usage ;;
  vks)
    [ $# -le 3 ] || usage
    NAMESPACE=${2:-${NAMESPACE:-}}
    CLUSTER_NAME=${3:-${CLUSTER_NAME:-}}
    ;;
  *) usage ;;
esac

# Set the variable named $1 from a prompt unless it already has a value. The
# prompt needs a terminal. Pass "secret" as $3 to hide what is typed. The
# variable is exported, so jq can read passwords from the environment instead
# of the command line.
require_input() {
  local var=$1 prompt=$2 secret=${3:-}
  if [ -z "${!var:-}" ]; then
    [ -t 0 ] || die "$var is not set"
    if [ "$secret" = secret ]; then
      read -rsp "$prompt: " "${var?}"
      echo
    else
      read -rp "$prompt: " "${var?}"
    fi
  fi
  [ -n "${!var}" ] || die "$var must not be empty"
  export "${var?}"
}

# vCenter settings used by govc. Replace the placeholders.
export GOVC_URL="${GOVC_URL:-https://<vcenter>/sdk}"
export GOVC_USERNAME="${GOVC_USERNAME:-administrator@vsphere.local}"
export DATACENTER="${DATACENTER:-<datacenter>}"
export GOVC_INSECURE="${GOVC_INSECURE:-1}"   # set to 0 if vCenter has a trusted certificate

case "$MODE" in
  supervisor) GUEST_USERNAME="${GUEST_USERNAME:-root}" ;;
  vks) GUEST_USERNAME="${GUEST_USERNAME:-vmware-system-user}" ;;
esac
BACKEDUP_MANIFEST="${BACKEDUP_MANIFEST:-/root/etcd.yaml}"
OUTPUT="${OUTPUT:-hosts.json}"

command -v govc >/dev/null || die "govc not found in PATH"
command -v jq >/dev/null || die "jq not found in PATH"
case "$GOVC_URL$DATACENTER" in
  *'<'*) die "edit GOVC_URL and DATACENTER at the top of this script first" ;;
esac

if [ -e "$OUTPUT" ] && [ "${FORCE:-}" != 1 ]; then
  die "$OUTPUT already exists, set FORCE=1 to overwrite it"
fi

require_input GOVC_PASSWORD "Password of vCenter user ${GOVC_USERNAME}" secret
require_input GUEST_PASSWORD "Password of guest user ${GUEST_USERNAME}" secret

# The vCenter user etcd-recovery logs in as.
case "$MODE" in
  supervisor)
    VCENTER_USERNAME=$GOVC_USERNAME
    export VCENTER_PASSWORD=$GOVC_PASSWORD
    ;;
  vks)
    VCENTER_USERNAME="${VCENTER_USERNAME:-etcd-recovery@vsphere.local}"
    require_input VCENTER_PASSWORD "Password of vCenter user ${VCENTER_USERNAME}" secret
    require_input NAMESPACE "Namespace of the cluster, i.e. test-ns"
    require_input CLUSTER_NAME "Name of the cluster, i.e. wl-antrea"
    ;;
esac

# etcd-recovery wants https://<host>/sdk, while govc accepts a bare host.
vcenter_url=$GOVC_URL
case "$vcenter_url" in
  *://*) ;;
  *) vcenter_url="https://$vcenter_url" ;;
esac
case "${vcenter_url#*://}" in
  *@*) die "remove the credentials from GOVC_URL, use GOVC_USERNAME and GOVC_PASSWORD instead" ;;
esac
vcenter_url=${vcenter_url%/}
case "$vcenter_url" in
  */sdk) ;;
  *) vcenter_url="$vcenter_url/sdk" ;;
esac

case "$GOVC_INSECURE" in
  1 | true) insecure=true ;;
  *) insecure=false ;;
esac

DC="${DATACENTER#/}"

# Lists the inventory paths of the control plane VMs, one per line.
find_vms() {
  case "$MODE" in
    supervisor)
      govc find "/${DC}/vm" -type m -name "${VM_NAME_PATTERN:-SupervisorControlPlaneVM*}"
      ;;
    vks)
      # Keep the VMs of the namespace, and drop the worker nodes.
      govc find "/${DC}/vm" -type m -name "${CLUSTER_NAME}-*" \
        | grep -F "/${NAMESPACE}/" \
        | awk -F/ 'index($NF, "node-pool") == 0'
      ;;
  esac
}

# Make sure the connection and the credentials work before going further.
govc about >/dev/null || die "cannot connect to $GOVC_URL as $GOVC_USERNAME"

tmp=$(mktemp)
trap 'rm -f "$tmp" "${OUTPUT}.tmp.$$"' EXIT

echo "discovering $MODE control plane VMs"
count=0
while IFS= read -r vm_path; do
  [ -n "$vm_path" ] || continue
  name=${vm_path##*/}

  # vm.ip can return several comma separated addresses, take the first one.
  ip=$(govc vm.ip -v4 -wait 30s "$vm_path" 2>/dev/null | cut -d, -f1) || true
  [ -n "$ip" ] || die "no IPv4 address reported for $vm_path, are VMware Tools running?"

  jq -n \
    --arg name "$name" \
    --arg host "$ip" \
    --arg manifest "$BACKEDUP_MANIFEST" \
    --arg url "$vcenter_url" \
    --arg vuser "$VCENTER_USERNAME" \
    --argjson insecure "$insecure" \
    --arg vm "$vm_path" \
    --arg guser "$GUEST_USERNAME" \
    '{
      name: $name,
      host: $host,
      vmtools: (
        {vcenter_url: $url, vcenter_username: $vuser, vcenter_password: env.VCENTER_PASSWORD}
        + (if $insecure then {vcenter_insecure: true} else {} end)
        + {vm: $vm, guest_username: $guser, guest_password: env.GUEST_PASSWORD}
      ),
      backedup_manifest: $manifest
    }' >>"$tmp"

  printf '  %-45s %-16s %s\n' "$name" "$ip" "$vm_path"
  count=$((count + 1))
done < <(find_vms | sort)

if [ "$count" -eq 0 ]; then
  case "$MODE" in
    supervisor) die "no VM matching '${VM_NAME_PATTERN:-SupervisorControlPlaneVM*}' found under /${DC}/vm" ;;
    vks) die "no control plane VM of cluster '${CLUSTER_NAME}' found in namespace '${NAMESPACE}' under /${DC}/vm" ;;
  esac
fi

# Write through a private temp file so the passwords are never world readable.
(
  umask 077
  jq -s '.' "$tmp" >"${OUTPUT}.tmp.$$"
)
mv "${OUTPUT}.tmp.$$" "$OUTPUT"

echo "wrote $count host(s) to $OUTPUT (mode 600), check the host addresses above"
