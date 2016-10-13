
# [gonf](https://github.com/cdelorme/gonf#)

An idiomatic go utility for standardizing and consolidating application configuration.


## sales pitch

This library consolidates three forms of configuration into a single tidy package; file, cli options, and environment variables.

The aim is to reduce cognitive load on developers by providing a minimal code base (_< 400 lines of code_), a limited number of exposed operations, and no transitive dependencies.

It comes with a complete suite of unit tests, and an easy-to-use setup defined in the readme.  All behavior is clearly defined and explained in the readme as well.

**For a more comprehensive set of features, you should checkout checkout [viper](https://github.com/spf13/viper).**  It's got nearly every bell and whistle, and all the transitive dependencies (eg. risk) that go with all of them.


## usage

Import the library:

	import "github.com/cdelorme/gonf"

Define a configuration struct, _optionally with a `GoString()` function for safe printing_:

	type MyConf struct { Name string }
	func (self MyConf) GoString() string { return "custom format" }

Create an instance of `gonf`, and set your configuration object with default values:

	c := &MyConf{Name: "Default"}
	config := gonf.Gonf{Description: "I want help automated", Configuration: c}

Supplying your configuration struct to the config instance will automatically populate and merge all the settings from cli options, environment variables, and configuration file data.

_Setting a `Description` enables the builtin `help` support which captures `help`, `-h`, and `--help` in cli flags and auto-prints to `stdout` before terminating._  _To supplement the help system, you can add examples of your utility:_

	config.Example("-o /path/to/output")

To deal with misconfiguration later you can call `Help()` directly to print usage information without automatically closing the application.

Register cli options and environment variables like this:

	config.Env("name", "optional description of name", "MYAPP_CONFIG_NAME")
	config.Option("name", "optional description of name", "-n", "--name")

You can even add keys with depth via period delimited values:

	config.Env("try.depth", "demo depth", "MYAPP_CONFIG_DEPTH")
	config.Option("try.depth", "demo depth", "-d", "--depth")

Finally, you can load all configuration in a single command (_optionally supplying alternative file paths_):

	config.Load()

_Your configuration object `c` from earlier will now be populated and you can begin using it._


## tests

This software comes with a complete suite of isolated unit tests and full coverage that can be validated with:

	go test -v -race -coverprofile=coverage.out
	go tool cover -html=coverage.out


## design

The design of this project was inspired over the past year using separate projects I had constructed to manage each facet of configuration independently.  Over time I realized that I always ended up with the same structure for loading configuration, but the result was a bit messy and complicated due to all the manipulation.

I concluded that since simplicity is favorable, merging all three into a single library that behaves following a common standard I would end up with a much cleaner setup in my applications that is also more intuitive.

Reducing settings and exposed functions, eliminating complicated optional type casting, and consolidating the three inputs results in a much cleaner solution.


### implementation

At a fundamental level, this application consolidates three methods of configuration input:

- cli options
- environment variables
- file

Configuration is prioritized in that order by standards; meaning cli options are used over environment variables, and environment variables supersede file based configuration.

Being cross-platform friendly this system looks in relative paths, `%APPDATA%`, and `XDG_CONFIG_DIRS` (or `$HOME`), when seeking configuration for the application.  _We've skipped OSX because most `~/Library/` and `/Library/` entries are for Cocoa and are generally not standard text._

A fully [posix compliant `getopt` implementation](https://en.wikipedia.org/wiki/Getopt) is supplied, with support for an explicit capture character (`:`) to always capture the content after the flag while safely dealing with whitespace.

Intelligent automatically generated help messages are provided, depending on whether `Description` has been set on the `Config` instance.  This allows an intuitive activation and will capture `-h`, `--help`, and simply `help`, to display generated help, followed by running `os.Exit(0)`.

All environment variables and cli options are loaded as strings, so we've added the `reflect` package to help identify the configuration object and cast the fields to the correct types before merging the final map with your structure.  _This is a trivial fix as json only supports three main data types and we only need to concern ourselves with one unexpected input format._  We silently discard failed casts in-line with the `json.Unmarshal` behavior.


## future

In the future I would like to make the following changes:

- eliminate `Configuration` interface in favor of just `interface{}`
	- replace with dynamic interface assertions for greater flexibility
- remove logger dependency and implementation (assertions to optionally log)
- enhance or simplify options, `getopt` parsing, and the associated help system
- strip comments from json configuration files
- add support for `sighup` reloads


## discarded

I have decided not to support the following:

- separate operation to register default values
- support for functions by type when registering cli and env settings
- reloading configuration file on change

_Since we are working with a configuration `struct`, all default values can be set on the object directly from the start._

_Types should be enforced by the `struct` and we have implemented the `reflect` package to automatically handle casting._

_Automatic reloads requires a transient dependency for true cross-platform support otherwise it requires a polling solution; it introduces a new level of complexity regarding tracking of multiple file paths as configuration path is variable; finally we may not want to run the callback when a reload fails and that adds yet another one-off behavior._  Overall, the complexity of this solution is not one I wish to burden my software with at this time.


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
