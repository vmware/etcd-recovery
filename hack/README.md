# hack

Helper scripts for using etcd-recovery with the vmtools transport. Both need [govc](https://github.com/vmware/govmomi/tree/main/govc)
in `PATH`, and `generate-hosts.sh` also needs `jq`.

Typical order: for a VKS cluster, run `vcenter-setup.sh` once, then `generate-hosts.sh` to produce `hosts.json`.
For a Supervisor cluster, `vcenter-setup.sh` is not needed: `generate-hosts.sh` uses the vCenter admin credentials directly.

## vcenter-setup.sh

Only needed for a VKS cluster, where etcd-recovery logs in as a dedicated vCenter user. A Supervisor cluster uses the
vCenter admin credentials, so skip this script there.

Creates a dedicated vCenter user and a least-privilege role (`VirtualMachine.GuestOperations.{Query,Modify,Execute}`),
and grants the role on the datacenter and on its `vm/Namespaces` folder. This avoids the `NoPermission` errors that
Supervisor-managed folders can cause for other users.

```
hack/vcenter-setup.sh [setup]    # create the role and user, and grant the role (default)
hack/vcenter-setup.sh cleanup    # remove the grants, the role and the user
```

Set these in the environment (or edit the exports at the top of the script):

| Variable                         | Description                                                                   |
|----------------------------------|-------------------------------------------------------------------------------|
| `GOVC_URL`                       | vCenter endpoint, with or without the `/sdk` path (govc adds it when missing) |
| `GOVC_USERNAME`, `GOVC_PASSWORD` | vCenter admin credentials                                                     |
| `DATACENTER`                     | datacenter name                                                               |

Optional: `NEW_USER` (default `etcd-recovery`), `NEW_PASSWORD` (prompted when unset), `SSO_DOMAIN` (default `vsphere.local`),
`ROLE_NAME` (default `etcd-recovery-guestops`), `VERIFY_VM` (a VM path or UUID to check the new user's access to).

Example:

```
export GOVC_URL=https://vc.example.com/sdk GOVC_PASSWORD='<admin-password>' DATACENTER=dc1
hack/vcenter-setup.sh
```

Then use `<NEW_USER>@<SSO_DOMAIN>` (default `etcd-recovery@vsphere.local`) as `vcenter_username` in `hosts.json`.

## generate-hosts.sh

Discovers the control plane VMs in vCenter and writes a `hosts.json` with a `vmtools` entry for each of them
(`name`, `host` = the IPv4 address reported by VMware Tools, `vm` = the inventory path). The file contains passwords,
so it is created with mode 600. **Check the generated `host` values, since a VM with several NICs may report an
address that is not the one etcd listens on**.

```
hack/generate-hosts.sh [supervisor]                       # Supervisor control plane VMs (default)
hack/generate-hosts.sh vks [<namespace> [<cluster-name>]] # control plane VMs of a VKS cluster
```

Discovery always uses `GOVC_URL`, `GOVC_USERNAME`, `GOVC_PASSWORD` and `DATACENTER` (as above). Passwords are prompted
for when unset and the script runs in a terminal.

| Variable                           | Supervisor                                                             | VKS                                                                                                                 |
|------------------------------------|------------------------------------------------------------------------|---------------------------------------------------------------------------------------------------------------------|
| `vcenter_username` in `hosts.json` | `GOVC_USERNAME` (default `administrator@vsphere.local`)                | `VCENTER_USERNAME` (default `etcd-recovery@vsphere.local`)                                                          |
| `vcenter_password` in `hosts.json` | `GOVC_PASSWORD`                                                        | `VCENTER_PASSWORD`                                                                                                  |
| `guest_username` in `hosts.json`   | `GUEST_USERNAME` (default `root`)                                      | `GUEST_USERNAME` (default `vmware-system-user`)                                                                     |
| `guest_password` in `hosts.json`   | `GUEST_PASSWORD` (prompted when unset)                                 | `GUEST_PASSWORD` (prompted when unset)                                                                              |
| VMs selected                       | names matching `VM_NAME_PATTERN` (default `SupervisorControlPlaneVM*`) | names starting with `<cluster-name>-` in the `<namespace>` folder, excluding workers (names containing `node-pool`) |

For VKS, the namespace and cluster name come from the arguments, the `NAMESPACE` and `CLUSTER_NAME` variables, or a prompt.

Optional: `BACKEDUP_MANIFEST` (default `/root/etcd.yaml`), `OUTPUT` (default `hosts.json`), `FORCE=1` to overwrite an existing `OUTPUT`.

Examples:

```
export GOVC_URL=https://vc.example.com/sdk GOVC_PASSWORD='<admin-password>' DATACENTER=dc1

# Supervisor cluster
GUEST_PASSWORD='<root-password>' hack/generate-hosts.sh

# VKS cluster
VCENTER_PASSWORD='<password>' GUEST_PASSWORD='<password>' hack/generate-hosts.sh vks test-ns wl-antrea
```
