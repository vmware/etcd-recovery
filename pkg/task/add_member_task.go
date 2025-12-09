// Copyright (c) 2025 Broadcom. All Rights Reserved.
// Broadcom Confidential. The term "Broadcom" refers to Broadcom Inc.
// and/or its subsidiaries.

package task

import (
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.etcd.io/etcd/api/v3/etcdserverpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"

	"github.com/vmware/etcd-recovery/pkg/cliui"
	"github.com/vmware/etcd-recovery/pkg/config"
	"github.com/vmware/etcd-recovery/pkg/ssh"
)

type AddMemberTask struct {
	Description string
	Master      *config.Host
	Learner     *config.Host
	AllHosts    []*config.Host
}

func (t *AddMemberTask) Name() string {
	return "AddMemberTask"
}

func (t *AddMemberTask) Run(client *ssh.Client) (string, error) {
	log.Printf("Starting AddMemberTask for learner %s (%s)\n", t.Learner.Name, t.Learner.Host)

	// Add or promote learner on master node
	promoted, err := t.addOrPromoteLearner(client)
	if err != nil {
		return "", fmt.Errorf("failed to add or promote learner: %w", err)
	}

	if promoted {
		log.Printf("Learner %s (%s) was already added and promoted successfully\n", t.Learner.Name, t.Learner.Host)
		return "learner already promoted", nil
	}

	// Start learner on learner node
	if err = t.startLearner(client); err != nil {
		return "", fmt.Errorf("failed to start learner: %w", err)
	}

	// Promote learner on master node
	promoted, err = t.addOrPromoteLearner(client)
	if err != nil {
		return "", fmt.Errorf("failed to promote learner after start: %w", err)
	}

	if !promoted {
		return "", fmt.Errorf("learner was not promoted after starting")
	}

	log.Printf("Successfully added and promoted learner %s (%s)\n", t.Learner.Name, t.Learner.Host)
	return "learner added and promoted successfully", nil
}

// addOrPromoteLearner adds or promotes a learner
// Returned values:
//   - bool: true means a learner is promoted; false means a learner is added
//   - error: error if any
func (t *AddMemberTask) addOrPromoteLearner(masterClient *ssh.Client) (bool, error) {
	log.Printf("AddOrPromoteLearner: checking cluster health and member status\n")
	var member *etcdserverpb.Member
	var memberID uint64
	var err error

	containerID, err := t.getEtcdContainerID(masterClient)
	if err != nil {
		return false, fmt.Errorf("failed to get etcd container ID: %w", err)
	}

	if err = t.waitForClusterOrMemberStatusHealthy(masterClient, containerID, true); err != nil {
		return false, fmt.Errorf("cluster health check failed: %w", err)
	}

	member, err = t.querryMember(masterClient, containerID)
	if err != nil {
		return false, fmt.Errorf("failed to check member existence: %w", err)
	}

	if member != nil {
		log.Printf("Member %s (%s) already exists in cluster (ID: %x, IsLearner: %v)\n", t.Learner.Name, t.Learner.Host, fmt.Sprintf("%x", member.ID), member.IsLearner)
		if member.IsLearner {
			if member.Name == "" {
				// The previous repair process was canceled or interrupted after the
				// learner was added but before the learner had actually started.
				log.Printf("The learner %x (%v) isn't started yet", member.ID, member.PeerURLs)
				return false, nil
			}
			log.Printf("Attempting to promote learner %s (%s)\n", t.Learner.Name, t.Learner.Host)
			if err = t.promoteLearner(masterClient, containerID, fmt.Sprintf("%x", member.ID)); err != nil {
				return false, fmt.Errorf("failed to promote learner: %w", err)
			}
			log.Printf("Successfully promoted learner %s (%s)\n", t.Learner.Name, t.Learner.Host)
			return true, nil
		}
		return true, nil
	}

	// handle other learners if exists
	err = t.handleOtherLearnersIfExists(masterClient, containerID)
	if err != nil {
		return false, err
	}

	log.Printf("Adding new member %s (%s) as learner\n", t.Learner.Name, t.Learner.Host)
	if memberID, err = t.addMemberToCluster(masterClient, containerID, true); err != nil {
		return false, fmt.Errorf("failed to add member %s (%s): %w", t.Learner.Name, t.Learner.Host, err)
	}

	log.Printf("Successfully added member %s with ID: %x", t.Learner.Name, fmt.Sprintf("%x", memberID))
	return false, nil
}

