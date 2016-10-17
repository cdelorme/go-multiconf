
# [gonf](https://github.com/cdelorme/gonf#)

An idiomatic go utility for standardizing and consolidating application configuration based on experiences using independent utilities for each type of configuration.


## sales pitch

This library consolidates three forms of configuration (_file, cli options, and environment variables_) into a single tidy package with minimal exposed functions, no more than 400 lines of code, and zero transitive dependencies.

It comes with a complete suite of unit tests, as well as instructions further down in this document.

**For a more comprehensive set of features you should checkout checkout [viper](https://github.com/spf13/viper).**  It's got nearly every bell and whistle, _and all the transitive dependencies (eg. risk) that go with them._


## design

Over the course of two years I found that I was always implementing the same configuration approach in all of my applications, and realized that by combining them into a single import I could greatly reduce the verbosity.

I also began to recognize configuration anti-patterns that I ran into, and took those into account when designing this solution having concluded that simplicity is best.

The result is a highly intuitive and much cleaner solution, which resolves type casting concerns, and exists as a single import with no transitive dependencies.


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


## usage

Import the library:

	import "github.com/cdelorme/gonf"

Define a configuration structure, _optionally with a `GoString()` function for safe printing_:

	type MyConf struct { Name string }
	func (self MyConf) GoString() string { return "custom format" }

Create an instance of `Gonf` and your configuration structure, adding it by reference:

	c := &MyConf{Name: "Default"}
	config := gonf.Gonf{Description: "I want help automated", Configuration: c}

_Setting a `Description` will enable built-in `help` support (capturing `help`, `--help` and `-h` by default and terminating the application)._  To supplement the help message you can add examples which will automatically prefix with the executable name:

	config.Example("-o /path/to/output")

To deal with misconfiguration at a later time you can choose to call `Help()` directly to print usage information without automatically closing the application.

Next you can register all settings like this:

	config.Add("Name", "What's in a name?", "NAME_ENV_VAR", "-n", "--name")
	config.Add("deep.prop", "period delimited depth support", "DEEP_PROP", "-d", "--deep-prop")

_This combines both environment variable and cli option registration into a single call, associating both with the same description and property name._  You may choose to omit cli options or use an empty string for the environment variable.

The configuration supports composition; you can use period delimited keys to set depth.

**The application will not stop you from registering the same keys multiple times.**

By default the system will use `XDG` or `APP_DATA` environment variables by platform to look for configuration files named after the application.  _You can supply an override during the final step:_

	config.Load("/optional/config/path/override.conf")

After execution, all settings will have been merged into your original configuration structure.

This solution provides dynamic support for `Debug`, and `Info` logging with expected signatures, as well as `Lock` and `Unlock` for mutex/concurrency support, and finally will run `Callback()` if available.


## tests

This software comes with a complete suite of isolated unit tests and full coverage that can be validated with:

	go test -v -race -coverprofile=coverage.out
	go tool cover -html=coverage.out


## future

In the future I would like to make the following changes:

- add `SIGHUP` and polling options for automatic configuration file reloading
- strip comments from `json` configuration files


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
