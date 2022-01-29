package server

import (
	_ "embed"
	"fmt"
	"net/http"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/gin-gonic/gin"
	libredis "github.com/go-redis/redis/v8"
	"github.com/ulule/limiter/v3"
	mgin "github.com/ulule/limiter/v3/drivers/middleware/gin"
	"github.com/ulule/limiter/v3/drivers/store/memory"
	sredis "github.com/ulule/limiter/v3/drivers/store/redis"
	"go.uber.org/zap"
)

//go:embed banner.html
var banner string

func New(log *zap.SugaredLogger, token string, mqttClient mqtt.Client, redisClient *libredis.Client) Server {
	rateLimit, err := rateLimitMiddleware(redisClient)
	if err != nil {
		log.Fatal(err)
	}
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()

	s := &server{
		log:   log,
		r:     r,
		token: token,
		mqtt:  mqttClient,
		redis: redisClient,
	}

	r.Use(gin.Recovery())
	r.GET("/", handleIndex)
	v1 := r.Group("/v1")
	v1.Use(rateLimit)
	v1.POST("/mqtt", s.handleMQTT)
	return s
}

type server struct {
	log   *zap.SugaredLogger
	r     *gin.Engine
	token string
	mqtt  mqtt.Client
	redis *libredis.Client
}

type Server interface {
	Run()
}

func (s *server) Run() {
	s.log.Infow("Starting", "port", 8080)
	s.log.Fatal(http.ListenAndServe(fmt.Sprintf(":%v", 8080), s.r))
}

func rateLimitMiddleware(client *libredis.Client) (gin.HandlerFunc, error) {
	// Define a limit rate to 4 requests per hour.
	rate, err := limiter.NewRateFromFormatted("4-H")
	if err != nil {
		return nil, err
	}

	var store limiter.Store
	if client != nil {
		// Create a store with the redis client.
		store, err = sredis.NewStoreWithOptions(client, limiter.StoreOptions{
			Prefix: "rest2mqtt_limiter",
		})
		if err != nil {
			return nil, err
		}
	} else {
		store = memory.NewStore()
	}

	// Create a new middleware with the limiter instance.
	middleware := mgin.NewMiddleware(limiter.New(store, rate))
	return middleware, nil
}

func handleIndex(c *gin.Context) {
	c.Data(http.StatusOK, "text/html", []byte(banner))
}

func (s *server) handleMQTT(c *gin.Context) {
	a := &mqttAction{}
	if err := c.ShouldBindJSON(a); err != nil {
		s.log.Infow("Info unmarshalling body", "from", readUserIP(c), "error", err)
		c.String(http.StatusBadRequest, "")
		return
	}

	if a.Token == "" || a.Token != s.token {
		s.log.Errorw("Error token mismatch", "from", readUserIP(c), "token", a.Token)
		c.String(http.StatusUnauthorized, "")
		return
	}

	if a.Topic == "" {
		s.log.Infow("Info topic is blank", "from", readUserIP(c))
		c.String(http.StatusBadRequest, "")
		return
	}

	if a.Payload == "" {
		s.log.Infow("Info payload is blank", "from", readUserIP(c))
		c.String(http.StatusBadRequest, "")
		return
	}

	t := s.mqtt.Publish(a.Topic, a.QOS, a.Retained, a.Payload)
	if t.Error() != nil {
		s.log.Infow("Info publishing", "from", readUserIP(c), "topic", a.Topic, "payload", a.Payload,
			"qos", a.QOS, "retained", a.Retained, "error", t.Error())
		c.String(http.StatusInternalServerError, "")
		return
	}
	s.log.Infow("Published", "from", readUserIP(c), "topic", a.Topic, "payload", a.Payload,
		"qos", a.QOS, "retained", a.Retained)
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