func (t *AddMemberTask) handleOtherLearnersIfExists(masterClient *ssh.Client, containerID string) error {
	// check for other learners if exists?
	otherLearnerMembers := t.fetchLearnerMembers(masterClient, containerID)
	if len(otherLearnerMembers) == 0 {
		// no learners found
		return nil
	}

	if len(otherLearnerMembers) > 1 {
		return fmt.Errorf("found %d learners, expected 0 or 1", len(otherLearnerMembers))
	}

	// extract learnerIP
	learnerIP := extractIPFromPeerURL(otherLearnerMembers[0].PeerURLs[0])
	log.Printf("Found other learner (%v) in cluster, handling...\n", learnerIP)
	if t.isKnownHost(otherLearnerMembers[0].PeerURLs[0]) {
		errorMsg := fmt.Sprintf("Another learner vm (%s) has been added but not started yet. Please add it again first.", learnerIP)
		log.Printf("WARNING: %s\n", errorMsg)
		return fmt.Errorf("%s", errorMsg)
	}

	// Remove the unknown learner
	log.Printf("Removing unknown learner %x at %s", otherLearnerMembers[0].ID, learnerIP)
	if err := t.removeMember(masterClient, containerID, fmt.Sprintf("%x", otherLearnerMembers[0].ID)); err != nil {
		return fmt.Errorf("failed to remove unknown learner %x: %w", otherLearnerMembers[0].ID, err)
	}
	log.Printf("Successfully removed unknown learner %x at %s", otherLearnerMembers[0].ID, learnerIP)

	return nil
}

func (t *AddMemberTask) isKnownHost(peerURL string) bool {
	isKnownHost := false
	learnerIP := extractIPFromPeerURL(peerURL)
	for _, h := range t.AllHosts {
		if h.Host == learnerIP {
			isKnownHost = true
			break
		}
	}
	return isKnownHost
}

func (t *AddMemberTask) removeMember(client *ssh.Client, containerID string, memberID string) error {
	log.Printf("Removing member %s", memberID)
	out, err := t.execEtcdctl(client, containerID, "member", "remove", memberID)
	if err != nil {
		if strings.Contains(out, "Member not found") {
			log.Printf("Member %s already removed", memberID)
			return nil
		}
		return fmt.Errorf("failed to remove member %s: %w", memberID, err)
	}
	log.Printf("Member %s removed successfully", memberID)
	return nil
}

func (t *AddMemberTask) startLearner(masterClient *ssh.Client) error {
	learnerClient, err := ssh.NewClient(&ssh.Config{
		User:                 t.Learner.Username,
		Host:                 t.Learner.Host,
		Password:             t.Learner.Password,
		PrivateKeyPath:       t.Learner.PrivateKey,
		PrivateKeyPassphrase: t.Learner.Passphrase,
	})
	if err != nil {
		return fmt.Errorf("failed to connect to Learner node: %w", err)
	}
	defer learnerClient.Close()

	log.Printf("StartLearner: starting learner on %s (%s)\n", t.Learner.Name, t.Learner.Host)

	// Check if etcd is already running
	checkEtcdCmd := "sudo crictl ps --label io.kubernetes.container.name=etcd -q | head -n 1"
	out, err := learnerClient.Run(checkEtcdCmd)
	if err == nil && strings.TrimSpace(string(out)) != "" {
		return fmt.Errorf("etcd is already running on %s (container ID: %s), please stop it before adding as learner", t.Learner.Host, strings.TrimSpace(string(out)))
	}

	log.Printf("Confirmed etcd is not running on %s (%s)\n", t.Learner.Name, t.Learner.Host)

	if err = t.cleanupLocalDataOnLearner(learnerClient, t.Learner, "/var/lib/etcd/member"); err != nil {
		return fmt.Errorf("failed to cleanup data directory: %w", err)
	}
	log.Printf("Successfully cleaned up etcd data directory on %s (%s)\n", t.Learner.Name, t.Learner.Host)

	initialCluster, err := t.buildInitialClusterString(masterClient)
	if err != nil {
		return fmt.Errorf("failed to build initial-cluster string: %w", err)
	}
	log.Printf("Built initial-cluster string: %s\n", initialCluster)

	localEtcdPath, err := t.updateManifest(learnerClient, initialCluster, "existing")
	if err != nil {
		return fmt.Errorf("failed to update etcd manifest %w, on learner %s (%s)", err, t.Learner.Name, t.Learner.Host)
	}

	if err = learnerClient.Upload(localEtcdPath, "/etc/kubernetes/manifests/etcd.yaml"); err != nil {
		return fmt.Errorf("failed to upload manifest: %w, on learner %s (%s)", err, t.Learner.Name, t.Learner.Host)
	}
	log.Printf("Successfully uploaded etcd manifest on %s (%s)\n", t.Learner.Name, t.Learner.Host)

	containerID, err := t.getEtcdContainerID(learnerClient)
	if err != nil {
		return fmt.Errorf("etcd container did not start: %w", err)
	}

	if err := t.waitForClusterOrMemberStatusHealthy(learnerClient, containerID, false); err != nil {
		return fmt.Errorf("learner health status check failed: %w", err)
	}
	log.Printf("etcd container %s is running on %s (%s), as learner\n", strings.TrimSpace(containerID), t.Learner.Name, t.Learner.Host)

	return nil
}

