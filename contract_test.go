package main_test

import (
	"context"
	"strings"
	"testing"

	"github.com/testcontainers/testcontainers-go/network"
	"github.com/znsio/specmatic-order-bff-go/internal/com/store/order/bff/config"
	"github.com/znsio/specmatic-order-bff-go/internal/com/store/order/bff/test"
)

var authToken = "API-TOKEN-SPEC"

func TestContract(t *testing.T) {
	env := setUpEnv(t)

	setUp(t, env)

	runTests(t, env)

	defer tearDown(t, env)
}

func setUpEnv(t *testing.T) *test.TestEnvironment {
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

	return &test.TestEnvironment{
		Ctx:                  ctx,
		Config:               config,
		BffTestNetwork:       newNetwork,
		ExpectedMessageCount: 3,
	}
}

func setUp(t *testing.T, env *test.TestEnvironment) {
	var err error

	printHeader(t, 1, "Starting Specmatic HTTP Stub Server to emulate domain server")
	env.DomainServiceContainer, env.DomainServiceDynamicPort, err = test.StartDomainService(t, env)
	if err != nil {
		t.Fatalf("could not start domain service container: %v", err)
	}

	printHeader(t, 2, "Starting Kafka Mock")
	env.KafkaServiceContainer, env.KafkaServiceDynamicPort, err = test.StartKafkaMock(t, env)
	if err != nil {
		t.Fatalf("could not start Kafka service container: %v", err)
	}

	printHeader(t, 3, "Starting BFF Service")
	env.BffServiceContainer, env.BffServiceDynamicPort, err = test.StartBFFService(t, env)
	if err != nil {
		t.Fatalf("could not start bff service container: %v", err)
	}
}

func runTests(t *testing.T, env *test.TestEnvironment) {
	printHeader(t, 4, "Starting tests")
	testLogs, err := test.RunTestContainer(env)

	if (err != nil) && !strings.Contains(err.Error(), "code 0") {
		t.Logf("Could not run test container: %s", err)
		t.Fail()
	}

	// Print test outcomes
	t.Log("Test Results:")
	t.Log(testLogs)
}

func tearDown(t *testing.T, env *test.TestEnvironment) {
	if env.BffServiceContainer != nil {
		if err := env.BffServiceContainer.Terminate(env.Ctx); err != nil {
			t.Logf("Failed to terminate BFF container: %v", err)
		}
	}

	if env.KafkaServiceContainer != nil {
		err := test.VerifyKafkaExpectations(env)
		if err != nil {
			t.Logf("Kafka expectations were not met: %s", err)
			t.Fail()
		}
		if err := env.KafkaServiceContainer.Terminate(env.Ctx); err != nil {
			t.Logf("Failed to terminate Kafka container: %v", err)
		}
	}

	if env.DomainServiceContainer != nil {
		if err := env.DomainServiceContainer.Terminate(env.Ctx); err != nil {
			t.Logf("Failed to terminate stub container: %v", err)
		}
	}
}

func printHeader(t *testing.T, stepNum int, title string) {
	t.Log("")
	t.Logf("======== STEP %d =========", stepNum)
	t.Log(title)
	t.Log("=========================")
	t.Log("")
}
