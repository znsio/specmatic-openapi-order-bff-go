package main_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/docker/go-connections/nat"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"github.com/znsio/specmatic-order-bff-go/internal/com/store/order/bff/config"
)

var authToken = "API-TOKEN-SPEC"

type testEnvironment struct {
	ctx                      context.Context
	domainServiceContainer   testcontainers.Container
	domainServiceDynamicPort string
	kafkaServiceContainer    testcontainers.Container
	kafkaServiceDynamicPort  string
	bffServiceContainer      testcontainers.Container
	bffServiceDynamicPort    string
	config                   *config.Config
}

func TestIntegration(t *testing.T) {
	env := setUpEnv(t)

	// setUp (start domain service stub with specmatic-grpc and bff server in container)
	setUp(t, env)

	// RUN (run specmatic-grpc test in container)
	runTests(t, env)

	// TEAR DOWN
	defer tearDown(t, env)
}

func setUpEnv(t *testing.T) *testEnvironment {
	config, err := config.LoadConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	return &testEnvironment{
		ctx:    context.Background(),
		config: config,
	}
}

func setUp(t *testing.T, env *testEnvironment) {
	var err error

	printHeader(t, 1, "Starting Domain Service")
	env.domainServiceContainer, env.domainServiceDynamicPort, err = startDomainService(t, env)
	if err != nil {
		t.Fatalf("could not start domain service container: %v", err)
	}

	printHeader(t, 2, "Starting BFF Service")
	env.bffServiceContainer, env.bffServiceDynamicPort, err = startBFFService(t, env)
	if err != nil {
		t.Fatalf("could not start bff service container: %v", err)
	}
}

func runTests(t *testing.T, env *testEnvironment) {
	printHeader(t, 3, "Starting tests")
	testLogs, err := runTestContainer(env)
	if err != nil {
		t.Fatalf("Could not run test container: %s", err)
	}

	// Print test outcomes
	t.Log("Test Results:")
	t.Log(testLogs)
}

func tearDown(t *testing.T, env *testEnvironment) {
	if env.bffServiceContainer != nil {
		if err := env.bffServiceContainer.Terminate(env.ctx); err != nil {
			t.Logf("Failed to terminate BFF container: %v", err)
		}
	}
	if env.domainServiceContainer != nil {
		if err := env.domainServiceContainer.Terminate(env.ctx); err != nil {
			t.Logf("Failed to terminate stub container: %v", err)
		}
	}
}

func startDomainService(t *testing.T, env *testEnvironment) (testcontainers.Container, string, error) {
	pwd, err := os.Getwd()
	if err != nil {
		return nil, "", fmt.Errorf("Error getting current directory: %v", err)

	}

	port, err := nat.NewPort("tcp", env.config.BackendPort)
	if err != nil {
		return nil, "", fmt.Errorf("invalid port number: %w", err)
	}

	req := testcontainers.ContainerRequest{
		Image:        "znsio/specmatic",
		ExposedPorts: []string{port.Port() + "/tcp"},
		Cmd:          []string{"stub"},
		Mounts: testcontainers.Mounts(
			testcontainers.BindMount(filepath.Join(pwd, "specmatic.json"), "/usr/src/app/specmatic.json"),
		),
		WaitingFor: wait.ForLog("Stub server is running"),
	}

	t.Log("BFF Container created")

	backendC, err := testcontainers.GenericContainer(env.ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, "", err
	}

	mappedPort, err := backendC.MappedPort(env.ctx, port)
	if err != nil {
		return nil, "", err
	}

	return backendC, mappedPort.Port(), nil
}

func startKafkaMock(env *testEnvironment) (testcontainers.Container, string, error) {
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

	kafkaC, err := testcontainers.GenericContainer(env.ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		fmt.Printf("Error starting Kafka mock container: %v", err)
	}

	mappedPort, err := kafkaC.MappedPort(env.ctx, "9092")
	if err != nil {
		fmt.Printf("Error getting mapped port for Kafka mock: %v", err)
	}

	return kafkaC, mappedPort.Port(), nil
}

func startBFFService(t *testing.T, env *testEnvironment) (testcontainers.Container, string, error) {
	dockerfilePath := "Dockerfile"

	bffImageName := "specmatic-order-bff-go"
	buildCmd := exec.Command("docker", "build", "-t", bffImageName, "-f", dockerfilePath, "--build-arg", fmt.Sprintf("PORT=%s", env.config.BFFServerPort), ".")

	if err := buildCmd.Run(); err != nil {
		return nil, "", fmt.Errorf("could not build BFF image: %w", err)
	}

	port, err := nat.NewPort("tcp", env.config.BFFServerPort)
	if err != nil {
		return nil, "", fmt.Errorf("invalid port number: %w", err)
	}

	req := testcontainers.ContainerRequest{
		Image: bffImageName,
		Env: map[string]string{
			"DOMAIN_SERVER_PORT": env.domainServiceDynamicPort,
			"DOMAIN_SERVER_HOST": "host.docker.internal",
		},
		ExposedPorts: []string{port.Port() + "/tcp"},
		WaitingFor:   wait.ForLog("Listening and serving"),
	}

	t.Log("BFF Container created")

	bffContainer, err := testcontainers.GenericContainer(env.ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, "", err
	}

	dynamicBffPort, err := bffContainer.MappedPort(env.ctx, port)
	if err != nil {
		return nil, "", err
	}

	return bffContainer, dynamicBffPort.Port(), nil
}

func runTestContainer(env *testEnvironment) (string, error) {
	pwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("Error getting current directory: %v", err)
	}

	bffPortInt, err := strconv.Atoi(env.bffServiceDynamicPort)
	if err != nil {
		return "", fmt.Errorf("invalid port number: %w", err)
	}

	req := testcontainers.ContainerRequest{
		Image: "znsio/specmatic",
		Env: map[string]string{
			"SPECMATIC_GENERATIVE_TESTS": "true",
		},
		Cmd: []string{"test", fmt.Sprintf("--port=%d", bffPortInt), "--host=host.docker.internal"},
		Mounts: testcontainers.Mounts(
			testcontainers.BindMount(filepath.Join(pwd, "specmatic.json"), "/usr/src/app/specmatic.json"),
		),
		WaitingFor: wait.ForLog("Passed Tests:"),
	}

	testContainer, err := testcontainers.GenericContainer(env.ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return "", err
	}
	// Terminate test container post completion
	defer testContainer.Terminate(env.ctx)

	// Streaming testing logs to terminal
	logReader, err := testContainer.Logs(env.ctx)
	if err != nil {
		return "", err
	}
	defer logReader.Close()

	var buf bytes.Buffer
	_, err = io.Copy(&buf, logReader)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}

func printHeader(t *testing.T, stepNum int, title string) {
	t.Log("")
	t.Logf("======== STEP %d =========", stepNum)
	t.Log(title)
	t.Log("=========================")
	t.Log("")
}
