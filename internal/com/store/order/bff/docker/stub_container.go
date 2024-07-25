package docker

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/docker/go-connections/nat"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"github.com/znsio/specmatic-order-bff-go/internal/com/store/order/bff/config"
)

func StartOrderStub() {
	ctx := context.Background()

	// Get current working directory
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
			testcontainers.BindMount(filepath.Join(pwd, "./.specmatic"), "/.specmatic"),
		),
		WaitingFor: wait.ForLog("Stub server is running"),
	}

	specmaticC, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		log.Fatalf("Error starting container: %v", err)
	}
	defer func() {
		if err := specmaticC.Terminate(ctx); err != nil {
			log.Fatalf("Error terminating container: %v", err)
		}
	}()

	// Get the container's host and port
	host, err := specmaticC.Host(ctx)
	if err != nil {
		log.Fatalf("Error getting host: %v", err)
	}

	port, err := specmaticC.MappedPort(ctx, nat.Port("9000/tcp")) // Ensure using nat.Port("9000/tcp")
	if err != nil {
		log.Fatalf("Error getting port: %v", err)
	}

	// // update the backend port as test containers give random port bindings, to avoid collisions. and it can't be overriden.
	config.SetBackendPort(port.Port())

	// Access configuration
	cfg := config.GetConfig()

	fmt.Println("The updated value is ==>", cfg.BackendPort)

	fmt.Printf("Specmatic Stub server is running on http://%s:%s\n", host, port.Port())

	// Keep the container running
	select {}
}
