#!/usr/bin/env bash
#
# Create a dedicated vCenter user and role for etcd-recovery's vmtools transport,
# and grant the role on the datacenter and its Namespaces folder.
#
# The role only has the guest operations etcd-recovery needs:
#   VirtualMachine.GuestOperations.{Query,Modify,Execute}
#
# Usage:
#   vcenter-setup.sh [setup]    create the role and user, and grant the role (default)
#   vcenter-setup.sh cleanup    remove the grants, the role and the user
#
# Admin credentials and the datacenter: edit the exports below, or set the
# variables in the environment (an already set variable wins). They are
# exported for this script only, and unset again at the end of cleanup.
#
# Optional variables:
#   NEW_USER      name of the user to create          (default: etcd-recovery)
#   NEW_PASSWORD  its password                        (default: prompted)
#   SSO_DOMAIN    SSO domain                          (default: vsphere.local)
#   ROLE_NAME     name of the role to create          (default: etcd-recovery-guestops)
#   VERIFY_VM     VM path or UUID; if set, setup checks the new user's access to it
#
# The role is granted, with propagation, on /<DATACENTER> and on
# /<DATACENTER>/vm/Namespaces, the folder of Supervisor-managed VMs.

set -euo pipefail

NEW_USER="${NEW_USER:-etcd-recovery}"
SSO_DOMAIN="${SSO_DOMAIN:-vsphere.local}"
ROLE_NAME="${ROLE_NAME:-etcd-recovery-guestops}"

PRIVILEGES=(
  VirtualMachine.GuestOperations.Query
  VirtualMachine.GuestOperations.Modify
  VirtualMachine.GuestOperations.Execute
)

die() { echo "error: $*" >&2; exit 1; }

usage() { sed -n '2,/^set -euo/p' "$0" | sed '$d' | sed 's/^# \{0,1\}//'; exit 1; }

[ $# -le 1 ] || usage
ACTION=${1:-setup}
case "$ACTION" in
  setup | cleanup) ;;
  *) usage ;;
esac

# vCenter admin credentials used by govc. Replace the placeholders.
export GOVC_URL="${GOVC_URL:-https://<vcenter>/sdk}"
export GOVC_USERNAME="${GOVC_USERNAME:-administrator@vsphere.local}"
export GOVC_PASSWORD="${GOVC_PASSWORD:-<admin-password>}"
export DATACENTER="${DATACENTER:-<datacenter>}"
export GOVC_INSECURE=1   # uncomment for a self-signed vCenter certificate

echo "GOVC_URL=${GOVC_URL}"

command -v govc >/dev/null || die "govc not found in PATH"
case "$GOVC_URL$GOVC_PASSWORD$DATACENTER" in
  *'<'*) die "set environment variables GOVC_URL, GOVC_PASSWORD and DATACENTER first" ;;
esac

# Inventory objects to grant the role on. DATACENTER may be given with or without a leading "/".
DC="/${DATACENTER#/}"
TARGETS=("$DC" "$DC/vm/Namespaces")

# govc wants DOMAIN\user, with the domain in upper case.
PRINCIPAL="$(printf '%s' "$SSO_DOMAIN" | tr '[:lower:]' '[:upper:]')\\${NEW_USER}"

# Make sure the connection and the admin credentials work before changing anything.
govc about >/dev/null || die "cannot connect to $GOVC_URL as $GOVC_USERNAME"

setup() {
  if [ -z "${NEW_PASSWORD:-}" ]; then
    read -rsp "Password for new user ${NEW_USER}@${SSO_DOMAIN}: " NEW_PASSWORD
    echo
  fi
  [ -n "$NEW_PASSWORD" ] || die "password must not be empty"

  # The role. An existing role is left as is, so check its privileges if you reuse one.
  # "govc role.ls" prints two columns, the role name and its label, so match the first one.
  if govc role.ls | awk -v r="$ROLE_NAME" '$1 == r { found = 1 } END { exit !found }'; then
    echo "role $ROLE_NAME already exists, skipping"
  else
    echo "creating role $ROLE_NAME"
    govc role.create "$ROLE_NAME" "${PRIVILEGES[@]}"
  fi

  # The user. It must not be a member of any group, since group permissions
  # on Supervisor folders can override the grants below.
  if govc sso.user.ls | grep -Eq "(^|[^A-Za-z0-9_.-])${NEW_USER}([^A-Za-z0-9_.-]|$)"; then
    echo "user $NEW_USER already exists, skipping"
  else
    echo "creating user ${NEW_USER}@${SSO_DOMAIN}"
    govc sso.user.create -p "$NEW_PASSWORD" "$NEW_USER"
  fi

  # The grants.
  for t in "${TARGETS[@]}"; do
    echo "granting $ROLE_NAME to $PRINCIPAL on $t"
    govc permissions.set -principal "$PRINCIPAL" -role "$ROLE_NAME" -propagate=true "$t"
  done

  if [ -n "${VERIFY_VM:-}" ]; then
    echo "verifying access of ${NEW_USER}@${SSO_DOMAIN} to $VERIFY_VM"
    # Credentials are set per command so they do not leak into this shell's govc calls.
    role=$(GOVC_USERNAME="${NEW_USER}@${SSO_DOMAIN}" GOVC_PASSWORD="$NEW_PASSWORD" \
      govc object.collect -s "$VERIFY_VM" effectiveRole) \
      || die "the new user cannot see $VERIFY_VM, check the grants"
    echo "effective role id on the VM: $role"
  fi

  cat <<EOF

Done. In hosts.json, set for each vmtools host:
  "vcenter_username": "${NEW_USER}@${SSO_DOMAIN}",
  "vcenter_password": "<the password you chose>"
EOF
}

cleanup() {
  for t in "${TARGETS[@]}"; do
    echo "removing permission of $PRINCIPAL on $t"
    govc permissions.remove -principal "$PRINCIPAL" "$t" || echo "  (nothing to remove, continuing)"
  done
  echo "removing role $ROLE_NAME"
  govc role.remove -force "$ROLE_NAME" || echo "  (role not found, continuing)"
  echo "removing user $NEW_USER"
  govc sso.user.rm "$NEW_USER" || echo "  (user not found, continuing)"

  unset GOVC_URL GOVC_USERNAME GOVC_PASSWORD DATACENTER
}

"$ACTION"
