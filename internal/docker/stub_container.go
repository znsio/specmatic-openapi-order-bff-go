package docker

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
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

	port, err := specmaticC.MappedPort(ctx, "9000")
	if err != nil {
		log.Fatalf("Error getting port: %v", err)
	}

	fmt.Printf("Specmatic stub server is running on http://%s:%s\n", host, port.Port())

	// Keep the container running
	select {}
}
