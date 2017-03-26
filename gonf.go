// This package solves configuration by consolidating command line options,
// environment variables, and json files into a single logical process for
// application configuration.
//
// It enforces standards in order to reduce the complexity per project, and
// to provide a clear non-verbose implementation, while automatically dealing
// with common expectations.
//
// It automatically detects the application name using os.Args[0].  It also
// determines the true path to the application by resolving symbolic links.
//
// It keeps track of two paths as a package global for dealing with
// configuration files; the relative path to the application, and a
// sane default per operating system.  On windows it checks %APPDATA%,
// on mac it checks ~/Library/Preferences, and for the rest it uses
// $XDG_HOME_PATH with a fallback of ~/.config.
//
// Example
//
//	package main
//	import "github.com/cdelorme/gonf"
//	type Application struct {
//		Path string
//		Skip bool
//		HowMany int `json:"number,omitempty"`
//	}
//	func main() {
//		app := &Application{Path: "/tmp/default"}
//		c := gonf.Config{Target: app, Description: "An example application"}
//		c.Add("Path", "Path to run operations in", "APP_PATH", "-p", "--path")
//		c.Add("Skip", "a skippable boolean (false is default)", "APP_SKIP", "-s", "--skip")
//		c.Add("number", "number of cycles", "APP_NUMBER", "-n", "--number")
//		c.Example("-p ~/ -sn 3")
//		c.Example("--path=~/ --number=3")
//		c.Load()
//		// run your applications operations
//	}
package gonf

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

var (
	goos    = runtime.GOOS
	appPath = os.Args[0]
	appName = strings.TrimSuffix(filepath.Base(appPath), filepath.Ext(appPath))
	paths   []string
)

func init() {
	paths = []string{}
	if p, e := filepath.EvalSymlinks(appPath); e == nil {
		if a, e := filepath.Abs(p); e == nil {
			paths = append(paths, filepath.Join(filepath.Dir(a)))
		}
	}

	if appData := os.Getenv("APPDATA"); appData != "" {
		paths = append(paths, filepath.Join(appData, "Roaming"))
	} else if home := os.Getenv("HOME"); home != "" {
		if goos == "darwin" {
			paths = append(paths, filepath.Join(home, "Library", "Preferences"))
		} else if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
			paths = append(paths, xdg)
		} else {
			paths = append(paths, filepath.Join(home, ".config"))
		}
	}
}
