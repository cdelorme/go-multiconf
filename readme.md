
# [go-multiconf](https://github.com/cdelorme/go-multiconf)

An idiomatic go utility for standardizing and consolidating application configuration.


## sales pitch

This library consolidates three forms of configuration into a single tidy package; file, cli options, and environment variables.

The aim is to reduce cognitive load on developers by providing a minimal code base (_< 350 lines of code_), a limited number of exposed operations, and no transitive dependencies.

It comes with a complete suite of unit tests, and an easy-to-use setup defined in the readme.  All behavior is clearly defined and explained in the readme as well.

**For a more comprehensive set of features, you should checkout checkout [viper](https://github.com/spf13/viper).**  It's got nearly every bell and whistle, and all the transitive dependencies (eg. risk) that go with all of them.


## usage

Import the library:

	import "github.com/cdelorme/go-multiconf"

Define a configuration struct, _optionally with a `GoString()` function for safe printing_:

	type MyConf struct { Name string }
	func (self MyConf) GoString() string { return "custom format" }

Create an instance of multiconf, and set your configuration object:

	c := &MyConf{Name: "Default"}
	config := multiconf.Config{Description: "I want help automated", Configuration: c}

Supplying your configuration struct to the config instance will automatically populate and merge all the settings from cli options, environment variables, and configuration file data.

_Setting a `Description` enables the builtin `help` support which captures `help`, `-h`, and `--help` in cli flags and auto-prints to `stdout` before terminating._  _To supplement the help system, you can add examples of your utility:_

	config.Example("-o /path/to/output")

If in validating your configuration (_such as during post-processing_), you can call `Help()` to print usage information without automatically closing the application (whether it closes when calling the public method is up to your code).

Register cli options and environment variables like this:

	config.Env("name", "optional description of name", "MYAPP_CONFIG_NAME")
	config.Option("name", "optional description of name", "-n", "--name")

You can even add keys with depth via period delimited values:

	config.Env("try.depth", "demo depth", "MYAPP_CONFIG_DEPTH")
	config.Option("try.depth", "demo depth", "-d", "--depth")

Finally, you can load all configuration in a single command (_optionally supplying alternative file paths_):

	config.Load()

_Your configuration object `c` from earlier will now be populated and you can begin using it._


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


## future

In the future I would like to make the following changes:

- add support for default values and force merge with `parseEnv` & `parseOption`
- enhance or simplify options, `getopt` parsing, and the associated help system
- add support for `sighup` based reloads


I have decided not to support the following:

- type-specific methods for registration of environment variables and cli options
- introducing fswatch to automatically reload when configuration file changes

_I firmly believe types should be inferred by the configuration struct supplied, and that may involve some `reflect` logic in the future to support attempts to safely correct inputs from cli options and environment variables._

_Automatic reloads requires a transient dependency for cross-platform support, and introduces a new level of complexity regarding tracking of multiple folders.  I may expose a `reload()` function in the future, but I don't intend to add fswatch behavior._


# references

- [go-option](https://github.com/cdelorme/go-option)
- [go-env](https://github.com/cdelorme/go-env)
- [go-config](https://github.com/cdelorme/go-config)
- [go-maps](https://github.com/cdelorme/go-maps)
- [go sighup](https://gist.github.com/andelf/5889946)
- [go method sets](https://golang.org/ref/spec#Method_sets)
- [viper project](https://github.com/spf13/viper)
- [getopt](https://en.wikipedia.org/wiki/Getopt)
