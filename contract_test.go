package main_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/docker/go-connections/nat"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/network"
	"github.com/testcontainers/testcontainers-go/wait"
	"github.com/znsio/specmatic-order-bff-go/internal/com/store/order/bff/config"
)

var authToken = "API-TOKEN-SPEC"

type testEnvironment struct {
	ctx                      context.Context
	bffTestNetwork           *testcontainers.DockerNetwork
	domainServiceContainer   testcontainers.Container
	domainServiceDynamicPort string
	kafkaServiceContainer    testcontainers.Container
	kafkaServiceDynamicPort  string
	kafkaAPIPort             string
	bffServiceContainer      testcontainers.Container
	bffServiceDynamicPort    string
	expectedMessageCount     int
	config                   *config.Config
}

func TestContract(t *testing.T) {
	env := setUpEnv(t)

	setUp(t, env)

	runTests(t, env)

	defer tearDown(t, env)
}

func setUpEnv(t *testing.T) *testEnvironment {
	config, err := config.LoadConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// create a context
	ctx := context.Background()

	// Create a net and store in env.
	newNetwork, err := network.New(ctx)
	if err != nil {
		t.Fatal(err)
	}

	return &testEnvironment{
		ctx:                  ctx,
		config:               config,
		bffTestNetwork:       newNetwork,
		expectedMessageCount: 3,
	}
}

func setUp(t *testing.T, env *testEnvironment) {
	var err error

	printHeader(t, 1, "Starting Domain Service")
	env.domainServiceContainer, env.domainServiceDynamicPort, err = startDomainService(t, env)
	if err != nil {
		t.Fatalf("could not start domain service container: %v", err)
	}

	printHeader(t, 2, "Starting Kafka Service")
	env.kafkaServiceContainer, env.kafkaServiceDynamicPort, err = startKafkaMock(t, env)
	if err != nil {
		t.Fatalf("could not start Kafka service container: %v", err)
	}

	printHeader(t, 3, "Starting BFF Service")
	env.bffServiceContainer, env.bffServiceDynamicPort, err = startBFFService(t, env)
	if err != nil {
		t.Fatalf("could not start bff service container: %v", err)
	}

}

