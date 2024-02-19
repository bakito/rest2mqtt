package mqtt

import (
	"crypto/tls"
	"os"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"go.uber.org/zap"
)

func Client(log *zap.SugaredLogger, url string, user string, password string) (mqtt.Client, error) {
	opts := mqtt.NewClientOptions()
	opts.AddBroker(url)

	opts.SetClientID("rest2mqtt_" + os.Getenv("HOSTNAME"))
	opts.SetUsername(user)
	opts.SetPassword(password)
	opts.SetCleanSession(false)
	opts.SetAutoReconnect(true)
	opts.SetMaxReconnectInterval(10 * time.Second)

	opts.TLSConfig = &tls.Config{
		// #nosec G402
		InsecureSkipVerify: true,
	}

	client := mqtt.NewClient(opts)

	if token := client.Connect(); token.Wait() && token.Error() != nil {
		return nil, token.Error()
	}

	log.Infow("Using MQTT", "host", url)
	return client, nil
}
