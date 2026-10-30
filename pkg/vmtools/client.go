// Copyright (c) 2026 Broadcom. All Rights Reserved.
// Broadcom Confidential. The term "Broadcom" refers to Broadcom Inc.
// and/or its subsidiaries.

// Package vmtools implements the transport on top of the vSphere Guest
// Operations API, which talks to open-vm-tools (vmtoolsd) inside the guest.
// It needs no guest networking and no sshd.
package vmtools

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/guest"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/soap"
	"github.com/vmware/govmomi/vim25/types"
)

const (
	DefaultTimeout = 5 * time.Minute
	pollInterval   = 500 * time.Millisecond
)

// Config describes how to reach a VM through vCenter or ESXi.
type Config struct {
	// URL is the vCenter or ESXi endpoint, e.g. https://vc.example.com/sdk.
	URL string
	// Username and Password authenticate to vCenter/ESXi.
	Username string
	Password string
	// Insecure skips TLS verification of vCenter and ESXi.
	Insecure bool
	// VM is an inventory path (starting with "/") or a VM instance/BIOS UUID.
	VM string
	// GuestUser and GuestPassword authenticate to the guest OS.
	GuestUser     string
	GuestPassword string
	// Timeout bounds each operation. Defaults to DefaultTimeout.
	Timeout time.Duration
}

// Client runs guest operations against one VM.
type Client struct {
	vc      *govmomi.Client
	auth    types.BaseGuestAuthentication
	procs   *guest.ProcessManager
	files   *guest.FileManager
	timeout time.Duration
}

// ExitError is returned by Run when the command exits non-zero.
type ExitError struct {
	Code int32
}

func (e *ExitError) Error() string { return fmt.Sprintf("command exited with status %d", e.Code) }

// NewClient logs in to vCenter/ESXi, locates the VM and prepares guest operations.
func NewClient(cfg *Config) (*Client, error) {
	if cfg.URL == "" || cfg.VM == "" {
		return nil, errors.New("vmtools: url and vm are required")
	}
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = DefaultTimeout
	}

	u, err := soap.ParseURL(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("vmtools: parse url: %w", err)
	}
	u.User = url.UserPassword(cfg.Username, cfg.Password)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	vc, err := govmomi.NewClient(ctx, u, cfg.Insecure)
	if err != nil {
		return nil, fmt.Errorf("vmtools: connect to %s: %w", u.Host, err)
	}

	c := &Client{
		vc:      vc,
		timeout: timeout,
		auth:    &types.NamePasswordAuthentication{Username: cfg.GuestUser, Password: cfg.GuestPassword},
	}
	if err := c.init(ctx, cfg.VM); err != nil {
		_ = c.Close()
		return nil, err
	}
	return c, nil
}

func (c *Client) init(ctx context.Context, vmRef string) error {
	vm, err := findVM(ctx, c.vc, vmRef)
	if err != nil {
		return err
	}
	ops := guest.NewOperationsManager(c.vc.Client, vm.Reference())
	if c.procs, err = ops.ProcessManager(ctx); err != nil {
		return fmt.Errorf("vmtools: process manager: %w", err)
	}
	if c.files, err = ops.FileManager(ctx); err != nil {
		return fmt.Errorf("vmtools: file manager: %w", err)
	}
	return nil
}

func findVM(ctx context.Context, vc *govmomi.Client, ref string) (*object.VirtualMachine, error) {
	if strings.HasPrefix(ref, "/") {
		vm, err := find.NewFinder(vc.Client).VirtualMachine(ctx, ref)
		if err != nil {
			return nil, fmt.Errorf("vmtools: find vm %q: %w", ref, err)
		}
		return vm, nil
	}

	si := object.NewSearchIndex(vc.Client)
	for _, instance := range []bool{true, false} {
		r, err := si.FindByUuid(ctx, nil, ref, true, &instance)
		if err != nil {
			return nil, fmt.Errorf("vmtools: find vm by uuid %q: %w", ref, err)
		}
		if r != nil {
			return object.NewVirtualMachine(vc.Client, r.Reference()), nil
		}
	}
	return nil, fmt.Errorf("vmtools: vm %q not found (use an inventory path starting with / or a VM UUID)", ref)
}

