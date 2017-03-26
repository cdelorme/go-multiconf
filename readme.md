
# [gonf](https://github.com/cdelorme/gonf)

An idiomatic go package for standardizing and consolidating application configuration in the form of json file, command line, and environment variables.

It was designed around consistent use of built-in and single-purpose packages to reduce configuration verbosity.


## sales pitch

While written with the same usual goals of at-a-glance comprehension, the main focus is simplicity of implementation with no external dependencies.

**This library:**

- has zero transitive dependencies
- enables configuration consistency
- all operations are concurrently safe
- features sighup on supporting platforms
- comes with a feature-complete suite of unit tests
- provides POSIX compliant `getopt` command line options
- provides optional logging through the configuration target
- provides optional mutex locking through the configuration target
- enlists sane-defaults for configuration paths by operating system
- remains under 500 lines of code (_under 1200 if you count comments and tests_)

**For a more comprehensive set of features you should checkout checkout [viper](https://github.com/spf13/viper).**  It's got nearly every bell and whistle, _including the complexity and transitive dependencies._


## design

An idiomatic structure, with a minimal set of exposed properties and functions to keep things simple for developers.

A pointer to a structure is expected as the `Target`.

If a `Description` is set, it implies automated help options and message generation using the registered settings and examples.

The file format used is json, and the `Target` may use json tags for its properties.

A [fully POSIX compliant `getopt` implementation](https://en.wikipedia.org/wiki/Getopt) is supplied, with support for an explicit capture character (`:`) to always capture the content after the option while safely dealing with whitespace.

A single function to register new properties by name or by json tag, which may have a description, environment variable, and many flags.  If no environment variable value or flags are supplied, it will fail and return without modifying anything.  Support for deep properties is provided using dot-notation (eg. `parent.child`).

A function to display the automatically generated help message on demand, but without terminating the application.

A function to register examples used when generating the help message.

Since all input from command line and environment variables are strings by default, this tool leverages reflection against the `Target` to cast to the common json data types.

A single function to execute the entire parsing and file loading process, as well as to register a `sighup` listener on compatible platforms so that the system can reload configuration from its file without needing to be completely restarted.

A function to manually reload, makes it trivial to add polling or on-demand reload behaviors, which is the same operation a `sighup` triggers.

The package abstracts the configuration file paths so that it can enforce common standards.  While you can override the file name, the path chosen is either relative to the application, or specific to each operating system.

While the json specification does not support comments, the system will safely filter comments using the `//` and `/**/` formats from the configuration file prior to parsing it.

If no file exists, one will be created by combining the OS-specific path with the first filename (eg. the default, or the first name supplied to `Load()`).

If the `Target` offers functions for logging, mutex, or a Callback, they will be executed according to the situation.  Prior to unmarshalling changes, the mutex locking will be used.  Logging will be used when errors are encountered, as well as after the configuration has been applied.  Finally, it will run `Callback()` once finished, allowing for any post-processing to be run.


**Reasons:**

There are completely logical reasons for all of these implementation details, and I figured it would help to explain them here.

The birth of this package stems from four separate projects I had previously used in many others to handle the three configuration methods.  The fourth package was necessary to combine the data for use.  I learned over the course of a couple years that I never relied on direct properties and always ended up using structures, and by combining them I could both simplify the implementation as well as the verbosity.

Selecting json as the file storage type was mostly to simplify the data types to sanely deal with casting from environment variables and command line options.  The fact that a json package is built in was just an added bonus.

I chose not to use the built in `flag` library because it does not provide a POSIX compatible getopt implementation, _which can turn command line into a verbose mess._

I ran into a few projects that would have benefited from the capacity to easily reload configuration without completely restarting.  This called for concurrency safe behavior, _but was not really possible on Windows so I provided a `Reload()` function._

It would be relatively trivial to create a polling mechanism, which will only read the file when the modified time has changed, which is why I have not included it as part of the package:

	go func(c) {
		for {
			time.Sleep(1 * time.Minute)
			c.Reload()
		}
	}()

For a cross-platform friendly approach to dealing with configuration files the tool checks `%APPDATA%` for windows, `$HOME/Library/Preferences/` for darwin/osx, with a linux fallback of `$HOME`, `$XDG_CONFIG_HOME` or `$HOME/.config/`.  _Support for path overrides may be added in the future, but I have yet to encounter a particular need that wasn't for a specific platform (eg. linux or unix using `/etc/` traditionally for services)._

Support for comments was a whim, and was only added because I thought it might help to allow configuration files to include comments (like most ini style configuration files).  _If there was a built-in `encoding/ini` I would probably have chosen it, but object mapping would not have easily been mapped._  However, I would never have picked yaml, since it's syntax is too white-space sensitive for human modification.

Creating a file on first run is a way of having an application self-document for users by printing its sane defaults in a predictable place so that a user knows what settings are available.  _Obviously if you depend on defaults by type and use `omitempty`, this will be of little benefit._

While the term `Callback` may carry a negative connotation from javascript, the intended function is to allow your application to respond to reload events that may occur asynchronously.  Things like post-processing of the configuration values, invalidating cache, and loading in the new settings to restart or transition execution.


## usage

Here is an example of using this package:

	package main

	import "github.com/cdelorme/gonf"

	type Application struct {
		Path string
		Skip bool
		HowMany int `json:"number,omitempty"`
	}

	func main() {
		app := &Application{Path: "/tmp/default"}

		c := gonf.Config{Target: app, Description: "An example application"}
		c.Add("Path", "Path to run operations in", "APP_PATH", "-p", "--path")
		c.Add("Skip", "a skippable boolean (false is default)", "APP_SKIP", "-s", "--skip")
		c.Add("number", "number of cycles", "APP_NUMBER", "-n", "--number")
		c.Example("-p ~/ -sn 3")
		c.Example("--path=~/ --number=3")
		c.Load()

		// run your applications operations
	}


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
