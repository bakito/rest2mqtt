package main

import (
	_ "embed"
	"flag"
	"os"

	"github.com/bakito/rest2mqtt/pkg/mqtt"
	"github.com/bakito/rest2mqtt/pkg/server"
	libredis "github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const (
	envMQTTHost     = "MQTT_HOST"
	envMQTTUser     = "MQTT_USER"
	envMQTTPassword = "MQTT_PASSWORD"
	envToken        = "TOKEN"
	encRedisURL     = "REDIS_URL"
)

var (
	token = os.Getenv(envToken)
	log   *zap.SugaredLogger
)

func main() {
	healthz := flag.Bool("healthz", false, "run healthcheck")
	flag.Parse()

	if *healthz {
		os.Exit(server.Healthz())
	}

	cfg := zap.NewDevelopmentConfig()
	cfg.DisableStacktrace = true
	logger, _ := cfg.Build()
	defer func() { _ = logger.Sync() }()
	log = logger.Sugar()

	var err error
	mqttClient, err := mqtt.Client(log, os.Getenv(envMQTTHost), os.Getenv(envMQTTUser), os.Getenv(envMQTTPassword))
	if err != nil {
		log.Fatal(err)
	}

	var redis *libredis.Client
	// Create a redis client.
	if url, ok := os.LookupEnv(encRedisURL); ok {
		option, err := libredis.ParseURL(url)
		if err != nil {
			log.Fatal(err)
		}
		redis = libredis.NewClient(option)
		log.Infow("Using Redis", "url", url)
	}

	s := server.New(log, token, mqttClient, redis)
	s.Run()
}
