package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"
	es "github.com/jetbasrawi/go.geteventstore"
	"github.com/patrickmn/go-cache"
	"github.com/xeipuuv/gojsonschema"
)

type cricdConfig struct {
	eventStoreURL        string
	eventStorePort       string
	eventStoreStreamName string
	nextBallURL          string
	nextBallPort         string
}

var config cricdConfig
var client *es.Client
var c = cache.New(5*time.Minute, 30*time.Second)

func validateJSON(event string) bool {
	s, err := ioutil.ReadFile("./event_schema.json")
	if err != nil {
		log.WithFields(log.Fields{"value": err}).Fatal("Unable to load json schema")
	}
	schemaLoader := gojsonschema.NewBytesLoader(s)
	documentLoader := gojsonschema.NewStringLoader(event)

	result, err := gojsonschema.Validate(schemaLoader, documentLoader)
	if err != nil {
		log.WithFields(log.Fields{"value": err}).Fatal("Unable to validate json schema for event")
	}

	if result.Valid() {
		return true
	}
	return false

}

func mustGetConfig(config *cricdConfig) {
	esURL := os.Getenv("EVENTSTORE_IP")
	if esURL != "" {
		config.eventStoreURL = esURL
	} else {
		log.WithFields(log.Fields{"value": "EVENTSTORE_IP"}).Info("Unable to find env var, using default `localhost`")
		config.eventStoreURL = "localhost"
	}

	esPort := os.Getenv("EVENTSTORE_PORT")
	if esPort != "" {
		config.eventStorePort = esPort
	} else {
		log.WithFields(log.Fields{"value": "EVENTSTORE_PORT"}).Info("Unable to find env var, using default `2113`")
		config.eventStorePort = "2113"
	}

	esStreamName := os.Getenv("EVENTSTORE_STREAM_NAME")
	if esStreamName != "" {
		config.eventStoreStreamName = esStreamName
	} else {
		log.WithFields(log.Fields{"value": "EVENTSTORE_STREAM_NAME"}).Info("Unable to find env var, using default `cricket_events_v1`")
		config.eventStoreStreamName = "cricket_events_v1"
	}

	nextBallURL := os.Getenv("NEXT_BALL_IP")
	if nextBallURL != "" {
		config.nextBallURL = nextBallURL
	} else {
		log.WithFields(log.Fields{"value": "NEXT_BALL_IP"}).Info("Unable to find env var, using default `localhost`")
		config.nextBallURL = "localhost"
	}

	nextBallPort := os.Getenv("NEXT_BALL_PORT")
	if nextBallPort != "" {
		config.nextBallPort = nextBallPort
	} else {
		log.WithFields(log.Fields{"value": "NEXT_BALL_PORT"}).Info("Unable to find env var, using default `3004`")
		config.nextBallPort = "3004"
	}
}

// TODO: Alter this to pass a point to the es.Client rather than return.
func mustSetupES(config *cricdConfig) *es.Client {
	client, err := es.NewClient(nil, "http://"+config.eventStoreURL+":"+config.eventStorePort)
	if err != nil {
		log.WithFields(log.Fields{"value": err}).Fatal("Unable to create ES connection")
	}
	return client
}

func pushToES(config *cricdConfig, esClient *es.Client, event string) (string, error) {
	valid := validateJSON(event)
	if !valid {
		log.WithFields(log.Fields{"value": event}).Error("Invalid JSON for event and cannot push to ES")
		return "", errors.New("Unable to send to ES due to invalid JSON")
	}

	// Store cache
	key := base64.StdEncoding.EncodeToString([]byte(event))
	_, found := c.Get(key)
	if found {
		log.WithFields(log.Fields{"value": key}).Error("Event already received in the last 5 minutes")
		return "", errors.New("Received this event in the last 5 minutes")
	}
	c.Set(key, &event, cache.DefaultExpiration)

	uuid := es.NewUUID()
	myESEvent := es.NewEvent(uuid, "cricket_event", event, nil)

	// Create a new StreamWriter
	writer := esClient.NewStreamWriter(config.eventStoreStreamName)
	err := writer.Append(nil, myESEvent)
	if err != nil {
		// Handle errors
		log.WithFields(log.Fields{"value": err}).Error("Unable to push event to ES")
		return "", err
	}

	return uuid, nil

}

func getNextEvent(config *cricdConfig, event []byte) (string, error) {
	// Get the match ID from the json event
	var f interface{}
	err := json.Unmarshal(event, &f)
	if err != nil {
		log.WithFields(log.Fields{"value": err}).Errorf("Unable to unmarshall event %v", event)
	}
	if f == nil {
		log.WithFields(log.Fields{"value": err}).Errorf("Unable to get match id")
		return "", err
	}
	m := f.(map[string]interface{})
	match := m["match"]
	if match == nil {
		log.WithFields(log.Fields{"value": err}).Errorf("Unable to get match id")
		return "", err
	}
	matchString := strconv.FormatFloat(match.(float64), 'E', -1, 64)

	url := "http://" + config.nextBallURL + ":" + config.nextBallPort
	resp, err := http.Get(url + "?match=" + matchString)
	if err != nil {
		log.WithFields(log.Fields{"value": err}).Errorf("Unable to get next event")
		return "", err
	}
	nextEvent, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.WithFields(log.Fields{"value": err}).Errorf("Unable to parse next event")
		return "", err
	}
	return string(nextEvent), nil
}

func main() {
	mustGetConfig(&config)
	client = mustSetupES(&config)
	router := mux.NewRouter().StrictSlash(true)
	router.HandleFunc("/event", postEventHandler).Methods("POST")
	http.ListenAndServe(":4567", router)

}

func postEventHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")

	event, err := ioutil.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(400)
		fmt.Fprintf(w, "Unable to read event")
		log.WithFields(log.Fields{"value": err}).Fatalf("Unable to read event %v", string(event))
		return
	}
	uuid, err := pushToES(&config, client, string(event))
	if err != nil {
		w.WriteHeader(500)
		fmt.Fprintf(w, "Unable to push event to ES")
		return
	}
	if uuid == "" {
		w.WriteHeader(500)
		fmt.Fprintf(w, "Internal server error")
		return
	}

	nextEvent, err := getNextEvent(&config, event)
	if nextEvent != "" {
		w.WriteHeader(201)
		fmt.Fprintf(w, nextEvent)
		return
	}

	w.WriteHeader(201)
	log.WithFields(log.Fields{"value": uuid}).Info("Successfully pushed event to ES")
	return
}
