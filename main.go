package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/boltdb/bolt"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/gowon-irc/go-gowon"
	"github.com/imroc/req/v3"
	"github.com/jessevdk/go-flags"
)

type Options struct {
	Prefix string `short:"P" long:"prefix" env:"GOWON_PREFIX" default:"." description:"prefix for commands"`
	Broker string `short:"b" long:"broker" env:"GOWON_BROKER" default:"localhost:1883" description:"mqtt broker"`
	APIKey string `short:"k" long:"api-key" env:"GOWON_XBOXLIVE_API_KEY" required:"true" description:"openxbl api key"`
	KVPath string `short:"K" long:"kv-path" env:"GOWON_XBOXLIVE_KV_PATH" default:"kv.db" description:"path to kv db"`
}

const (
	moduleName               = "xboxlive"
	mqttConnectRetryInternal = 5
	mqttDisconnectTimeout    = 1000
)

func setUser(kv *bolt.DB, nick, gamerTag, xuid []byte) error {
	err := kv.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("xboxlive_xuid"))
		return b.Put([]byte(nick), []byte(xuid))
	})

	if err != nil {
		return err
	}

	err = kv.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("xboxlive_gamertag"))
		return b.Put([]byte(nick), []byte(gamerTag))
	})

	return err
}

func getUser(kv *bolt.DB, nick []byte) (gamerTag, user []byte, err error) {
	err = kv.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("xboxlive_gamertag"))
		v := b.Get([]byte(nick))
		gamerTag = v

		b = tx.Bucket([]byte("xboxlive_xuid"))
		v = b.Get([]byte(nick))
		user = v

		return nil
	})
	return gamerTag, user, err
}

func parseArgs(msg string) (command, user string) {
	fields := strings.Fields(msg)

	if len(fields) >= 1 {
		command = fields[0]
	}

	if len(fields) >= 2 {
		user = fields[1]
	}

	return command, user
}

func setUserHandler(client *req.Client, kv *bolt.DB, nick, user string) (string, error) {
	if user == "" {
		return "Error: username needed", nil
	}

	xuid, gamerTag, err := xblGetXuid(client, user)
	if errors.Is(userNotFoundErr, err) {
		return fmt.Sprintf("Error: no user found for %s", user), nil
	}
	if err != nil {
		return "", err
	}

	err = setUser(kv, []byte(nick), []byte(gamerTag), []byte(xuid))
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("set %s's user to %s (%s)", nick, gamerTag, xuid), nil
}

type commandFunc func(*req.Client, string, string) (string, error)

func CommandHandler(client *req.Client, kv *bolt.DB, nick, user string, f commandFunc) (string, error) {
	if user != "" {
		xuid, gamerTag, err := xblGetXuid(client, user)
		if errors.Is(userNotFoundErr, err) {
			return fmt.Sprintf("Error: no user found for %s", user), nil
		}
		if err != nil {
			return "", err
		}
		return f(client, gamerTag, xuid)
	}

	gamerTag, xuid, err := getUser(kv, []byte(nick))
	if err != nil {
		return "", err
	}

	if len(xuid) == 0 {
		return "Error: username needed", nil
	}

	return f(client, string(gamerTag), string(xuid))
}

func genXblHandler(client *req.Client, kv *bolt.DB) func(m gowon.Message) (string, error) {
	return func(m gowon.Message) (string, error) {
		command, user := parseArgs(m.Args)

		switch command {
		case "s", "set":
			return setUserHandler(client, kv, m.Nick, user)
		case "r", "recent":
			return CommandHandler(client, kv, m.Nick, user, xblRecentGames)
		case "l", "last":
			return CommandHandler(client, kv, m.Nick, user, xblLastGame)
		case "a", "achievement":
			return CommandHandler(client, kv, m.Nick, user, xblLastAchievement)
		case "p", "player":
			return CommandHandler(client, kv, m.Nick, user, xblPlayerSummary)
		}

		return "one of [s]et, [r]ecent, [l]ast, [a]chievements or [p]layer  must be passed as a command", nil
	}
}

func defaultPublishHandler(c mqtt.Client, msg mqtt.Message) {
	log.Printf("unexpected message:  %s\n", msg)
}

func onConnectionLostHandler(c mqtt.Client, err error) {
	log.Println("connection to broker lost")
}

func onRecconnectingHandler(c mqtt.Client, opts *mqtt.ClientOptions) {
	log.Println("attempting to reconnect to broker")
}

func onConnectHandler(c mqtt.Client) {
	log.Println("connected to broker")
}

func main() {
	log.Printf("%s starting\n", moduleName)

	opts := Options{}
	if _, err := flags.Parse(&opts); err != nil {
		log.Fatal(err)
	}

	mqttOpts := mqtt.NewClientOptions()
	mqttOpts.AddBroker(fmt.Sprintf("tcp://%s", opts.Broker))
	mqttOpts.SetClientID(fmt.Sprintf("gowon_%s", moduleName))
	mqttOpts.SetConnectRetry(true)
	mqttOpts.SetConnectRetryInterval(mqttConnectRetryInternal * time.Second)
	mqttOpts.SetAutoReconnect(true)

	mqttOpts.DefaultPublishHandler = defaultPublishHandler
	mqttOpts.OnConnectionLost = onConnectionLostHandler
	mqttOpts.OnReconnecting = onRecconnectingHandler
	mqttOpts.OnConnect = onConnectHandler

	kv, err := bolt.Open(opts.KVPath, 0666, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer kv.Close()

	for _, bucket := range []string{"xboxlive_xuid", "xboxlive_gamertag"} {
		err = kv.Update(func(tx *bolt.Tx) error {
			_, err := tx.CreateBucketIfNotExists([]byte(bucket))
			return err
		})
		if err != nil {
			log.Fatal(err)
		}
	}

	httpClient := req.C().
		SetCommonHeader("x-authorization", opts.APIKey).
		SetCommonHeader("accept", "*/*")

	mr := gowon.NewMessageRouter()
	mr.AddCommand("xbl", genXblHandler(httpClient, kv))
	mr.Subscribe(mqttOpts, moduleName)

	log.Print("connecting to broker")

	c := mqtt.NewClient(mqttOpts)
	if token := c.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}

	log.Print("connected to broker")

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	<-sigs

	log.Println("signal caught, exiting")
	c.Disconnect(mqttDisconnectTimeout)
	log.Println("shutdown complete")
}
