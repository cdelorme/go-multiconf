package gonf_test

import (
	"github.com/cdelorme/gonf"
)

type Simple struct {
	Path    string
	Skip    bool
	HowMany int `json:"number,omitempty"`
}

func (s *Simple) Run() {
	// run the applications logic
}

func Example() {
	app := &Simple{Path: "/tmp/default"}

	c := &gonf.Config{}
	c.Target(app)
	c.Description("A simple example application")

	c.Add("Path", "Path to run operations in", "APP_PATH", "-p:", "--path")
	c.Add("Skip", "a skippable boolean (false is default)", "APP_SKIP", "-s", "--skip")
	c.Add("number", "number of cycles", "APP_NUMBER", "-n:", "--number")

	c.Example("-p ~/ -sn 3")
	c.Example("--path=~/ --number=3")

	c.Load()
	app.Run()
}
