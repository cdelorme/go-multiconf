package gonf_test

import (
	"time"

	"github.com/cdelorme/gonf"
)

type Polling struct {
	Path    string
	Skip    bool
	HowMany int `json:"number,omitempty"`
}

func (p *Polling) PostProcessing() {
	// fix sensitive inputs
	// generate computed fields
	// clear effected caches
	// safely restart dependent services
}

func (p *Polling) Run() {
	// run the applications logic
}

func (p *Polling) polling(c *gonf.Config) {
	for {
		time.Sleep(1 * time.Minute)
		if c.Reload() == nil {
			p.PostProcessing()
		}
	}
}

func Example_polling() {
	app := &Polling{Path: "/tmp/default"}

	c := &gonf.Config{}
	c.Target(app)
	c.Description("An example application with polling reloads")

	c.Add("Path", "Path to run operations in", "APP_PATH", "-p:", "--path")
	c.Add("Skip", "a skippable boolean (false is default)", "APP_SKIP", "-s", "--skip")
	c.Add("number", "number of cycles", "APP_NUMBER", "-n:", "--number")

	c.Example("-p ~/ -sn 3")
	c.Example("--path=~/ --number=3")

	c.Load()
	app.PostProcessing()
	go app.polling(c)
	app.Run()
}
