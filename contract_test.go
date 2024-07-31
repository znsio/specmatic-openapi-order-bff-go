package main_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/docker/go-connections/nat"
	"github.com/stretchr/testify/require"
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
	bffServiceContainer      testcontainers.Container
	bffServiceDynamicPort    string
	expectedMessageCount     int
	config                   *config.Config
}

func TestContract(t *testing.T) {
	env := setUpEnv(t)

	setUp(t, env)

	runTests(t, env)

	select {}

	// defer tearDown(t, env)
}

func setUpEnv(t *testing.T) *testEnvironment {
	config, err := config.LoadConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	return &testEnvironment{
		ctx:                  context.Background(),
		config:               config,
		expectedMessageCount: 6,
	}
}

func setUp(t *testing.T, env *testEnvironment) {
	var err error

	// Create a sub net and store in env.
	newNetwork, err := network.New(env.ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		require.NoError(t, newNetwork.Remove(env.ctx))
	})
	env.bffTestNetwork = newNetwork

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
	if err != nil {
		t.Logf("Could not run test container: %s", err)
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
		if err := env.kafkaServiceContainer.Terminate(env.ctx); err != nil {
			t.Logf("Failed to terminate Kafka container: %v", err)
		}
	}

	if env.kafkaServiceContainer != nil {
		logs, err := env.kafkaServiceContainer.Logs(env.ctx)
		if err != nil {
			t.Fatalf("Failed to get Kafka mock logs: %v", err)
		}
		defer logs.Close()

		logContent, err := io.ReadAll(logs)
		if err != nil {
			t.Fatalf("Failed to read Kafka mock logs: %v", err)
		}

		if !strings.Contains(string(logContent), "All expectations met") {
			t.Fatalf("Kafka mock expectations were not met. Logs: %s", string(logContent))
		}

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
			testcontainers.BindMount(filepath.Join(pwd, "specmatic.json"), "/usr/src/app/specmatic.json"),
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
		Image: "znsio/specmatic-kafka:0.22.7-local",
		// Image:        "znsio/specmatic-kafka-trial:0.22.5",
		ExposedPorts: []string{port.Port() + "/tcp"},
		Networks: []string{
			networkName,
		},
		NetworkAliases: map[string][]string{
			networkName: {env.config.KafkaHost},
		},
		Cmd: []string{"--config=/specmatic.json"}, // TODO: Switch to YAML
		Mounts: testcontainers.Mounts(
			testcontainers.BindMount(filepath.Join(pwd, "specmatic.json"), "/specmatic.json"),
		),
		Env: map[string]string{
			"KAFKA_EXTERNAL_HOST":    env.config.KafkaHost,
			"KAFKA_EXTERNAL_PORT":    env.config.KafkaPort,
			"EXPECTED_TOPIC":         env.config.KafkaTopic,
			"EXPECTED_MESSAGE_COUNT": strconv.Itoa(env.expectedMessageCount),
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
			testcontainers.BindMount(filepath.Join(pwd, "specmatic.json"), "/usr/src/app/specmatic.json"),
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

func printHeader(t *testing.T, stepNum int, title string) {
	t.Log("")
	t.Logf("======== STEP %d =========", stepNum)
	t.Log(title)
	t.Log("=========================")
	t.Log("")
}
