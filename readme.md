
# [gonf](https://github.com/cdelorme/gonf)

An idiomatic go package for standardizing and consolidating application configuration in the form of json file, command line, and environment variables.

It was designed around consistent use of built-in and single-purpose packages to reduce configuration verbosity.


## sales pitch

While written with the same usual goals of at-a-glance comprehension, the main focus is simplicity of implementation with no external dependencies.

**This library:**

- includes black-box unit tests
- has zero transitive dependencies
- enables configuration consistency
- all operations are concurrently safe
- provides a POSIX compliant `getopt` implementation
- enlists sane-defaults for configuration paths by operating system
- provides optional (mutex) locking through the configuration target
- remains under 500 lines of code (_under 1300 if you count comments and tests_)

**For a more comprehensive set of features you should checkout checkout [viper](https://github.com/spf13/viper).**  It's got nearly every bell and whistle, _including the complexity and transitive dependencies._


## design

A simple structure with a set of functions that expose a minimal set of behaviors to keep things simple while being concurrently safe.

To set a `Target()`, pass a pointer to a structure you will use to aggregate configuration.  The file format and parsing process uses json encoding so the structure may use json tags for its properties.

If you wish to enable automated help, set a `Description()`.  Three command line options will be automatically watched for help (`-h`, `--help`, and `help`), and will automatically generate the output using any registered settings (via `Add()`) and examples (via `Example()`).

A [fully POSIX compliant `getopt` implementation](https://en.wikipedia.org/wiki/Getopt) is supplied, with support for an explicit capture (_greedy_) character (`:`) to always capture the content after the option when dealing with single character command line flags where the initial characters in the value matches other registered flags.

The `Add()` function exists to register new properties by name or by json tag, which may have a description, environment variable, and many flags.  Support for deep properties is provided using dot-notation in the name (eg. `parent.child`).  If the name is empty, or both the environment variable and options are empty, an error will be returned.  Similarly if the name has already been registered an error will be returned.  _However, it supports multiple registrations of environment variables and command line options._

The `Help()` function will print the automatically generated information without terminating the application, but only if the description is not empty.

The `Example()` function accepts command line options to demonstrate usage through command line.  _Each is automatically prefixed with the executable name._

Since all input from command line and environment variables are strings by default, this tool leverages reflection against the target to cast to the common json data types.

The `Load()` function acquires all three forms of supported input, and combines them onto the target in the expected order.  All errors are aggregated and returned, _however they will not stop the system from making a best-effort to apply the properties._

The `Reload()` function allows manual reloads, making it trivial to add polling or `sighip` solutions with relative ease.

The package abstracts the configuration file paths, enforcing common standards per operation system.  _When calling `Load()` you can try other file names, or full paths._

While the json specification does not support comments, the system will safely filter comments using the `//` and `/**/` formats from the configuration file prior to parsing it.

When `Load()` is run, it will try all supplied configuration files, setting the one that succeeded as the one to use when `Save()` and `Reload()` are called.  If no file has been found it will combine the first file name supplied with the OS-specific user-path, _unless the first override is an absolute path._

All inputs will be gathered, and applied to the target.  If the target offers functions mutex locking behavior, it will be locked prior to applying configuration settings to it.


**Reasons:**

There are completely logical reasons for all of these implementation details, and I figured it would help to explain them here.

The birth of this package stems from four separate projects I had previously used in many others to handle the three configuration methods.  The fourth package was necessary to combine the data for use.  I learned over the course of a couple years that I never relied on direct properties and always ended up using structures, and by combining them I could both simplify the implementation as well as the verbosity.

Selecting json as the file storage type was mostly to simplify the data types to sanely deal with casting from environment variables and command line options.  The fact that a json package is built in was just an added bonus.

I chose not to use the built in `flag` library because it does not provide a POSIX compatible getopt implementation, _which can turn command line into a verbose mess._

There are many cases where an application may benefit from reconfiguration without actually restarting.  _However, the implementation is best left to the developer due to conflicting opinions on polling versus operating-system limited signals and dealing with post-processing without discarding errors; although an example of each is provided._

For a cross-platform friendly approach to dealing with configuration files the tool checks `%APPDATA%` for windows, `$HOME/Library/Preferences/` for darwin/osx, with a fallback of `$HOME`, `$XDG_CONFIG_HOME` or `$HOME/.config/`.  If the file name is an absolute path it will override the default paths, which is useful when you need full control such as traditional `/etc/` configuration files where services do not have user-space directories.

Support for comments was a whim, and was only added because I thought it might help to allow configuration files to be more descriptive, like most ini style configuration files.  _If there was a built-in `encoding/ini` I would probably have chosen it, but structures would probably not have been mapped as easily._  However, I would never have picked yaml, since it's syntax is too white-space sensitive for safe human modification.

Creating a file on first run is a way of having an application self-document for users by printing its sane defaults in a predictable place so that a user knows what settings are available.  _Obviously if you depend on defaults by type and use `omitempty`, this will be of little benefit._


## usage

Here is a comprehensive example, including mutex locking, and both polling and signal based reloads:

	package main

	import (
		"os"
		"os/signal"
		"runtime"
		"sync"
		"syscall"
		"time"

		"github.com/cdelorme/gonf"
	)

	type Application struct {
		sync.Mutex
		Path    string
		Skip    bool
		HowMany int `json:"number,omitempty"`
	}

	func (a *Application) PostProcessing() {
		a.Lock()
		// fix sensitive inputs
		// generate computed fields
		// clear effected caches
		// safely restart dependent services
		a.Unlock()
	}

	func (a *Application) Run() {
		// begin operations
	}

	func (a *Application) polling(c *gonf.Config) {
		for {
			time.Sleep(1 * time.Minute)
			if c.Reload() == nil {
				a.PostProcessing()
			}
		}
	}

	func (a *Application) sighup(c *gonf.Config) {
		if runtime.GOOS == "windows" {
			return
		}
		s := make(chan os.Signal)
		signal.Notify(s, syscall.SIGHUP)
		go sighup(c)
		for _ = range s {
			if c.Reload() != nil {
				a.PostProcessing()
			}
		}
	}

	func main() {
		app := &Application{Path: "/tmp/default"}

		// create configuration, registering the target and description
		c := &gonf.Config{}
		c.Target(app)
		c.Description("An example application")

		// register properties
		c.Add("Path", "Path to run operations in", "APP_PATH", "-p:", "--path")
		c.Add("Skip", "a skippable boolean (false is default)", "APP_SKIP", "-s", "--skip")
		c.Add("number", "number of cycles", "APP_NUMBER", "-n:", "--number")

		// add examples for help output
		c.Example("-p ~/ -sn 3")
		c.Example("--path=~/ --number=3")

		// load configuration onto the app, then run post-processing
		c.Load()
		app.PostProcessing()

		// setup reload behaviors then run the application logic
		go a.polling(c)
		go a.sighup(c)
		app.Run()
	}

A series of examples are now part of the code:

- [simple](example_test.go)
- [concurrently safe with post processing](example_mutex_test.go)
- [post process and polling reloads](example_polling_test.go)
- [post process and signal reloads](example_signal_test.go)


## tests

This software is fully tested, and tests can be run and checked with:

	go test -v -race -coverprofile=coverage.out
	go tool cover -html=coverage.out


# references

- [go-option](https://github.com/cdelorme/go-option)
- [go-env](https://github.com/cdelorme/go-env)
- [go-config](https://github.com/cdelorme/go-config)
- [go-maps](https://github.com/cdelorme/go-maps)
- [go sighup](https://gist.github.com/andelf/5889946)
- [go method sets](https://golang.org/ref/spec#Method_sets)
- [viper project](https://github.com/spf13/viper)
- [getopt](https://en.wikipedia.org/wiki/Getopt)
- [golang laws of reflection](http://blog.golang.org/laws-of-reflection)
- [reflect get struct tags](https://golang.org/pkg/reflect/#StructTag.Get)
