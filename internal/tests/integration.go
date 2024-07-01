package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"os/exec"
	"time"
)

// HOST and PORT settings for the subs and the application
const (
	domainStubAPIHost = "localhost"
	domainStubAPIPort = "8090"
	kafkaMockHost     = "localhost"
	kafkaMockPort     = "9092"
	bffServerHost     = "localhost"
	bffServerPort     = "8080"
)

// Map to store all cleanup function to stubs and application, for tear down later on.
var tearDownActions []func() error

// Represnts any service that we need to start on asynchronously
type Service struct {
	Name string
	Cmd  *exec.Cmd
	Host string
	Port string
}

func main() {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)

	// Defer cancellation of context to stop all services when main exits
	defer cancel()

	// Defer cleanup of all started stubs and BFF application
	defer func() {
		for _, cleanup := range tearDownActions {
			if err := cleanup(); err != nil {
				log.Printf("Error during cleanup: %v", err)
			}
		}
	}()

	// Step 1. add services to be started, in order.
	services := []Service{
		{Name: "order-api", Cmd: exec.CommandContext(ctx, "specmatic", "stub", "--host", domainStubAPIHost, "--port", domainStubAPIPort), Host: domainStubAPIHost, Port: domainStubAPIPort},
		// {Name: "kafka-mock", Cmd: exec.CommandContext(ctx, "specmatic", "stub", "--host", kafkaMockHost, "--port", kafkaMockPort), Host: kafkaMockHost, Port: kafkaMockPort},
		{Name: "BFF", Cmd: exec.CommandContext(ctx, "go", "run", "cmd/main.go"), Host: bffServerHost, Port: bffServerPort},
	}

	// Step 2. start the services
	for _, service := range services {
		err := startService(ctx, service)
		if err != nil {
			log.Fatalf("Failed to start %s: %v", service.Name, err)
		}
	}

	// Step 3: Run the tests
	err := runTests(ctx, bffServerHost, bffServerPort)
	if err != nil {
		log.Fatalf("Failed to run tests: %v", err)
	}
}

// Starts a service asynchronously and returns a cleanup function
func startService(ctx context.Context, service Service) error {
	fmt.Printf("Starting %s on %s:%s\n", service.Name, service.Host, service.Port)

	if err := service.Cmd.Start(); err != nil {
		return fmt.Errorf("failed to start %s: %v", service.Name, err)
	}

	// Capture the cleanup function for the current service
	cleanup := func() error {
		fmt.Printf("Stopping %s on %s:%s\n", service.Name, service.Host, service.Port)
		return service.Cmd.Process.Kill()
	}

	// Store the cleanup function for later execution
	tearDownActions = append(tearDownActions, cleanup)

	// Start the service asynchronously
	go func() {
		err := service.Cmd.Wait()
		if err != nil {
			log.Printf("%s stopped with error: %v", service.Name, err)
		}
	}()

	// Wait a bit to ensure the server has started (adjust as needed based on startup time)
	time.Sleep(5 * time.Second)

	return nil
}

func runTests(ctx context.Context, host, port string) error {
	cmd := exec.CommandContext(ctx, "specmatic", "test", "--host", host, "--port", port)
	fmt.Printf("Running tests on %s:%s\n", host, port)

	// Create pipes to capture stdout and stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("error creating stdout pipe: %v", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("error creating stderr pipe: %v", err)
	}

	// Start the command
	err = cmd.Start()
	if err != nil {
		return fmt.Errorf("error starting tests: %v", err)
	}

	// Print stdout and stderr in separate goroutines
	go printOutput(stdout, "[specmatic test] ")
	go printOutput(stderr, "[specmatic test ERROR] ")

	// Wait for command to finish
	err = cmd.Wait()
	if err != nil {
		return fmt.Errorf("error running tests: %v", err)
	}

	fmt.Println("Tests completed successfully.")

	return nil
}

// printOutput reads from the test pipe and prints lines prefixed with the given prefix
func printOutput(pipe io.ReadCloser, prefix string) {
	scanner := bufio.NewScanner(pipe)
	for scanner.Scan() {
		fmt.Printf("%s%s\n", prefix, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		log.Printf("Error reading from pipe: %v", err)
	}
}
