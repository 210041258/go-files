// Package testutils provides container management for integration tests.
// It uses the local Docker daemon via the `docker` CLI (no heavy SDK).
package testutils

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Container represents a running Docker container.
type Container struct {
	ID        string
	Image     string
	Name      string
	HostPorts map[int]int // containerPort -> hostPort
	Cmd       []string
	Env       []string
}

// ContainerOption configures a container before start.
type ContainerOption func(*Container) error

// WithName sets the container name.
func WithName(name string) ContainerOption {
	return func(c *Container) error {
		c.Name = name
		return nil
	}
}

// WithPort publishes a container port to a random (or specific) host port.
// If hostPort is 0, Docker will assign a random free port.
func WithPort(containerPort, hostPort int) ContainerOption {
	return func(c *Container) error {
		if c.HostPorts == nil {
			c.HostPorts = make(map[int]int)
		}
		c.HostPorts[containerPort] = hostPort
		return nil
	}
}

// WithCmd sets the command to run in the container.
func WithCmd(cmd ...string) ContainerOption {
	return func(c *Container) error {
		c.Cmd = cmd
		return nil
	}
}

// WithEnv sets environment variables in the form "KEY=value".
func WithEnv(env ...string) ContainerOption {
	return func(c *Container) error {
		c.Env = append(c.Env, env...)
		return nil
	}
}

// RunContainer starts a new container using the `docker run` command.
// It returns a Container object and a cleanup function that stops and removes the container.
// The container is started detached and automatically removed when the test ends.
// If the image is not present locally, it will be pulled (this may take time).
//
// Example:
//
//	func TestWithPostgres(t *testing.T) {
//		pg, cleanup := testutils.RunContainer(t, "postgres:15",
//			testutils.WithPort(5432, 0),
//			testutils.WithEnv("POSTGRES_PASSWORD=secret"),
//		)
//		defer cleanup()
//
//		hostPort := pg.GetPort(5432)
//		dsn := fmt.Sprintf("postgres://postgres:secret@localhost:%d/postgres?sslmode=disable", hostPort)
//		// ... use DSN
//	}
func RunContainer(t *testing.T, image string, opts ...ContainerOption) (*Container, func()) {
	t.Helper()

	c := &Container{
		Image:     image,
		HostPorts: make(map[int]int),
	}

	// Apply options
	for _, opt := range opts {
		if err := opt(c); err != nil {
			t.Fatalf("testutils: container option error: %v", err)
		}
	}

	// Build `docker run` arguments
	args := []string{"run", "-d", "--rm"}
	if c.Name != "" {
		args = append(args, "--name", c.Name)
	}
	for containerPort, hostPort := range c.HostPorts {
		pub := fmt.Sprintf("%d:%d", hostPort, containerPort)
		if hostPort == 0 {
			pub = fmt.Sprintf("%d", containerPort) // Docker will assign
		}
		args = append(args, "-p", pub)
	}
	for _, env := range c.Env {
		args = append(args, "-e", env)
	}
	args = append(args, image)
	if len(c.Cmd) > 0 {
		args = append(args, c.Cmd...)
	}

	// Execute docker run
	out, err := exec.Command("docker", args...).CombinedOutput()
	if err != nil {
		t.Fatalf("testutils: docker run failed: %v\noutput: %s", err, out)
	}
	c.ID = strings.TrimSpace(string(out))
	t.Logf("testutils: started container %s (%s)", c.ID[:12], image)

	// Wait a moment for container to be ready
	time.Sleep(2 * time.Second)

	// Inspect container to get actual port mappings (for hostPort = 0)
	if err := c.inspect(); err != nil {
		t.Fatalf("testutils: inspect container: %v", err)
	}

	// Return cleanup function
	cleanup := func() {
		t.Helper()
		if err := c.Stop(); err != nil {
			t.Logf("testutils: container stop: %v", err)
		}
	}
	return c, cleanup
}

// Stop terminates the container.
func (c *Container) Stop() error {
	if c.ID == "" {
		return nil
	}
	out, err := exec.Command("docker", "stop", c.ID).CombinedOutput()
	if err != nil {
		return fmt.Errorf("stop failed: %w\noutput: %s", err, out)
	}
	return nil
}

// GetPort returns the host port that maps to the given container port.
// Panics if the port mapping is not known.
func (c *Container) GetPort(containerPort int) int {
	hostPort, ok := c.HostPorts[containerPort]
	if !ok {
		panic(fmt.Sprintf("testutils: container port %d not mapped", containerPort))
	}
	return hostPort
}

// inspect fetches detailed container info via `docker inspect` and updates
// HostPorts with the actual host ports assigned by Docker.
func (c *Container) inspect() error {
	out, err := exec.Command("docker", "inspect", c.ID).Output()
	if err != nil {
		return fmt.Errorf("inspect: %w", err)
	}
	var data []struct {
		NetworkSettings struct {
			Ports map[string][]struct {
				HostIP   string `json:"HostIp"`
				HostPort string `json:"HostPort"`
			} `json:"Ports"`
		} `json:"NetworkSettings"`
	}
	if err := json.Unmarshal(out, &data); err != nil {
		return fmt.Errorf("parse inspect: %w", err)
	}
	if len(data) == 0 {
		return fmt.Errorf("no inspect data")
	}
	// Update port mappings
	for containerPort, hostPort := range c.HostPorts {
		key := fmt.Sprintf("%d/tcp", containerPort)
		if bindings, ok := data[0].NetworkSettings.Ports[key]; ok && len(bindings) > 0 {
			if p, err := strconv.Atoi(bindings[0].HostPort); err == nil {
				c.HostPorts[containerPort] = p
			}
		}
	}
	return nil
}

// Exec runs a command inside the container and returns stdout, stderr, and error.
func (c *Container) Exec(cmd ...string) (stdout, stderr []byte, err error) {
	args := append([]string{"exec", c.ID}, cmd...)
	cmdExec := exec.Command("docker", args...)
	var outBuf, errBuf bytes.Buffer
	cmdExec.Stdout = &outBuf
	cmdExec.Stderr = &errBuf
	err = cmdExec.Run()
	return outBuf.Bytes(), errBuf.Bytes(), err
}

// WaitForPort blocks until the given container port is listening on its mapped host port,
// or until the timeout expires. Useful for services that need startup time.
func (c *Container) WaitForPort(ctx context.Context, containerPort int) error {
	hostPort := c.GetPort(containerPort)
	address := net.JoinHostPort("localhost", strconv.Itoa(hostPort))
	for {
		conn, err := net.DialTimeout("tcp", address, 500*time.Millisecond)
		if err == nil {
			conn.Close()
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(200 * time.Millisecond):
		}
	}
}
