// Package testutils provides utilities for testing web APIs.
package testutils

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"
)

// ------------------------------------------------------------------------
// Container configuration
// ------------------------------------------------------------------------

// ContainerConfig holds parameters for running a container.
type ContainerConfig struct {
	Image        string            // Docker image name (required)
	Name         string            // Optional container name
	PortMap      map[string]string // hostPort:containerPort, e.g. "8080:80"
	Env          []string          // Environment variables (KEY=value)
	Mounts       []string          // Volume mounts, e.g. "/host:/container"
	Cmd          []string          // Command to run inside container
	Entrypoint   []string          // Override entrypoint
	Network      string            // Network name
	HealthCmd    string            // Command to check health (for WaitForContainerHealthy)
	HealthInterval time.Duration   // Interval between health checks
	PullAlways   bool              // Always pull image before run
}

// ContainerInfo holds information about a running container.
type ContainerInfo struct {
	ID   string
	Name string
}

// ------------------------------------------------------------------------
// Low‑level docker command execution
// ------------------------------------------------------------------------

// docker runs a docker command and returns combined output.
func docker(args ...string) (string, error) {
	cmd := exec.Command("docker", args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	return strings.TrimSpace(out.String()), err
}

// dockerCombinedOutput is like docker but returns stdout/stderr separately.
func dockerCombinedOutput(args ...string) ([]byte, error) {
	cmd := exec.Command("docker", args...)
	return cmd.CombinedOutput()
}

// ------------------------------------------------------------------------
// Image management
// ------------------------------------------------------------------------

// PullImage pulls a Docker image. If force is true, it always pulls;
// otherwise it only pulls if the image is not already present.
func PullImage(image string, force bool) error {
	if !force {
		// Check if image exists locally.
		out, err := docker("image", "inspect", image, "--format", "{{.Id}}")
		if err == nil && out != "" {
			return nil // image already exists
		}
	}
	_, err := docker("pull", image)
	return err
}

// MustPullImage is like PullImage but panics on error.
func MustPullImage(image string, force bool) {
	if err := PullImage(image, force); err != nil {
		panic("testutils: PullImage failed: " + err.Error())
	}
}

// ------------------------------------------------------------------------
// Container lifecycle
// ------------------------------------------------------------------------

// RunContainer starts a new container according to the given config.
// It returns a ContainerInfo and a cleanup function that stops and removes
// the container. If t is non‑nil, the cleanup is automatically registered
// with t.Cleanup.
func RunContainer(t TestingT, cfg ContainerConfig) (*ContainerInfo, func()) {
	t.Helper()

	if cfg.PullAlways {
		MustPullImage(cfg.Image, true)
	}

	args := []string{"run", "-d"}
	if cfg.Name != "" {
		args = append(args, "--name", cfg.Name)
	}
	for hostPort, containerPort := range cfg.PortMap {
		args = append(args, "-p", hostPort+":"+containerPort)
	}
	for _, env := range cfg.Env {
		args = append(args, "-e", env)
	}
	for _, mount := range cfg.Mounts {
		args = append(args, "-v", mount)
	}
	if cfg.Network != "" {
		args = append(args, "--network", cfg.Network)
	}
	if len(cfg.Entrypoint) > 0 {
		args = append(args, "--entrypoint", strings.Join(cfg.Entrypoint, " "))
	}
	args = append(args, cfg.Image)
	if len(cfg.Cmd) > 0 {
		args = append(args, cfg.Cmd...)
	}

	out, err := dockerCombinedOutput(args...)
	if err != nil {
		t.Fatalf("RunContainer: docker run failed: %v\n%s", err, out)
	}
	containerID := strings.TrimSpace(string(out))

	info := &ContainerInfo{
		ID:   containerID,
		Name: cfg.Name,
	}

	cleanup := func() {
		// Stop container (force kill after timeout)
		stopArgs := []string{"stop", "--time", "5", containerID}
		if out, err := dockerCombinedOutput(stopArgs...); err != nil {
			t.Errorf("RunContainer cleanup: stop failed: %v\n%s", err, out)
		}
		rmArgs := []string{"rm", "-v", containerID}
		if out, err := dockerCombinedOutput(rmArgs...); err != nil {
			t.Errorf("RunContainer cleanup: rm failed: %v\n%s", err, out)
		}
	}
	if t != nil {
		t.Cleanup(cleanup)
	}
	return info, cleanup
}

// StopContainer stops a running container (graceful, then kill after timeout).
func StopContainer(containerID string, timeout time.Duration) error {
	_, err := docker("stop", "--time", fmt.Sprint(int(timeout.Seconds())), containerID)
	return err
}

// RemoveContainer removes a stopped container.
func RemoveContainer(containerID string, volumes bool) error {
	args := []string{"rm"}
	if volumes {
		args = append(args, "-v")
	}
	args = append(args, containerID)
	_, err := docker(args...)
	return err
}

// ------------------------------------------------------------------------
// Inspecting containers
// ------------------------------------------------------------------------

// ContainerLogs returns the logs of a container (stdout+stderr).
func ContainerLogs(containerID string, tail int) (string, error) {
	args := []string{"logs"}
	if tail > 0 {
		args = append(args, "--tail", fmt.Sprint(tail))
	}
	args = append(args, containerID)
	return docker(args...)
}

// ContainerIP returns the IP address of the container on the default network.
func ContainerIP(containerID string) (string, error) {
	out, err := docker("inspect", "-f", "{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}", containerID)
	if err != nil {
		return "", err
	}
	return out, nil
}

// ContainerState returns the current state (running, exited, etc.).
func ContainerState(containerID string) (string, error) {
	out, err := docker("inspect", "-f", "{{.State.Status}}", containerID)
	if err != nil {
		return "", err
	}
	return out, nil
}

// ------------------------------------------------------------------------
// Waiting for container readiness
// ------------------------------------------------------------------------

// WaitForContainerLog waits for a substring to appear in the container logs.
// It polls every 100ms until the timeout is reached.
func WaitForContainerLog(containerID string, substr string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		logs, err := ContainerLogs(containerID, 50) // last 50 lines
		if err != nil {
			return err
		}
		if strings.Contains(logs, substr) {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timeout waiting for log %q", substr)
		}
		<-ticker.C
	}
}

