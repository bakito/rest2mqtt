package server

import (
	_ "embed"
	"fmt"
	"net/http"
	"net/http/httputil"
	"os"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/gin-gonic/gin"
	libredis "github.com/redis/go-redis/v9"
	"github.com/ulule/limiter/v3"
	mgin "github.com/ulule/limiter/v3/drivers/middleware/gin"
	"github.com/ulule/limiter/v3/drivers/store/memory"
	sredis "github.com/ulule/limiter/v3/drivers/store/redis"
	"go.uber.org/zap"
)

const (
	port        = 8080
	healthzPath = "/healthz"
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
	r.GET(healthzPath, s.handleHealthz)
	v1 := r.Group("/v1")
	if !strings.EqualFold(os.Getenv("SKIP_RATE_LIMIT"), "true") {
		v1.Use(rateLimit)
	}
	v1.POST("/mqtt", s.handleMQTT)
	v1.POST("/log", s.handleLog)
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
	s.log.Infow("Starting", "port", port)
	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", port),
		Handler:           s.r,
		ReadHeaderTimeout: 1 * time.Second,
	}

	s.log.Fatal(srv.ListenAndServe())
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

func (s *server) handleHealthz(c *gin.Context) {
	if s.mqtt.IsConnected() {
		c.String(http.StatusOK, "OK")
	} else {
		c.String(http.StatusInternalServerError, "NOT CONNECTED")
	}
}

func (s *server) handleMQTT(c *gin.Context) {
	auth := c.GetHeader("Authorization")
	authorized := false
	if auth != "" {
		token := strings.Replace(auth, "Bearer ", "", 1)
		if token != s.token {
			s.log.Errorw("Error authorization token mismatch", "from", readUserIP(c), "token", token)
			c.String(http.StatusUnauthorized, "")
			return
		}
		authorized = true
	}

	a := &mqttAction{}
	if err := c.ShouldBindJSON(a); err != nil {
		s.log.Infow("Info unmarshalling body", "from", readUserIP(c), "error", err)
		c.String(http.StatusBadRequest, "")
		return
	}

	if !authorized && (a.Token == nil || *a.Token != s.token) {
		s.log.Errorw("Error message token mismatch", "from", readUserIP(c), "token", a.Token)
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
	Topic    string  `json:"topic"`
	Payload  string  `json:"payload"`
	QOS      byte    `json:"qos"`
	Retained bool    `json:"retained"`
	Token    *string `json:"token,omitempty"`
}

func readUserIP(c *gin.Context) string {
	IPAddress := c.GetHeader("X-Real-Ip")
	if IPAddress == "" {
		IPAddress = c.GetHeader("X-Forwarded-For")
	}
	if IPAddress == "" {
		IPAddress = c.RemoteIP()
	}
	return IPAddress
}

func (s *server) handleLog(c *gin.Context) {
	d, err := httputil.DumpRequest(c.Request, true)
	if err != nil {
		c.String(http.StatusInternalServerError, "")
		return
	}
	s.log.Infof("Request\n%s\n", string(d))
}
