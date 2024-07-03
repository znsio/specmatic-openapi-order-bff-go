package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"github.com/znsio/specmatic-order-bff-go/internal/config"
)

func main() {
	ctx := context.Background()

	// Load configuration from config.yaml
	if err := config.LoadConfig(); err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Access configuration
	cfg := config.GetConfig()

	fmt.Println("=== 1 === >> Starting backend stub")
	// STEP 1 :: Start backend stub
	backendC, backendPort := startBackendStub(ctx)
	defer backendC.Terminate(ctx)

	fmt.Println("=== 1.2 === >> Starting Kafka stub")
	// STEP 2 :: Start Kafka mock
	kafkaC, kafkaPort := startKafkaMock(ctx)
	defer kafkaC.Terminate(ctx)

	// Update configuration
	config.SetBackendPort(backendPort)
	config.SetKafkaPort(kafkaPort)

	fmt.Println("=== 2 === >> Starting BFF on host")
	// STEP 3 :: Start BFF service (assuming it's started separately on the host)
	go startBFFService(ctx, cfg)

	fmt.Println("=== 3 === >> before sleept")
	// Give some time for the BFF service to start
	time.Sleep(5 * time.Second)

	fmt.Println("=== 4 === >> after sleep, about to run test")
	// STEP 4 (final step) :: Run tests
	err := runTestContainer(ctx, backendPort, kafkaPort)
	if err != nil {
		fmt.Printf("Error running test container: %v", err)
	}
	fmt.Println("=== 5 === xxxxx all DONE.")
}

func startBackendStub(ctx context.Context) (testcontainers.Container, string) {
	pwd, err := os.Getwd()
	if err != nil {
		log.Fatalf("Error getting current directory: %v", err)
	}

	req := testcontainers.ContainerRequest{
		Image:        "znsio/specmatic",
		ExposedPorts: []string{"9000/tcp"},
		Cmd:          []string{"stub", "--config=/specmatic.json"},
		Mounts: testcontainers.Mounts(
			testcontainers.BindMount(filepath.Join(pwd, "./specmatic.json"), "/specmatic.json"),
		),
		WaitingFor: wait.ForLog("Stub server is running"),
	}

	backendC, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		fmt.Printf("Error starting backend stub container: %v", err)
	}

	mappedPort, err := backendC.MappedPort(ctx, "9000")
	if err != nil {
		fmt.Printf("Error getting mapped port for backend stub: %v", err)
	}

	fmt.Println("=== 1.1 === >> existing from start backnd stub func")
	return backendC, mappedPort.Port()
}

func startKafkaMock(ctx context.Context) (testcontainers.Container, string) {
	pwd, err := os.Getwd()
	if err != nil {
		fmt.Printf("Error getting current directory: %v", err)
	}

	req := testcontainers.ContainerRequest{
		Image:        "kafka-stub:latest", // Replace with actual Kafka mock image
		ExposedPorts: []string{"9092/tcp"},
		Cmd:          []string{"--config=/specmatic.json"},
		Mounts: testcontainers.Mounts(
			testcontainers.BindMount(filepath.Join(pwd, "./specmatic.json"), "/specmatic.json"),
		),
		WaitingFor: wait.ForListeningPort("9092/tcp"),
	}

	kafkaC, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		fmt.Printf("Error starting Kafka mock container: %v", err)
	}

	mappedPort, err := kafkaC.MappedPort(ctx, "9092")
	if err != nil {
		fmt.Printf("Error getting mapped port for Kafka mock: %v", err)
	}

	return kafkaC, mappedPort.Port()
}

func startBFFService(ctx context.Context, cfg *config.Config) {

	cmd := exec.CommandContext(ctx, "go", "run", "cmd/main.go")

	/*
	* Setting dynamically allocated PORT for the backend service, via test controllers.
	* this will be needed by BFF, that will run on a seperate thread.
	 */
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("BACKEND_PORT=%s", cfg.BackendPort),
		fmt.Sprintf("KAFKA_PORT=%s", cfg.BackendPort),
	)

	err := cmd.Start()
	if err != nil {
		log.Fatalf("Failed to start BFF service: %v", err)
	}

	fmt.Println("=== 2.2 === >> existing from Starting BFF")
}

func runTestContainer(ctx context.Context, backendPort, kafkaPort string) error {
	pwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("error getting current directory: %v", err)
	}

	req := testcontainers.ContainerRequest{
		Image: "znsio/specmatic",
		Cmd:   []string{"test", "--host", "host.docker.internal", "--port", "8080", "--config=/specmatic.json"},
		Mounts: testcontainers.Mounts(
			testcontainers.BindMount(filepath.Join(pwd, "specmatic.json"), "/specmatic.json"),
		),
		Env: map[string]string{
			"BACKEND_PORT": backendPort,
			"KAFKA_PORT":   kafkaPort,
		},
		WaitingFor:  wait.ForLog("Tests completed"),
		NetworkMode: "host",
	}

	testC, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return fmt.Errorf("error starting test container: %v", err)
	}
	defer testC.Terminate(ctx)

	fmt.Println("=== 4.1=== >> setting log reader")
	// Stream logs from the container
	logReader, err := testC.Logs(ctx)
	if err != nil {
		return fmt.Errorf("error getting container logs: %v", err)
	}
	defer logReader.Close()

	go func() {
		_, err := io.Copy(os.Stdout, logReader)
		if err != nil && err != io.EOF {
			fmt.Printf("Error streaming logs: %v", err)
		}
	}()

	fmt.Println("=== 4.2 === >> after log rader")
	// Wait for the container to finish
	waitCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	fmt.Println("=== 4.3 === >> starting log reader")
	for {
		select {
		case <-waitCtx.Done():
			return fmt.Errorf("timeout waiting for container to finish")
		default:
			state, err := testC.State(ctx)
			if err != nil {
				return fmt.Errorf("error getting container state: %v", err)
			}
			if !state.Running {
				if state.ExitCode != 0 {
					return fmt.Errorf("tests failed with exit code: %d", state.ExitCode)
				}
				return nil // Container finished successfully
			}
			time.Sleep(1 * time.Second) // Wait before checking again
		}
	}
}
