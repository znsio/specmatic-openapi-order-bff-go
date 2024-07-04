package main_test

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"github.com/znsio/specmatic-order-bff-go/internal/api"
	"github.com/znsio/specmatic-order-bff-go/internal/config"
	"github.com/znsio/specmatic-order-bff-go/internal/services"
)

var authToken = "API-TOKEN-SPEC"

func TestIntegration(t *testing.T) {
	ctx := context.Background()

	// Load configuration from config.yaml
	if err := config.LoadConfig("../../"); err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// STEP 1 :: Start backend stub
	printHeader(1, "Start Backend Stub")
	backendC, backendPort := startBackendStub(ctx)
	defer backendC.Terminate(ctx)

	// STEP 2 :: Start Kafka mock
	printHeader(2, "Start Kafka Mock")
	// kafkaC, kafkaPort := startKafkaMock(ctx)
	// defer kafkaC.Terminate(ctx)

	// Update configuration, with dynamic port provided by test containers
	config.SetBackendPort(backendPort)
	// config.SetKafkaPort(kafkaPort)

	// STEP 3 :: Start BFF service (assuming it's started separately on the host)
	printHeader(3, "Start BFF Service")
	serverCtx, serverCancel := context.WithCancel(ctx)
	serverUp := make(chan struct{})
	serverDone := make(chan struct{})

	go startBFFServer(serverCtx, serverDone, serverUp)

	// Wait for the BFF server to be up and running, max for 10 seconds
	select {
	case <-serverUp:
		fmt.Println("BFF server is up and running")
	case <-time.After(10 * time.Second):
		t.Fatal("Timeout waiting for BFF server to start")
	}

	// STEP 4 (final step) :: Run tests
	printHeader(4, "Start TEST")
	err := runTestContainer(ctx, backendPort, "9092")
	if err != nil {
		fmt.Printf("Error running test container: %v", err)
	}

	// Signal the BFF server to shutdown
	// STEP 5 :: Terminate BFF server
	printHeader(5, "Begin Tear Down, tests completed.")
	serverCancel()
	select {
	case <-serverDone:
		fmt.Println("BFF server has been terminated")
	case <-time.After(10 * time.Second):
		t.Fatal("Timeout waiting for BFF server to terminate")
	}
}

func startBFFServer(ctx context.Context, done chan<- struct{}, up chan<- struct{}) {
	// Access configuration
	cfg := config.GetConfig()

	backendURL := url.URL{
		Scheme: "http",
		Host:   cfg.BackendHost + ":" + cfg.BackendPort,
	}

	backendService := services.NewBackendService(backendURL.String(), authToken)

	// setup router and start server
	r := api.SetupRouter(backendService)
	// create a new http.Server
	srv := &http.Server{
		Addr:    ":" + cfg.ServerPort,
		Handler: r,
	}

	// channel to signal when the server is ready
	ready := make(chan struct{})

	go func() {
		// signal that the server is ready to accept connections
		close(ready)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	// wait for the server to be ready
	<-ready
	// signal that the server is up
	close(up)

	// wait for cancel signal
	<-ctx.Done()

	// shutdown the server with a timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("Server Shutdown Failed:%+v", err)
	}

	// signal that the server has shut down
	close(done)
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
			testcontainers.BindMount(filepath.Join(pwd, "../../specmatic.json"), "/specmatic.json"),
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

	return backendC, mappedPort.Port()
}

func startKafkaMock(ctx context.Context) (testcontainers.Container, string) {
	pwd, err := os.Getwd()
	if err != nil {
		fmt.Printf("Error getting current directory: %v", err)
	}

	req := testcontainers.ContainerRequest{
		Image:        "znsio/specmatic-kafka:0.22.5-TRIAL",
		ExposedPorts: []string{"9092/tcp"},
		Cmd:          []string{"--config=/specmatic.json"}, // TODO: Switch to YAML
		Mounts: testcontainers.Mounts(
			testcontainers.BindMount(filepath.Join(pwd, "../../specmatic.json"), "/specmatic.json"),
		),
		WaitingFor: wait.ForLog("Listening on topics: (product-queries)").WithStartupTimeout(2 * time.Minute),
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

func runTestContainer(ctx context.Context, backendPort, kafkaPort string) error {
	pwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("error getting current directory: %v", err)
	}

	req := testcontainers.ContainerRequest{
		Image: "znsio/specmatic",
		Cmd:   []string{"test", "--host", "host.docker.internal", "--port", "8080", "--config=/specmatic.json"},
		Mounts: testcontainers.Mounts(
			testcontainers.BindMount(filepath.Join(pwd, "../../specmatic.json"), "/specmatic.json"),
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

	// Wait for the container to finish
	waitCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

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

func printHeader(stepNum int, title string) {
	fmt.Println("")
	fmt.Printf("======== STEP %d =========\n", stepNum)
	fmt.Println(title)
	fmt.Println("=========================")
	fmt.Println("")
}
