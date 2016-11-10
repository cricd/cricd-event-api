package main

import (
	"io/ioutil"
	"os"
	"testing"
)

type validateJSONTest struct {
	fileName string
	valid    bool
}

var validateJSONTests = []validateJSONTest{
	{"test/good_event.json", true},
	{"test/bad_event.json", false},
}

func TestValidateJSON(t *testing.T) {
	for _, test := range validateJSONTests {
		expected := test.valid
		s, err := ioutil.ReadFile(test.fileName)
		if err != nil {
			panic(err)
		}
		actual := validateJSON(string(s))
		if actual != expected {
			t.Errorf("Failed to validate json for %v expected %v got %v", test.fileName, expected, actual)
		}
	}

}

type mustGetConfigTest struct {
	input  cricdConfig
	output cricdConfig
}

var mustGetConfigTests = []mustGetConfigTest{
	{cricdConfig{"google.com", "1337", "foo", "google.co.nz", "1234"}, cricdConfig{"google.com", "1337", "foo", "google.co.nz", "1234"}},
	{cricdConfig{"", "", "", "", ""}, cricdConfig{"localhost", "2113", "cricket_events_v1", "localhost", "3004"}},
}

func TestMustGetConfig(t *testing.T) {
	for _, test := range mustGetConfigTests {
		inConfig := test.input
		os.Setenv("EVENTSTORE_IP", inConfig.eventStoreURL)
		os.Setenv("EVENTSTORE_PORT", inConfig.eventStorePort)
		os.Setenv("EVENTSTORE_STREAM_NAME", inConfig.eventStoreStreamName)
		os.Setenv("NEXT_BALL_IP", inConfig.nextBallURL)
		os.Setenv("NEXT_BALL_PORT", inConfig.nextBallPort)
		expected := test.output
		mustGetConfig(&inConfig)
		if inConfig != expected {
			t.Errorf("Failed to get config expected %v but got %v", expected, inConfig)
		}
	}
}

type pushToESTest struct {
	eventFile string
	valid     bool
}

var pushToESTests = []pushToESTest{
	{"test/good_event.json", true},
	{"test/bad_event.json", false},
}

func TestPushToES(t *testing.T) {
	var testConfig cricdConfig
	mustGetConfig(&testConfig)
	testClient := mustSetupES(&testConfig)
	for _, test := range pushToESTests {
		s, err := ioutil.ReadFile(test.eventFile)
		if err != nil {
			panic(err)
		}
		expected := test.valid
		uuid, _ := pushToES(&testConfig, testClient, string(s))
		actual := (uuid != "")
		if actual != expected {
			t.Errorf("Failed to push to ES expected %v but got %v", expected, actual)
		}
	}
}

func TestDuplicatesPushToES(t *testing.T) {
	var testConfig cricdConfig
	mustGetConfig(&testConfig)
	testClient := mustSetupES(&testConfig)
	test := pushToESTests[0]
	s, err := ioutil.ReadFile(test.eventFile)
	if err != nil {
		panic(err)
	}
	// Going to push twice
	_, _ = pushToES(&testConfig, testClient, string(s))
	_, err = pushToES(&testConfig, testClient, string(s))
	if err == nil {
		t.Errorf("Expected to get error for duplicate event but didn't")
	}
}

type getNextEventTest struct {
	eventFile   string
	expectEvent bool
}

var getNextEventTests = []getNextEventTest{
	{"test/good_event.json", true},
	{"test/no_match_event.json", false},
	{"test/empty_event.json", false},
}

func TestGetNextEvent(t *testing.T) {
	var testConfig cricdConfig
	mustGetConfig(&testConfig)
	for _, test := range getNextEventTests {
		s, err := ioutil.ReadFile(test.eventFile)
		if err != nil {
			panic(err)
		}
		nextEvent, err := getNextEvent(&testConfig, s)
		gotEvent := (nextEvent != "")
		if gotEvent != test.expectEvent {
			t.Errorf("Failed to get next event expected %v but got %v", test.expectEvent, gotEvent)
		}
	}

}