func (t *AddMemberTask) querryMember(client *ssh.Client, containerID string) (member *etcdserverpb.Member, err error) {
	membersResp, err := t.getMembers(client, containerID)
	if err != nil {
		return nil, err
	}

	learnerMemberName, err := t.Learner.FetchMemberName()
	if err != nil {
		return nil, fmt.Errorf("failed to get learner member name: %w", err)
	}

	for _, member := range membersResp.Members {
		for _, peerURL := range member.PeerURLs {
			if memberIP := extractIPFromPeerURL(peerURL); memberIP != "" && memberIP == t.Learner.Host {
				log.Printf("Member check result for %s (%s): exists=%v, isLearner=%v, found by PeerURL", t.Learner.Host, learnerMemberName, true, member.IsLearner)
				return member, nil
			}
		}
	}

	log.Printf("Member check result for %s (%s): exists=false", t.Learner.Host, learnerMemberName)
	return nil, nil
}

func (t *AddMemberTask) fetchLearnerMembers(client *ssh.Client, containerID string) (members []*etcdserverpb.Member) {
	membersResp, err := t.getMembers(client, containerID)
	if err != nil {
		log.Printf("failed to get members list: %v", err)
		return members
	}

	for _, member := range membersResp.Members {
		if member.IsLearner {
			members = append(members, member)
		}
	}

	return members
}

// extractIPFromPeerURL extracts the IP address from a peer URL
// Format: https://IP:2380 -> IP
func extractIPFromPeerURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		log.Printf("error parsing peerURL: %v, err: %v\n", rawURL, err)
		return ""
	}
	return u.Hostname()
}

func (t *AddMemberTask) getMembers(client *ssh.Client, containerID string) (*clientv3.MemberListResponse, error) {
	out, err := t.execEtcdctl(client, containerID, "member", "list", "-w", "json")
	if err != nil {
		return nil, fmt.Errorf("failed to list members: %w", err)
	}

	var resp clientv3.MemberListResponse
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		return nil, fmt.Errorf("failed to parse member list: %w", err)
	}
	log.Printf("Current cluster has %d members", len(resp.Members))
	return &resp, nil
}

func (t *AddMemberTask) addMemberToCluster(masterClient *ssh.Client, containerID string, isLearner bool) (uint64, error) {
	peerURLs := fmt.Sprintf("https://%s:2380", t.Learner.Host)
	learnerMemberName, err := t.Learner.FetchMemberName()
	if err != nil {
		return 0, fmt.Errorf("failed to get learner member name: %w", err)
	}

	args := []string{"member", "add", learnerMemberName, fmt.Sprintf("--peer-urls=%s", peerURLs), "-w", "json"}
	if isLearner {
		args = append(args, "--learner")
	}

	out, err := t.execEtcdctl(masterClient, containerID, args...)
	if err != nil {
		if strings.Contains(out, "Error: etcdserver: Peer URLs already exists") {
			return 0, nil
		}
		return 0, fmt.Errorf("failed to add member: %w", err)
	}

	var addResponse clientv3.MemberAddResponse
	if err := json.Unmarshal([]byte(out), &addResponse); err != nil {
		return 0, fmt.Errorf("unmarshal adding learner (%s) response failed: %w, output: %s", learnerMemberName, err, out)
	}

	return addResponse.Member.ID, nil
}

