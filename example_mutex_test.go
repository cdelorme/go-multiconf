package gonf_test

import (
	"sync"

	"github.com/cdelorme/gonf"
)

type Safe struct {
	sync.Mutex

	Path    string
	Skip    bool
	HowMany int `json:"number,omitempty"`
}

func (s *Safe) PostProcessing() {
	s.Lock()
	// fix sensitive inputs
	// generate computed fields
	// clear effected caches
	// safely restart dependent services
	s.Unlock()
}

func (s *Safe) Run() {
	// run the applications logic
}

func Example_mutex() {
	app := &Safe{Path: "/tmp/default"}

	c := &gonf.Config{}
	c.Target(app)
	c.Description("A concurrently safe example application")

	c.Add("Path", "Path to run operations in", "APP_PATH", "-p:", "--path")
	c.Add("Skip", "a skippable boolean (false is default)", "APP_SKIP", "-s", "--skip")
	c.Add("number", "number of cycles", "APP_NUMBER", "-n:", "--number")

	c.Example("-p ~/ -sn 3")
	c.Example("--path=~/ --number=3")

	c.Load()
	app.PostProcessing()
	app.Run()
}
