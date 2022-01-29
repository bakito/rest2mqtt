package main

import (
	"crypto/tls"
	_ "embed"
	"fmt"
	"net/http"
	"os"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/gin-gonic/gin"
	"github.com/ulule/limiter/v3"
	mgin "github.com/ulule/limiter/v3/drivers/middleware/gin"
	"github.com/ulule/limiter/v3/drivers/store/memory"
	"go.uber.org/zap"
)

const (
	envMQTTHost     = "MQTT_HOST"
	envMQTTUser     = "MQTT_USER"
	envMQTTPassword = "MQTT_PASSWORD"
	envToken        = "TOKEN"
)

var (
	mqttClient mqtt.Client
	token      = os.Getenv(envToken)
	log        *zap.SugaredLogger

	//go:embed banner.html
	banner string
)

func main() {
	cfg := zap.NewDevelopmentConfig()
	cfg.DisableStacktrace = true
	logger, _ := cfg.Build()
	defer func() { _ = logger.Sync() }()
	log = logger.Sugar()

	var err error
	mqttClient, err = newClient(os.Getenv(envMQTTHost), os.Getenv(envMQTTUser), os.Getenv(envMQTTPassword))
	if err != nil {
		log.Fatal(err)
	}

	rateLimit, err := rateLimitMiddleware()
	if err != nil {
		log.Fatal(err)
	}

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.GET("/", handleIndex)
	v1 := r.Group("/v1")
	v1.Use(rateLimit)
	v1.POST("/mqtt", handleMQTT)

	log.Infow("Starting", "port", 8080)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%v", 8080), r))
}

func rateLimitMiddleware() (gin.HandlerFunc, error) {
	// Define a limit rate to 4 requests per hour.
	rate, err := limiter.NewRateFromFormatted("4-H")
	if err != nil {
		return nil, err
	}
	store := memory.NewStore()
	// Create a new middleware with the limiter instance.
	middleware := mgin.NewMiddleware(limiter.New(store, rate))
	return middleware, nil
}

func handleIndex(c *gin.Context) {
	c.Data(http.StatusOK, "text/html", []byte(banner))
}

func handleMQTT(c *gin.Context) {
	a := &mqttAction{}
	if err := c.ShouldBindJSON(a); err != nil {
		log.Infow("Info unmarshalling body", "from", readUserIP(c), "error", err)
		c.String(http.StatusBadRequest, "")
		return
	}

	if a.Token == "" || a.Token != token {
		log.Errorw("Error token mismatch", "from", readUserIP(c), "token", a.Token)
		c.String(http.StatusUnauthorized, "")
		return
	}

	if a.Topic == "" {
		log.Infow("Info topic is blank", "from", readUserIP(c))
		c.String(http.StatusBadRequest, "")
		return
	}

	if a.Payload == "" {
		log.Infow("Info payload is blank", "from", readUserIP(c))
		c.String(http.StatusBadRequest, "")
		return
	}

	t := mqttClient.Publish(a.Topic, a.QOS, a.Retained, a.Payload)
	if t.Error() != nil {
		log.Infow("Info publishing", "from", readUserIP(c), "topic", a.Topic, "payload", a.Payload,
			"qos", a.QOS, "retained", a.Retained, "error", t.Error())
		c.String(http.StatusInternalServerError, "")
		return
	}
	log.Infow("Published", "from", readUserIP(c), "topic", a.Topic, "payload", a.Payload,
		"qos", a.QOS, "retained", a.Retained)
}

func newClient(url string, user string, password string) (mqtt.Client, error) {
	opts := mqtt.NewClientOptions()
	opts.AddBroker(url)

	opts.SetClientID("rest2mqtt_" + os.Getenv("HOSTNAME"))
	opts.SetUsername(user)
	opts.SetPassword(password)
	opts.SetCleanSession(false)
	opts.SetAutoReconnect(true)
	opts.SetMaxReconnectInterval(10 * time.Second)

	// #nosec G402
	opts.TLSConfig = &tls.Config{
		InsecureSkipVerify: true,
	}

	client := mqtt.NewClient(opts)

	if token := client.Connect(); token.Wait() && token.Error() != nil {
		return nil, token.Error()
	}

	log.Infow("Using MQTT", "host", url)
	return client, nil
}

type mqttAction struct {
	Topic    string `json:"topic"`
	Payload  string `json:"payload"`
	QOS      byte   `json:"qos"`
	Retained bool   `json:"retained"`
	Token    string `json:"token"`
}

func readUserIP(c *gin.Context) string {
	IPAddress := c.GetHeader("X-Real-Ip")
	if IPAddress == "" {
		IPAddress = c.GetHeader("X-Forwarded-For")
	}
	if IPAddress == "" {
		ip, _ := c.RemoteIP()
		IPAddress = ip.String()
	}
	return IPAddress
}
