package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
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

func readFromES(streamName string, esClient *es.Client) ([]interface{}, error) {
	reader := client.NewStreamReader(streamName)
	var allEvents []interface{}
	for reader.Next() {
		if reader.Err() != nil {
			switch err := reader.Err().(type) {

			case *url.Error, *es.ErrTemporarilyUnavailable:
				log.WithFields(log.Fields{"value": err}).Error("Server temporarily unavailable, retrying in 30s")
				<-time.After(time.Duration(30) * time.Second)

			case *es.ErrNotFound:
				log.WithFields(log.Fields{"value": err}).Error("Stream does not exist")
				return nil, errors.New("Unable to read from stream that does not exist")

			case *es.ErrUnauthorized:
				log.WithFields(log.Fields{"value": err}).Error("Unauthorized request")
				return nil, errors.New("Unauthorized to access ES")

			case *es.ErrNoMoreEvents:
				return allEvents, nil
			default:
				log.WithFields(log.Fields{"value": err}).Error("Unknown error occurred when reading from ES")
				return nil, errors.New("Unknown error occurred when reading from ES")
			}
		}
		var eventData interface{}
		var eventMeta interface{}
		err := reader.Scan(&eventData, &eventMeta)
		if err != nil {
			log.WithFields(log.Fields{"value": err}).Error("Unable to deserialize event")
			return nil, errors.New("Unable to deserialize event from ES")
		}
		allEvents = append(allEvents, eventData)
	}
	return allEvents, nil
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
	router.HandleFunc("/event", eventHandler).Methods("POST", "GET")
	http.ListenAndServe(":4567", router)

}

func eventHandler(w http.ResponseWriter, r *http.Request) {

	switch r.Method {
	case "GET":
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		streamName := r.URL.Query().Get("match")
		if streamName == "" {
			w.WriteHeader(400)
			fmt.Fprintf(w, "Requires a 'match' parameter")
			log.Errorf("Request sent without a match parameter")
			return
		}
		events, err := readFromES(streamName, client)
		if err != nil {
			w.WriteHeader(500)
			fmt.Fprintf(w, "Internal server error")
			log.WithFields(log.Fields{"value": err}).Errorf("Unable to read from ES")
			return
		}

		jsonEvents, err := json.Marshal(events)
		if err != nil {
			w.WriteHeader(500)
			fmt.Fprintf(w, "Internal server error")
			log.WithFields(log.Fields{"value": err}).Errorf("Unable to marshal JSON")
			return
		}
		// TODO: GZIP?
		w.WriteHeader(200)
		fmt.Fprintf(w, string(jsonEvents))
		return

	case "POST":
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")

		event, err := ioutil.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(400)
			fmt.Fprintf(w, "Unable to read event")
			log.WithFields(log.Fields{"value": err}).Errorf("Unable to read event")
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
}