// Close logs out of vCenter/ESXi.
func (c *Client) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()
	return c.vc.Logout(ctx)
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// Run runs cmd through /bin/sh in the guest and returns its combined output.
// The Guest Operations API does not return stdout, so output goes to a temp
// file in the guest which is then downloaded and removed.
func (c *Client) Run(cmd string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	outPath := fmt.Sprintf("/tmp/etcd-recovery_%d.out", time.Now().UnixNano())
	pid, err := c.procs.StartProgram(ctx, c.auth, &types.GuestProgramSpec{
		ProgramPath: "/bin/sh",
		Arguments:   "-c " + shellQuote(fmt.Sprintf("{ %s\n} >%s 2>&1", cmd, outPath)),
	})
	if err != nil {
		return nil, fmt.Errorf("vmtools: start program: %w", err)
	}

	exitCode, waitErr := c.wait(ctx, pid)

	// Fetch output even when the command failed, to help diagnose.
	out, dlErr := c.readFile(ctx, outPath)
	_ = c.files.DeleteFile(ctx, c.auth, outPath)

	switch {
	case waitErr != nil:
		return out, waitErr
	case dlErr != nil:
		return nil, fmt.Errorf("vmtools: read command output: %w", dlErr)
	case exitCode != 0:
		return out, &ExitError{Code: exitCode}
	}
	return out, nil
}

func (c *Client) wait(ctx context.Context, pid int64) (int32, error) {
	t := time.NewTicker(pollInterval)
	defer t.Stop()
	for {
		infos, err := c.procs.ListProcesses(ctx, c.auth, []int64{pid})
		if err != nil {
			return 0, fmt.Errorf("vmtools: list processes: %w", err)
		}
		if len(infos) == 1 && infos[0].EndTime != nil {
			return infos[0].ExitCode, nil
		}
		select {
		case <-ctx.Done():
			return 0, fmt.Errorf("vmtools: waiting for pid %d: %w", pid, ctx.Err())
		case <-t.C:
		}
	}
}

func (c *Client) readFile(ctx context.Context, remotePath string) ([]byte, error) {
	rc, err := c.openDownload(ctx, remotePath)
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}

func (c *Client) openDownload(ctx context.Context, remotePath string) (io.ReadCloser, error) {
	info, err := c.files.InitiateFileTransferFromGuest(ctx, c.auth, remotePath)
	if err != nil {
		return nil, err
	}
	u, err := c.files.TransferURL(ctx, info.Url)
	if err != nil {
		return nil, err
	}
	rc, _, err := c.vc.Client.Client.Download(ctx, u, &soap.DefaultDownload)
	return rc, err
}

// Upload copies a local file to the guest, preserving its permission bits.
func (c *Client) Upload(localPath string, remotePath string) error {
	local, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer local.Close()
	st, err := local.Stat()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	attr := &types.GuestPosixFileAttributes{Permissions: int64(st.Mode().Perm())}
	rawURL, err := c.files.InitiateFileTransferToGuest(ctx, c.auth, remotePath, attr, st.Size(), true)
	if err != nil {
		return fmt.Errorf("vmtools: upload %s: %w", remotePath, err)
	}
	u, err := c.files.TransferURL(ctx, rawURL)
	if err != nil {
		return err
	}
	p := soap.DefaultUpload
	p.ContentLength = st.Size()
	if err := c.vc.Client.Client.Upload(ctx, local, u, &p); err != nil {
		return fmt.Errorf("vmtools: upload %s: %w", remotePath, err)
	}
	return nil
}

// Download copies a guest file to the local path.
func (c *Client) Download(remotePath string, localPath string) error {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	rc, err := c.openDownload(ctx, remotePath)
	if err != nil {
		return fmt.Errorf("vmtools: download %s: %w", remotePath, err)
	}
	defer rc.Close()

	local, err := os.Create(localPath)
	if err != nil {
		return err
	}
	if _, err := io.Copy(local, rc); err != nil {
		local.Close()
		return err
	}
	return local.Close()
}
