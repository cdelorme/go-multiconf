package main

// run with `LOG_LEVEL=debug`

import (
	"github.com/cdelorme/go-log"
	"github.com/cdelorme/gonf"
)

type program struct {
	Name string `json:"name,omitempty"`
	log.Logger
}

func main() {
	p := &program{}

	g := gonf.Gonf{
		Description:   "demonstration program for gonf",
		Configuration: p,
	}

	g.Add("name", "name of the program", "GONF_DEMO_NAME", "-n", "--name")
	g.Example("-n example")
	g.Load()
}
