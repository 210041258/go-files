package testutils

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"
)

// DockerManager handles Docker Compose operations.
type DockerManager struct {
	config     *config.TestConfig
	dockerCfg  config.DockerConfig
	logger     *test.TestLogger
	cancelFunc context.CancelFunc
}

// NewDockerManager creates a new Docker manager instance.
func NewDockerManager(cfg *config.TestConfig, logger *test.TestLogger) (*DockerManager, error) {
	if cfg == nil {
		return nil, fmt.Errorf("test config cannot be nil")
	}
	if cfg.DockerConfig.ComposePath == "" {
		return nil, fmt.Errorf("docker compose path not found")
	}

	// Ensure the directory exists
	if err := os.MkdirAll(cfg.DockerConfig.ComposePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create docker compose directory: %w", err)
	}

	return &DockerManager{
		config:    cfg,
		dockerCfg: cfg.DockerConfig,
		logger:    logger,
	}, nil
}

// Start launches Docker containers and waits for services to be ready.
// It accepts a context for cancellation support.
func (dm *DockerManager) Start(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	dm.cancelFunc = cancel

	args := []string{"compose", "-f", dm.dockerCfg.ComposeFile}
	if dm.dockerCfg.Network != "" {
		args = append(args, "--project-name", dm.dockerCfg.Network)
	}

	args = append(args, "up", "-d")
	if dm.dockerCfg.Build {
		args = append(args, "--build")
	}
	if dm.dockerCfg.ForceRecreate {
		args = append(args, "--force-recreate")
	}
	if dm.dockerCfg.RemoveOrphans {
		args = append(args, "--remove-orphans")
	}

	dm.logger.Info("Starting Docker containers", "composeFile", dm.dockerCfg.ComposeFile)

	// Create command with context to allow cancellation
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Dir = dm.dockerCfg.ComposePath
	cmd.Stdout = dm.logger.Writer()
	cmd.Stderr = dm.logger.Writer()

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to start docker compose: %w", err)
	}

	return dm.waitForServices(ctx)
}

// Stop terminates Docker containers and cleans up resources.
func (dm *DockerManager) Stop() error {
	// Cancel any ongoing wait loops if running
	if dm.cancelFunc != nil {
		dm.cancelFunc()
	}

	args := []string{"compose", "-f", dm.dockerCfg.ComposeFile, "down"}
	if dm.dockerCfg.RemoveOrphans {
		args = append(args, "--remove-orphans")
	}
	if dm.dockerCfg.RemoveVolumes {
		args = append(args, "--volumes")
	}

	dm.logger.Info("Stopping Docker containers")

	cmd := exec.Command("docker", args...)
	cmd.Dir = dm.dockerCfg.ComposePath
	cmd.Stdout = dm.logger.Writer()
	cmd.Stderr = dm.logger.Writer()

	return cmd.Run()
}

// waitForServices verifies that all required services are accessible.
func (dm *DockerManager) waitForServices(ctx context.Context) error {
	if len(dm.dockerCfg.Services) == 0 {
		dm.logger.Info("No services defined in config, skipping wait")
		return nil
	}

	for _, service := range dm.dockerCfg.Services {
		dm.logger.Debug("Waiting for service", "service", service)

		// Check for context cancellation before starting to wait
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if err := dm.waitForServicePort(ctx, service, dm.dockerCfg.Timeout); err != nil {
				return fmt.Errorf("service %s not ready: %w", service, err)
			}
		}
	}
	return nil
}

// waitForServicePort verifies TCP connectivity to a service.
// It is private to the package as it is an implementation detail of the manager.
func (dm *DockerManager) waitForServicePort(ctx context.Context, service string, timeout time.Duration) error {
	parts := strings.Split(service, ":")
	if len(parts) != 2 {
		return fmt.Errorf("invalid service format: %s, expected 'host:port'", service)
	}

	host, port := parts[0], parts[1]
	address := net.JoinHostPort(host, port)

	deadline := time.Now().Add(timeout)

	ticker := time.NewTicker(dm.config.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			conn, err := net.DialTimeout("tcp", address, 2*time.Second)
			if err == nil {
				conn.Close()
				dm.logger.Debug("Service port accessible", "service", service)
				return nil
			}

			if time.Now().After(deadline) {
				return fmt.Errorf("service %s not accessible after %v", service, timeout)
			}
		}
	}
}