func (t *AddMemberTask) getEtcdContainerID(client *ssh.Client) (string, error) {
	waitTask := &WaitForEtcdRunningTask{
		Description:      "Get etcd container ID",
		TimeoutSec:       300,
		RetryIntervalSec: 5,
	}

	containerID, err := waitTask.Run(client)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(containerID), nil
}

func (t *AddMemberTask) waitForClusterOrMemberStatusHealthy(client *ssh.Client, containerID string, cluster bool) error {
	msg := "current member"
	args := []string{"endpoint", "status", "-w", "json"}
	if cluster {
		msg = "cluster"
		args = append(args, "--cluster")
	}
	log.Printf("Waiting for %s to be healthy\n", msg)

	maxRetries := 20
	retryInterval := 5 * time.Second

	for attempt := 0; attempt < maxRetries; attempt++ {
		out, err := t.execEtcdctl(client, containerID, args...)
		if err == nil {
			if validateClusterStatus([]byte(out)) {
				log.Printf("%s is healthy\n", msg)
				return nil
			}
			log.Printf("%s is not healthy: %s, attempt %d/%d\n", msg, out, attempt+1, maxRetries)
		} else {
			log.Printf("%s health check failed: %v, attempt %d/%d\n", msg, err, attempt+1, maxRetries)
		}
		time.Sleep(retryInterval)
	}

	return fmt.Errorf("%s did not become healthy after %d attempts", msg, maxRetries)
}

func (t *AddMemberTask) buildInitialClusterString(masterClient *ssh.Client) (string, error) {
	containerID, err := t.getEtcdContainerID(masterClient)
	if err != nil {
		return "", fmt.Errorf("failed to get etcd container ID: %w", err)
	}

	resp, err := t.getMembers(masterClient, containerID)
	if err != nil {
		return "", err
	}

	learnerMemberName, err := t.Learner.FetchMemberName()
	if err != nil {
		return "", fmt.Errorf("failed to get learner member name: %w", err)
	}

	var parts []string
	for _, member := range resp.Members {
		if len(member.PeerURLs) > 0 {
			name := member.Name
			// Handle case where member name might be empty (newly added learner)
			if name == "" && strings.Contains(member.PeerURLs[0], t.Learner.Host) {
				name = learnerMemberName
			}
			if name != "" {
				parts = append(parts, fmt.Sprintf("%s=%s", name, member.PeerURLs[0]))
			}
		}
	}

	if len(parts) == 0 {
		return "", fmt.Errorf("no members found in cluster")
	}

	return strings.Join(parts, ","), nil
}

func (t *AddMemberTask) promoteLearner(client *ssh.Client, containerID string, MemberID string) error {
	maxRetries := 50
	retryInterval := 5 * time.Second

	log.Printf("Attempting to promote member %s\n", strings.TrimSpace(MemberID))

	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		_, err := t.execEtcdctl(client, containerID, "member", "promote", strings.TrimSpace(MemberID))
		if err == nil {
			log.Printf("Member %s promoted successfully\n", MemberID)
			return nil
		}

		lastErr = err
		if strings.Contains(err.Error(), "can only promote a learner member which is in sync with leader") {
			log.Printf("Learner not in sync yet, retrying (%d/%d)...\n", attempt+1, maxRetries)
		} else {
			log.Printf("Promotion failed: %v, retrying (%d/%d)...\n", err, attempt+1, maxRetries)
		}
		time.Sleep(retryInterval)
	}

	return fmt.Errorf("failed to promote member after %d attempts: %w", maxRetries, lastErr)
}

func (t *AddMemberTask) cleanupLocalDataOnLearner(client *ssh.Client, learner *config.Host, dataDir string) error {
	dataDir = strings.TrimSuffix(dataDir, "/")
	if dataDir == "" {
		dataDir = "/var/lib/etcd/member"
	}

	log.Printf("Checking if etcd data directory exists: %s\n", dataDir)
	if _, err := client.Run(fmt.Sprintf("sudo test -d %s", dataDir)); err != nil {
		log.Printf("Directory %s does not exist, skipping cleanup\n", dataDir)
		return nil
	}

	_, decision, err := cliui.Select(
		fmt.Sprintf("The data directory (%s) must be deleted before member %s can join. Continue?", dataDir, learner.Name),
		[]string{"yes", "no"},
	)
	if err != nil {
		return fmt.Errorf("no selection made: %w", err)
	}

	if decision != "yes" {
		return fmt.Errorf("user did not confirm data cleanup")
	}

	log.Printf("Removing %s\n", dataDir)
	if _, err = client.Run(fmt.Sprintf("sudo -i rm -rf %s", dataDir)); err != nil {
		return fmt.Errorf("failed to remove directory: %w", err)
	}

	log.Printf("etcd local data cleaned successfully\n")
	return nil
}

