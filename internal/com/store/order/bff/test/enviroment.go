package test

import (
	"context"

	"github.com/testcontainers/testcontainers-go"
	"github.com/znsio/specmatic-order-bff-go/internal/com/store/order/bff/config"
)

type TestEnvironment struct {
	Ctx                      context.Context
	BffTestNetwork           *testcontainers.DockerNetwork
	DomainServiceContainer   testcontainers.Container
	DomainServiceDynamicPort string
	KafkaServiceContainer    testcontainers.Container
	KafkaServiceDynamicPort  string
	KafkaDynamicAPIPort      string
	KafkaAPIHost             string
	BffServiceContainer      testcontainers.Container
	BffServiceDynamicPort    string
	ExpectedMessageCount     int
	Config                   *config.Config
}
