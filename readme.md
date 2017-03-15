
# [gonf](https://github.com/cdelorme/gonf#)

An idiomatic go utility for standardizing and consolidating application configuration based on experiences using independent utilities for each type of configuration.


## sales pitch

This library consolidates three forms of configuration (_file, cli options, and environment variables_) into a single tidy package with minimal exposed functions, under 500 lines of code, and zero transitive dependencies.

It comes with a complete suite of unit tests, as well as instructions further down in this document.

**For a more comprehensive set of features you should checkout checkout [viper](https://github.com/spf13/viper).**  It's got nearly every bell and whistle, _including the risks of transitive dependencies._


## design

Over the course of two years I found that I was always implementing the same configuration approach using my three libraries in all of my applications, and realized that by combining them into a single import I could greatly reduce the verbosity and complexity.

I also began to recognize configuration anti-patterns that I ran into, and took those into account when designing this solution after concluding that simplicity is generally best.

The result is a highly intuitive and much cleaner solution, which resolves type casting concerns, and exists as a single import with no transitive dependencies.


### implementation

At a fundamental level, this application consolidates three methods of configuration input:

- cli options
- environment variables
- file

Configuration is prioritized in that order by standards; meaning cli options are used over environment variables, and environment variables supersede file based configuration.

Being cross-platform friendly it checks `%APPDATA%` for windows, checks for darwin/osx to use `~/Library/Preferences/`, and finally looks for `$HOME` and `$XDG_CONFIG_HOME` with a fallback to `$HOME/.config/`).  _If none of those are matched, it uses the path relative to the application._

A [fully posix compliant `getopt` implementation](https://en.wikipedia.org/wiki/Getopt) is supplied, with support for an explicit capture character (`:`) to always capture the content after the flag while safely dealing with whitespace.

Intelligent automatically generated help messages are provided, depending on whether `Description` has been set on the `Config` instance.  This allows an intuitive activation and will capture `-h`, `--help`, and simply `help`, to display generated help, followed by running `os.Exit(0)`.  _You can print this help at runtime without exiting the application by calling `Help()`._

All environment variables and cli options are loaded as strings, so we've added the `reflect` package to help identify the configuration object and cast the fields to the correct types before merging the final map with your structure.  _This is a trivial fix as json only supports three main data types and we only need to concern ourselves with casting from `string`._  We silently discard failed casts in-line with the `json.Unmarshal` behavior.

The source activates sighup by default on non-windows platforms.  _Users can disable registered signals with `os/signal.Reset()`.  Between complexity and race conditions it was decided to omit polling._  For users who want to implement file-watch, polling, or other custom behaviors `Gonf` now exposes `Reload()` and `ConfigFile()`.

When a file has successfully been loaded it is automatically run through a filter that eliminates single and multi-line comments in `//` and `/**/` formats, and correctly skips starting sequences inside quotes.


## usage

Import the library:

	import "github.com/cdelorme/gonf"

Define a configuration structure, _optionally with a `GoString()` function for safe printing_:

	type MyConf struct { Name string }
	func (self MyConf) GoString() string { return "custom format" }

Create an instance of `Gonf` and your configuration structure, adding it by reference:

	c := &MyConf{Name: "Default"}
	g := gonf.Gonf{Description: "I want help automated", Configuration: c}

_Setting a `Description` will enable built-in `help` support (capturing `help`, `--help` and `-h` by default and terminating the application)._  To supplement the help message you can add examples which will automatically prefix with the executable name:

	g.Example("-o /path/to/output")

To deal with misconfiguration at a later time you can choose to call `Help()` directly to print usage information without automatically closing the application.

Next you can register all settings like this:

	g.Add("Name", "What's in a name?", "NAME_ENV_VAR", "-n", "--name")
	g.Add("deep.prop", "period delimited depth support", "DEEP_PROP", "-d", "--deep-prop")

_This combines both environment variable and cli option registration into a single call, associating both with the same description and property name._  You may choose to omit cli options or use an empty string for the environment variable.

The configuration supports composition; you can use period delimited keys to set depth.

**The application will not stop you from registering the same keys multiple times.**

You can use the default application name for the configuration file, _or you can supply an override including a path:_

	g.Load("midifed-path/custom-filename.extension")

After execution, all settings will have been merged into your original configuration structure.

If you wish you can trigger a reload which will pull changes from a modified configuration file, and run the configuration callback:

	g.Reload()

_If you want to know the file used to store the configuration, you can get it with `g.ConfigFile()`._

This solution provides dynamic support for `Debug`, and `Info` logging with expected signatures, as well as `Lock` and `Unlock` for mutex/concurrency support, and finally will run `Callback()` if available.  _The `Gonf` structure itself is concurrency safe, which means it can be manipulated at runtime without risk of race conditions and subsequent panics._


## tests

This software comes with a complete suite of isolated unit tests and full coverage that can be validated with:

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