// execEtcdctl executes etcdctl command inside the container
func (t *AddMemberTask) execEtcdctl(client *ssh.Client, containerID string, args ...string) (string, error) {
	cmd := fmt.Sprintf("sudo crictl exec %s etcdctl --endpoints=https://127.0.0.1:2379 "+
		"--cert /etc/kubernetes/pki/etcd/healthcheck-client.crt "+
		"--key /etc/kubernetes/pki/etcd/healthcheck-client.key "+
		"--cacert /etc/kubernetes/pki/etcd/ca.crt %s",
		strings.TrimSpace(containerID), strings.Join(args, " "))

	cmdTask := &CommandTask{
		Description: "Execute etcdctl command",
		Command:     cmd,
		Check: &Check{
			ExpectedExitCode: 0,
			TimeoutSec:       30,
			RetryIntervalSec: 5,
		},
	}
	return cmdTask.Run(client)
}

type epStatus struct {
	Ep   string                   `json:"Endpoint"`
	Resp *clientv3.StatusResponse `json:"Status"`
}

func validateClusterStatus(output []byte) bool {
	var memberStatusResponse []epStatus
	if err := json.Unmarshal(output, &memberStatusResponse); err != nil {
		log.Printf("Failed to unmarshal etcdctl status JSON: %v\n", err)
		return false
	}

	if len(memberStatusResponse) == 0 {
		return false
	}

	for _, s := range memberStatusResponse {
		if len(s.Resp.Errors) > 0 {
			return false
		}
	}
	return true
}

func (t *AddMemberTask) updateManifest(learnerClient *ssh.Client, initialCluster, initialClusterState string) (string, error) {
	if t.Learner.BackedupManifest == "" {
		return "", fmt.Errorf("backup manifest path not provided in hosts.json")
	}

	localEtcdPath := filepath.Join(os.TempDir(), "etcd-learner.yaml")
	if err := learnerClient.Download(t.Learner.BackedupManifest, localEtcdPath); err != nil {
		return "", fmt.Errorf("failed to download manifest: %w", err)
	}

	manifestYaml, err := os.ReadFile(localEtcdPath)
	if err != nil {
		return "", fmt.Errorf("failed to read manifest: %w", err)
	}

	var pod corev1.Pod
	if err = yaml.Unmarshal(manifestYaml, &pod); err != nil {
		return "", fmt.Errorf("failed to unmarshal manifest: %w", err)
	}

	pod, err = updateEtcdManifestForExistingCluster(&pod, initialCluster, initialClusterState)
	if err != nil {
		return "", fmt.Errorf("failed to update manifest: %w", err)
	}

	updatedYaml, err := yaml.Marshal(&pod)
	if err != nil {
		return "", fmt.Errorf("failed to marshal manifest: %w", err)
	}

	tmpPath := filepath.Join(os.TempDir(), "etcd-updated.yaml")
	if err = os.WriteFile(tmpPath, updatedYaml, 0o644); err != nil {
		return "", fmt.Errorf("failed to write temp manifest: %w", err)
	}

	return tmpPath, nil
}

func updateEtcdManifestForExistingCluster(pod *corev1.Pod, initialCluster, initialClusterState string) (corev1.Pod, error) {
	var containerFound bool

	for i, container := range pod.Spec.Containers {
		if strings.TrimSpace(container.Name) != "etcd" {
			continue
		}
		containerFound = true

		var newCmd []string

		for _, command := range container.Command {
			if strings.HasPrefix(command, "--initial-cluster") {
				continue
			}
			if strings.HasPrefix(command, "--initial-cluster-state") {
				continue
			}
			newCmd = append(newCmd, command)
		}

		if initialCluster != "" {
			newCmd = append(newCmd, fmt.Sprintf("--initial-cluster=%s", initialCluster))
		}
		if initialClusterState != "" {
			newCmd = append(newCmd, fmt.Sprintf("--initial-cluster-state=%s", initialClusterState))
		}

		pod.Spec.Containers[i].Command = newCmd
		break
	}

	if !containerFound {
		return *pod, fmt.Errorf("etcd container not found in manifest")
	}

	return *pod, nil
}
