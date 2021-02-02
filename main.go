package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/gorilla/mux"
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
)

func main() {
	var err error
	mqttClient, err = newClient(os.Getenv(envMQTTHost), os.Getenv(envMQTTUser), os.Getenv(envMQTTPassword))
	if err != nil {
		panic(err)
	}
	r := mux.NewRouter()
	r.HandleFunc("/", handleRoot).Methods(http.MethodGet)
	v1 := r.PathPrefix("/v1").Subrouter()
	v1.HandleFunc("/mqtt", handleMQTT).Methods(http.MethodPost)
	log.Printf("Starting on port: %d", 8080)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%v", 8080), r))
}

func handleRoot(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	_, _ = w.Write([]byte(banner))
}

func handleMQTT(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Printf("Error: %v", err)
		http.Error(w, "", http.StatusBadRequest)
		return
	}
	if err := r.Body.Close(); err != nil {
		log.Printf("Error: %v", err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}

	a := &mqttAction{}
	if err := json.Unmarshal(body, a); err != nil {
		log.Printf("Error: %v", err)
		http.Error(w, "", http.StatusBadRequest)
		return
	}

	if a.Token == "" || a.Token != token {
		log.Printf("Error: Token mismatch %s", a.Token)
		http.Error(w, "", http.StatusUnauthorized)
		return
	}

	if a.Topic == "" || a.Payload == "" {
		log.Printf("Error: topic %q or payload %q is blank", a.Token, a.Payload)
		http.Error(w, "", http.StatusBadRequest)
		return
	}

	t := mqttClient.Publish(a.Topic, a.QOS, a.Retained, a.Payload)
	if t.Error() != nil {
		log.Printf("Error: %v / Published: topic %q, payload %q, qos %v, retained %v", t.Error(), a.Topic, a.Payload, a.QOS, a.Retained)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	log.Printf("Published: topic %q, payload %q, qos %v, retained %v", a.Topic, a.Payload, a.QOS, a.Retained)
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

	log.Printf("Using MQTT host: %s", url)
	return client, nil
}

type mqttAction struct {
	Topic    string `json:"topic"`
	Payload  string `json:"payload"`
	QOS      byte   `json:"qos"`
	Retained bool   `json:"retained"`
	Token    string `json:"token"`
}