func runTests(t *testing.T, env *testEnvironment) {
	printHeader(t, 4, "Starting tests")
	testLogs, err := runTestContainer(env)

	if (err != nil) && !strings.Contains(err.Error(), "code 0") {
		t.Logf("Could not run test container: %s", err)
		t.Fail()
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

	if env.kafkaServiceContainer != nil {
		verifyKafkaExpectations(env)
		if err := env.kafkaServiceContainer.Terminate(env.ctx); err != nil {
			t.Logf("Failed to terminate Kafka container: %v", err)
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
		Networks: []string{
			env.bffTestNetwork.Name,
		},
		Cmd: []string{"stub"},
		Mounts: testcontainers.Mounts(
			testcontainers.BindMount(filepath.Join(pwd, "specmatic.yaml"), "/usr/src/app/specmatic.yaml"),
		),
		NetworkAliases: map[string][]string{
			env.bffTestNetwork.Name: {env.config.BackendHost},
		},
		WaitingFor: wait.ForLog("Stub server is running"),
	}

	t.Log("Domain Container created")

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

func startKafkaMock(t *testing.T, env *testEnvironment) (testcontainers.Container, string, error) {
	pwd, err := os.Getwd()
	if err != nil {
		return nil, "", fmt.Errorf("Error getting current directory: %v", err)

	}

	port, err := nat.NewPort("tcp", env.config.KafkaPort)
	if err != nil {
		return nil, "", fmt.Errorf("invalid port number: %w", err)
	}

	networkName := env.bffTestNetwork.Name

	req := testcontainers.ContainerRequest{
		Image:        "znsio/specmatic-kafka-trial",
		ExposedPorts: []string{port.Port() + "/tcp"},
		Networks: []string{
			networkName,
		},
		NetworkAliases: map[string][]string{
			networkName: {env.config.KafkaHost},
		},
		Cmd: []string{"--config=/specmatic.yaml"},
		Mounts: testcontainers.Mounts(
			testcontainers.BindMount(filepath.Join(pwd, "specmatic.yaml"), "/specmatic.yaml"),
		),
		Env: map[string]string{
			"KAFKA_EXTERNAL_HOST": env.config.KafkaHost,
			"KAFKA_EXTERNAL_PORT": env.config.KafkaPort,
		},
		WaitingFor: wait.ForLog("Listening on topics: (product-queries)").WithStartupTimeout(2 * time.Minute),
	}

	kafkaC, err := testcontainers.GenericContainer(env.ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		fmt.Printf("Error starting Kafka mock container: %v", err)
	}

	mappedPort, err := kafkaC.MappedPort(env.ctx, port)
	if err != nil {
		fmt.Printf("Error getting mapped port for Kafka mock: %v", err)
	}

	// Get the API server port
	apiPort, err := getAPIServerPort(env.ctx, kafkaC)
	if err != nil {
		fmt.Printf("Error getting API server port: %v", err)
	} else {
		env.kafkaAPIPort = apiPort
	}

	if err := setKafkaExpectations(env, kafkaC); err != nil {
		return nil, "", fmt.Errorf("failed to set Kafka expectations: %v", err)
	}

	return kafkaC, mappedPort.Port(), nil
}

func startBFFService(t *testing.T, env *testEnvironment) (testcontainers.Container, string, error) {

	port, err := nat.NewPort("tcp", env.config.BFFServerPort)
	if err != nil {
		return nil, "", fmt.Errorf("invalid port number: %w", err)
	}

	networkName := env.bffTestNetwork.Name
	dockerfilePath := "Dockerfile"
	contextPath := "."

	req := testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:    contextPath,
			Dockerfile: dockerfilePath,
		},
		Env: map[string]string{
			"DOMAIN_SERVER_PORT": env.config.BackendPort,
			"DOMAIN_SERVER_HOST": env.config.BackendHost,
			"KAFKA_PORT":         env.config.KafkaPort,
			"KAFKA_HOST":         env.config.KafkaHost,
		},
		ExposedPorts: []string{port.Port() + "/tcp"},
		Networks: []string{
			env.bffTestNetwork.Name,
		},
		WaitingFor: wait.ForLog("Listening and serving"),
		NetworkAliases: map[string][]string{
			networkName: {"bff-service"},
		},
	}

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

	bffPortInt, err := strconv.Atoi(env.config.BFFServerPort)
	if err != nil {
		return "", fmt.Errorf("invalid port number: %w", err)
	}

	req := testcontainers.ContainerRequest{
		Image: "znsio/specmatic",
		Env: map[string]string{
			"SPECMATIC_GENERATIVE_TESTS": "true",
		},
		Cmd: []string{"test", fmt.Sprintf("--port=%d", bffPortInt), "--host=bff-service"},
		Mounts: testcontainers.Mounts(
			testcontainers.BindMount(filepath.Join(pwd, "specmatic.yaml"), "/usr/src/app/specmatic.yaml"),
		),
		Networks: []string{
			env.bffTestNetwork.Name,
		},
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

func setKafkaExpectations(env *testEnvironment, kafkaC testcontainers.Container) error {
	// Install curl
	_, _, err := kafkaC.Exec(env.ctx, []string{"apk", "add", "--no-cache", "curl"})
	if err != nil {
		fmt.Printf("Error installing curl: %v", err)
	}

	cmd := []string{
		"curl", "-X", "POST",
		"-H", "Content-Type: application/json",
		"-d", fmt.Sprintf(`[{"topic": "product-queries", "count": %d}]`, env.expectedMessageCount),
		fmt.Sprintf("http://localhost:%s/_expectations", env.kafkaAPIPort),
	}

	exitCode, output, err := kafkaC.Exec(env.ctx, cmd)
	if err != nil {
		return fmt.Errorf("failed to execute set expectations command: %v", err)
	}

	if exitCode != 0 {
		return fmt.Errorf("failed to set Kafka expectations. Exit code: %d, Output: %s", exitCode, output)
	}

	fmt.Printf("Kafka expectations set successfully. Output: %s\n", output)
	return nil
}

func verifyKafkaExpectations(env *testEnvironment) error {
	cmd := []string{
		"curl", "-s", "-X", "POST",
		fmt.Sprintf("http://localhost:%s/_expectations/verifications", env.kafkaAPIPort),
	}

	exitCode, outputReader, err := env.kafkaServiceContainer.Exec(env.ctx, cmd)
	if err != nil {
		return fmt.Errorf("failed to execute verify expectations command: %v", err)
	}

	outputBytes, err := io.ReadAll(outputReader)
	if err != nil {
		return fmt.Errorf("failed to read command output: %v", err)
	}

	if exitCode != 0 {
		return fmt.Errorf("failed to verify Kafka expectations. Exit code: %d, Output: %s", exitCode, string(outputBytes))
	}

	var result struct {
		Success bool     `json:"success"`
		Errors  []string `json:"errors"`
	}
	if err := json.Unmarshal(outputBytes, &result); err != nil {
		return fmt.Errorf("failed to parse Kafka expectations result: %v", err)
	}

	if !result.Success {
		return fmt.Errorf("Kafka mock expectations were not met. Errors: %v", result.Errors)
	}

	fmt.Println("Kafka mock expectations were met successfully.")
	return nil
}

func getAPIServerPort(ctx context.Context, container testcontainers.Container) (string, error) {
	logs, err := container.Logs(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get container logs: %v", err)
	}
	defer logs.Close()

	scanner := bufio.NewScanner(logs)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "Starting api server on port:") {
			parts := strings.Split(line, ":")
			if len(parts) > 1 {
				return strings.TrimSpace(parts[len(parts)-1]), nil
			}
		}
	}

	return "", fmt.Errorf("API server port not found in logs")
}

func printHeader(t *testing.T, stepNum int, title string) {
	t.Log("")
	t.Logf("======== STEP %d =========", stepNum)
	t.Log(title)
	t.Log("=========================")
	t.Log("")
}
