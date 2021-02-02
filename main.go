package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	"github.com/didip/tollbooth/v6"
	"github.com/didip/tollbooth/v6/limiter"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/gorilla/mux"
	"go.uber.org/zap"
)

const (
	envMQTTHost     = "MQTT_HOST"
	envMQTTUser     = "MQTT_USER"
	envMQTTPassword = "MQTT_PASSWORD"
	envToken        = "TOKEN"

	banner = `
                _   ___  __  __  ____ _______ _______ 
               | | |__ \|  \/  |/ __ \__   __|__   __|
  _ __ ___  ___| |_   ) | \  / | |  | | | |     | |
 | '__/ _ \/ __| __| / /| |\/| | |  | | | |     | |
 | | |  __/\__ \ |_ / /_| |  | | |__| | | |     | |
 |_|  \___||___/\__|____|_|  |_|\___\_\ |_|     |_|
`
)

var (
	mqttClient mqtt.Client
	token      = os.Getenv(envToken)
	log        *zap.SugaredLogger
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
		panic(err)
	}

	lmt := tollbooth.NewLimiter(1, &limiter.ExpirableOptions{DefaultExpirationTTL: time.Hour})

	r := mux.NewRouter()
	r.Handle("/", tollbooth.LimitFuncHandler(lmt, handleRoot)).Methods(http.MethodGet)
	v1 := r.PathPrefix("/v1").Subrouter()
	v1.Handle("/mqtt", tollbooth.LimitFuncHandler(lmt, handleMQTT)).Methods(http.MethodPost)

	log.Infow("Starting", "port", 8080)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%v", 8080), r))
}

func handleRoot(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	_, _ = w.Write([]byte(banner))
}

func handleMQTT(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Infow("Info reading body", "from", readUserIP(r), "error", err)
		http.Error(w, "", http.StatusBadRequest)
		return
	}
	if err := r.Body.Close(); err != nil {
		log.Infow("Info closing body", "from", readUserIP(r), "error", err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}

	a := &mqttAction{}
	if err := json.Unmarshal(body, a); err != nil {
		log.Infow("Info unmarshalling body", "from", readUserIP(r), "error", err)
		http.Error(w, "", http.StatusBadRequest)
		return
	}

	if a.Token == "" || a.Token != token {
		log.Errorw("Error token mismatch", "from", readUserIP(r), "token", a.Token)
		http.Error(w, "", http.StatusUnauthorized)
		return
	}

	if a.Topic == "" {
		log.Infow("Info topic is blank", "from", readUserIP(r))
		http.Error(w, "", http.StatusBadRequest)
		return
	}

	if a.Payload == "" {
		log.Infow("Info payload is blank", "from", readUserIP(r))
		http.Error(w, "", http.StatusBadRequest)
		return
	}

	t := mqttClient.Publish(a.Topic, a.QOS, a.Retained, a.Payload)
	if t.Error() != nil {
		log.Infow("Info publishing", "from", readUserIP(r), "topic", a.Topic, "payload", a.Payload,
			"qos", a.QOS, "retained", a.Retained, "error", t.Error())
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	log.Infow("Published", "from", readUserIP(r), "topic", a.Topic, "payload", a.Payload,
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

func readUserIP(r *http.Request) string {
	IPAddress := r.Header.Get("X-Real-Ip")
	if IPAddress == "" {
		IPAddress = r.Header.Get("X-Forwarded-For")
	}
	if IPAddress == "" {
		IPAddress = r.RemoteAddr
	}
	return IPAddress
}
