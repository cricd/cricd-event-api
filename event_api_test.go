package main

import (
	"io/ioutil"
	"os"
	"testing"

	es "github.com/cricd/es"
)

type useDefaultTest struct {
	input  cricdEventConfig
	output cricdEventConfig
}

var useDefaultTests = []useDefaultTest{
	{cricdEventConfig{"google.com", "1234"}, cricdEventConfig{"google.com", "1234"}},
	{cricdEventConfig{"", ""}, cricdEventConfig{"localhost", "3004"}},
}

func TestUseDefault(t *testing.T) {
	for _, test := range useDefaultTests {
		inConfig := test.input
		os.Setenv("NEXT_BALL_IP", inConfig.nextBallURL)
		os.Setenv("NEXT_BALL_PORT", inConfig.nextBallPort)
		expected := test.output
		inConfig.useDefault()
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

func TestDuplicatesPushToES(t *testing.T) {
	var testConfig cricdEventConfig
	var testClient es.CricdESClient
	testClient.UseDefaultConfig()
	testConfig.useDefault()
	testClient.Connect()
	test := pushToESTests[0]
	s, err := ioutil.ReadFile(test.eventFile)
	if err != nil {
		panic(err)
	}
	// Going to push twice
	_, _ = testClient.PushEvent(string(s), true)
	_, err = testClient.PushEvent(string(s), true)
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
	var testConfig cricdEventConfig
	testConfig.useDefault()
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