// WaitForContainerHealthy waits for the container to become healthy (if it has a health check).
// It uses `docker inspect` to check health status.
func WaitForContainerHealthy(containerID string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		status, err := docker("inspect", "-f", "{{.State.Health.Status}}", containerID)
		if err != nil {
			// If health status is not available, maybe container has no health check.
			// Fall back to running state.
			state, err2 := ContainerState(containerID)
			if err2 != nil {
				return err
			}
			if state == "running" {
				return nil
			}
		}
		if status == "healthy" {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timeout waiting for healthy status (last status: %s)", status)
		}
		<-ticker.C
	}
}

// ------------------------------------------------------------------------
// Exec inside container
// ------------------------------------------------------------------------

// ExecContainer runs a command inside a running container and returns stdout/stderr.
func ExecContainer(containerID string, cmd ...string) (string, error) {
	args := append([]string{"exec", containerID}, cmd...)
	return docker(args...)
}

// ExecContainerCombined is like ExecContainer but returns combined output.
func ExecContainerCombined(containerID string, cmd ...string) ([]byte, error) {
	args := append([]string{"exec", containerID}, cmd...)
	return dockerCombinedOutput(args...)
}

// ------------------------------------------------------------------------
// Must variants – panic on error
// ------------------------------------------------------------------------

// MustRunContainer is like RunContainer but panics on error.
func MustRunContainer(t TestingT, cfg ContainerConfig) (*ContainerInfo, func()) {
	t.Helper()
	info, cleanup := RunContainer(t, cfg)
	if info == nil {
		panic("RunContainer failed")
	}
	return info, cleanup
}

// MustContainerLogs calls ContainerLogs and panics on error.
func MustContainerLogs(containerID string, tail int) string {
	logs, err := ContainerLogs(containerID, tail)
	if err != nil {
		panic("testutils: ContainerLogs failed: " + err.Error())
	}
	return logs
}

// MustContainerIP calls ContainerIP and panics on error.
func MustContainerIP(containerID string) string {
	ip, err := ContainerIP(containerID)
	if err != nil {
		panic("testutils: ContainerIP failed: " + err.Error())
	}
	return ip
}