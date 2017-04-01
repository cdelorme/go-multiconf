package gonf_test

import (
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/cdelorme/gonf"
)

type Signal struct {
	Path    string
	Skip    bool
	HowMany int `json:"number,omitempty"`
}

func (s *Signal) PostProcessing() {
	// fix sensitive inputs
	// generate computed fields
	// clear effected caches
	// safely restart dependent services
}

func (a *Signal) Run() {
	// run the applications logic
}

func (s *Signal) sighup(c *gonf.Config) {
	if runtime.GOOS == "windows" {
		return
	}
	h := make(chan os.Signal)
	signal.Notify(h, syscall.SIGHUP)
	for _ = range h {
		if c.Reload() == nil {
			s.PostProcessing()
		}
	}
}

func Example_signal() {
	app := &Signal{Path: "/tmp/default"}

	c := &gonf.Config{}
	c.Target(app)
	c.Description("An example application with signal reloads")

	c.Add("Path", "Path to run operations in", "APP_PATH", "-p:", "--path")
	c.Add("Skip", "a skippable boolean (false is default)", "APP_SKIP", "-s", "--skip")
	c.Add("number", "number of cycles", "APP_NUMBER", "-n:", "--number")

	c.Example("-p ~/ -sn 3")
	c.Example("--path=~/ --number=3")

	c.Load()
	app.PostProcessing()
	go app.sighup(c)
	app.Run()
}
