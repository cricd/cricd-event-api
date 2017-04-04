package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"

	cricd "github.com/cricd/cricd-go"

	log "github.com/Sirupsen/logrus"
	es "github.com/cricd/es"
	"github.com/gorilla/mux"
)

type cricdEventConfig struct {
	nextBallURL  string
	nextBallPort string
}

var config cricdEventConfig
var client es.CricdESClient

func (config *cricdEventConfig) useDefault() {
	nbURL := os.Getenv("NEXT_BALL_IP")
	if nbURL != "" {
		config.nextBallURL = nbURL
	} else {
		log.WithFields(log.Fields{"value": "NEXT_BALL_IP"}).Info("Unable to find env var, using default `localhost`")
		config.nextBallURL = "localhost"
	}

	nbPort := os.Getenv("NEXT_BALL_PORT")
	if nbPort != "" {
		config.nextBallPort = nbPort
	} else {
		log.WithFields(log.Fields{"value": "NEXT_BALL_PORT"}).Info("Unable to find env var, using default `3004`")
		config.nextBallPort = "3004"
	}
}

func getNextEvent(config *cricdEventConfig, event []byte) (string, error) {
	log.Info("Trying to get next event")
	// Get the match ID from the json event
	var f interface{}
	err := json.Unmarshal(event, &f)
	if err != nil {
		log.WithFields(log.Fields{"value": err}).Errorf("Unable to unmarshall event %v", event)
	}
	if f == nil {
		log.WithFields(log.Fields{"value": err}).Errorf("Unmarshalled next event to empty interface")
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
	config.useDefault()
	client.UseDefaultConfig()
	ok := client.Connect()
	if !ok {
		log.Panicln("Unable to connect to EventStore")
	}
	router := mux.NewRouter().StrictSlash(true)
	router.HandleFunc("/event", eventHandler).Methods("POST")
	http.ListenAndServe(":4567", router)

}

func eventHandler(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "application/json; charset=UTF-8")

	event, err := ioutil.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(400)
		fmt.Fprintf(w, "Unable to read event")
		log.WithFields(log.Fields{"value": err}).Errorf("Unable to read event from request")
		return
	}
	var cd cricd.Delivery
	err = json.Unmarshal(event, &cd)
	if err != nil {
		w.WriteHeader(500)
		log.WithFields(log.Fields{"value": err}).Errorf("Failed to unmarshal event to a cricd Delivery")
		fmt.Fprintf(w, "Failed to unmarshal event %v", err)
		return
	}

	uuid, err := client.PushEvent(cd, true)
	if err != nil {
		w.WriteHeader(500)
		log.WithFields(log.Fields{"value": err}).Errorf("Failed to push event to ES")
		fmt.Fprintf(w, "Failed to push event %v", err)
		return
	}
	if uuid == "" {
		w.WriteHeader(500)
		log.Errorf("Failed to push event without error")
		fmt.Fprintf(w, "Internal server error")
		return
	}

	nextEvent, _ := getNextEvent(&config, event)
	if nextEvent != "" {
		w.WriteHeader(201)
		fmt.Fprintf(w, nextEvent)
		return
	}

	w.WriteHeader(201)
	log.WithFields(log.Fields{"value": uuid}).Info("Successfully pushed event to ES")
	return
}
