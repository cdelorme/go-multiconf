package gonf

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

type locker interface {
	Lock()
	Unlock()
}

type logger interface {
	Info(string, ...interface{})
	Debug(string, ...interface{})
}

type callbacker interface {
	Callback()
}

var (
	fmtPrintf = fmt.Printf
	readfile  = ioutil.ReadFile
	mkdirall  = os.MkdirAll
	create    = os.Create
	stat      = os.Stat
	exit      = os.Exit
)

// A simple interface for configuration, which expects a Target structure to
// apply registered settings against, and a description to enable automatically
// generated help flags and output.
type Config struct {
	sync.RWMutex
	Target         interface{}
	Description    string
	configFile     string
	configModified time.Time
	examples       []string
	settings       []setting
	sighup         chan os.Signal
}

func (c *Config) merge(maps ...map[string]interface{}) map[string]interface{} {
	m := make(map[string]interface{})
	for _, t := range maps {
		for k, v := range t {
			if _, me := m[k]; me {
				if m1, ok := m[k].(map[string]interface{}); ok {
					if m2, is := v.(map[string]interface{}); is {
						v = c.merge(m1, m2)
					}
				}
			}
			m[k] = v
		}
	}
	return m
}

func (c *Config) isNumeric(t reflect.Kind) bool {
	switch t {
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Float32, reflect.Float64:
		return true
	}
	return false
}

func (c *Config) cast(o interface{}, m map[string]interface{}, discard map[string]interface{}) {
	d := reflect.ValueOf(o).Elem()
	for i := 0; i < d.NumField(); i++ {
		t := strings.Split(d.Type().Field(i).Tag.Get("json"), ",")[0]
		if t == "-" {
			continue
		}
		n := d.Type().Field(i).Name
		tr := d.Field(i).Kind()
		for k, v := range m {
			if _, ok := discard[k]; !ok && (t == k || (t == "" && n == k)) {
				discard[k] = struct{}{}
				in := reflect.TypeOf(v).Kind()
				switch {
				case in == reflect.String && tr == reflect.Bool:
					m[k], _ = strconv.ParseBool(v.(string))
				case in == reflect.String && c.isNumeric(tr):
					m[k], _ = strconv.ParseFloat(v.(string), 64)
				case in == reflect.Map && tr == reflect.Struct:
					if p, ok := v.(map[string]interface{}); ok {
						c.cast(d.Field(i).Addr().Interface(), p, map[string]interface{}{})
					}
				}
			}
		}
	}
	for i := 0; i < d.NumField(); i++ {
		if t := strings.Split(d.Type().Field(i).Tag.Get("json"), ",")[0]; t != "" || d.Field(i).Kind() != reflect.Struct {
			continue
		}
		c.cast(reflect.New(d.Field(i).Type()).Interface(), m, discard)
	}
}

func (c *Config) to(data ...map[string]interface{}) {
	c.Lock()
	defer c.Unlock()
	combo := c.merge(data...)
	if c.Target != nil {
		if c, e := c.Target.(locker); e {
			c.Lock()
			defer c.Unlock()
		}
		c.cast(c.Target, combo, map[string]interface{}{})
		final, _ := json.Marshal(combo)
		json.Unmarshal(final, c.Target)
		if l, e := c.Target.(logger); e {
			l.Info("Configuration: %#v\n", c.Target)
		}
	}
}

func (c *Config) set(cursor map[string]interface{}, key string, value interface{}) {
	keys := strings.Split(key, ".")
	for i, k := range keys {
		if i+1 == len(keys) || keys[i+1] == "" {
			cursor[k] = value
		} else {
			if _, ok := cursor[k]; !ok {
				cursor[k] = map[string]interface{}{}
			}
			if v, ok := cursor[k].(map[string]interface{}); !ok {
				t := map[string]interface{}{}
				cursor[k] = t
				cursor = t
			} else {
				cursor = v
			}
		}
	}
}

func (c *Config) parseEnvs() map[string]interface{} {
	vars := make(map[string]interface{})
	for _, s := range c.settings {
		if s.Env == "" {
			continue
		}
		if v := os.Getenv(s.Env); len(v) > 0 {
			c.set(vars, s.Name, v)
		}
	}
	return vars
}

func (c *Config) help(discontinue bool) {
	c.RLock()
	defer c.RUnlock()
	if c.Description == "" {
		return
	}
	fmtPrintf("[%s]\nDescription:\n\t%s\n", appName, c.Description)
	fmtPrintf("\n\nFlags:\n")
	fmtPrintf("\t%s\n\t\t%s\n\n", "help, -h, --help", "display help information")
	for _, o := range c.settings {
		fmtPrintf("%s\n\n", o)
	}
	if len(c.examples) > 0 {
		fmtPrintf("\nUsage:\n\n")
	}
	for _, e := range c.examples {
		fmtPrintf("\t%s %s\n", appName, e)
	}
	fmtPrintf("\n")
	if discontinue {
		exit(0)
	}
}

func (c *Config) parseLong(i *int, m map[string]interface{}) {
	var y, greedy bool
	argv := strings.SplitN(os.Args[*i], "=", 2)
	for _, s := range c.settings {
		if y, greedy = s.Match(argv[0]); !y {
			continue
		}
		switch {
		case len(argv) == 1 && *i+1 < len(os.Args) && os.Args[*i+1] != "--" && (!strings.HasPrefix(os.Args[*i+1], "-") || greedy):
			*i++
			c.set(m, s.Name, os.Args[*i])
		case len(argv) == 2 && argv[1] != "":
			c.set(m, s.Name, argv[1])
		default:
			c.set(m, s.Name, true)
		}
	}
}

func (c *Config) parseShort(i *int, m map[string]interface{}) {
	var y, greedy bool
	a := strings.TrimPrefix(os.Args[*i], "-")
	for ci, cl := range a {
		for _, s := range c.settings {
			if y, greedy = s.Match("-" + string(cl)); !y {
				continue
			}
			switch {
			case ci+1 >= len(a) && *i+1 < len(os.Args) && os.Args[*i+1] != "--" && (!strings.HasPrefix(os.Args[*i+1], "-") || greedy):
				*i++
				c.set(m, s.Name, os.Args[*i])
			case ci+1 < len(a) && greedy:
				c.set(m, s.Name, a[ci+1:])
				return
			default:
				c.set(m, s.Name, true)
			}
		}
	}
}

func (c *Config) parseOptions() map[string]interface{} {
	vars := map[string]interface{}{}
	for i := 0; i < len(os.Args); i++ {
		if arg := os.Args[i]; arg == "--" {
			break
		} else if arg == "help" || arg == "-h" || arg == "--help" {
			c.help(true)
		} else if len(arg) == 1 || !strings.HasPrefix(arg, "-") {
			continue
		}

		if arg := os.Args[i]; strings.HasPrefix(arg, "--") {
			c.parseLong(&i, vars)
		} else {
			c.parseShort(&i, vars)
		}
	}
	return vars
}

func (c *Config) comment(data []byte) []byte {
	re := regexp.MustCompile(`(?:/\*[^*]*\*+(?:[^/*][^*]*\*+)*/|//[^\n]*(?:\n|$)|#[^\n]*(?:\n|$))|("[^"\\]*(?:\\[\S\s][^"\\]*)*"|'[^'\\]*(?:\\[\S\s][^'\\]*)*'|[\S\s][^/"'\\]*)`)
	return re.ReplaceAll(data, []byte("$1"))
}

func (c *Config) readfile() (map[string]interface{}, error) {
	vars := make(map[string]interface{})

	c.Lock()
	defer c.Unlock()
	modTime := c.configModified
	if fi, err := stat(c.configFile); err == nil {
		if modTime = fi.ModTime(); c.configModified.Equal(modTime) {
			return vars, nil
		}
	}

	data, err := readfile(c.configFile)
	if err != nil {
		if l, ok := c.Target.(logger); ok {
			l.Debug("failed to read %s (%s)", c.configFile, err)
		}
		return vars, err
	}

	if err := json.Unmarshal(c.comment(data), &vars); err != nil {
		if l, ok := c.Target.(logger); ok {
			l.Debug("failed to parse %s (%s)", c.configFile, err)
		}
		return vars, err
	}

	c.configModified = modTime
	return vars, nil
}

func (c *Config) parseFiles(filenames ...string) map[string]interface{} {
	vars := make(map[string]interface{})

	if c.ConfigFile() != "" {
		vars, _ := c.readfile()
		return vars
	}

	for _, p := range paths {
		for _, f := range filenames {
			c.Lock()
			c.configFile = filepath.Join(p, f)
			c.Unlock()
			if vars, err := c.readfile(); err == nil {
				return vars
			}
		}
	}

	c.Lock()
	c.configFile = filepath.Join(paths[len(paths)-1], filenames[0])
	c.Unlock()
	c.Save()
	return vars
}

func (c *Config) signal() {
	for _ = range c.sighup {
		c.Reload()
	}
}

// Used to manually reload changes from the configuration file, and called when
// a sighup is received.  It will check the modified time for changes first,
// and if the Target has a Callback function it will be executed for
// post-processing.
func (c *Config) Reload() {
	if c.ConfigFile() == "" {
		return
	} else if v, err := c.readfile(); err == nil && len(v) > 0 {
		c.to(v)
		if cb, e := c.Target.(callbacker); e {
			cb.Callback()
		}
	}
}

// For cases where you want to persist changes to the configuration target,
// this function will save to the ConfigFile identified during Load.
func (c *Config) Save() {
	c.RLock()
	defer c.RUnlock()
	if c.configFile == "" {
		return
	}

	mkdirall(filepath.Dir(c.configFile), 0775)
	f, err := create(c.configFile)
	if err != nil {
		if l, ok := c.Target.(logger); ok {
			l.Debug("failed to save %s (%s)", c.configFile, err)
		}
		return
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "\t")
	enc.Encode(c.Target)
}

// This is the primary function of the application, which abstracts the entire
// logical process of merging inputs from command line, environment variables
// and json file data onto the Target.
//
// A POSIX compatible getopt command line parser is run first to deal with
// optional help flags and terminate prior to any file system access.
//
// Custom file names may be supplied, and will be appended to the package paths
// but empty file may names supplied will be ignored.  The default  file name
// used is the application name as a directory followed by the application
// name again, then .json (eg. app/app.json).  If no file is found, it uses the
// first supplied file name to create one with the default properties on the
// Target.
//
// Data loaded from a file will apply if it matches any json tags or property
// names, even if they were not registered with Add().  File configuration is
// generally useful when you have complex data structures that cannot be easily
// represented with strings via command line options or environment variables.
//
// While json does not provide support for comments, if // or /**/ comments
// are found they will be safely filtered from the file (unless inside quotes).
//
// Both command line options and environment variables are assigned using
// reflection to check and cast to json compatible types.
//
// Once run it will optionally establish a sighup listener on supported
// platforms allowing configuration file reloads to override the state of the
// configuration target.
//
// If the Target provides Info and Debug functions, errors encountered will be
// printed; unmarshalling the final json onto the Target, saving to a file, and
// the resulting target after loading.
//
// The operation is concurrently safe, and performs a lock prior to running
// any steps that touch its own properties.  It also optionally locks the
// configuration target, if available functions exist (Lock() and Unlock()).
//
// Just before returning it will check for a Callback function on the Target
// and trigger it, allowing post-processing or asynchronous handlers to run.
func (c *Config) Load(filenames ...string) {
	opts := c.parseOptions()
	for i := len(filenames) - 1; i >= 0; i-- {
		if filenames[i] == "" {
			filenames = append(filenames[:i], filenames[i+1:]...)
		}
	}
	c.to(c.parseFiles(append(filenames, appName+".json")...), c.parseEnvs(), opts)

	c.Lock()
	if goos != "windows" && c.sighup == nil {
		c.sighup = make(chan os.Signal)
		go c.signal()
		signal.Notify(c.sighup, syscall.SIGHUP)
	}
	if cb, e := c.Target.(callbacker); e {
		cb.Callback()
	}
	c.Unlock()
}

// You can register any json tag or property here, using dot-notation to depict
// depth (eg. parent.child).  A description may be supplied for help output,
// and it expects either a non-empty environment variable or at least one
// command line option to be accepted.
//
// It will not notify you if the same property, command line option, or
// environment variable has already been registered.
//
// As defined by some POSIX getopt implementations, a colon suffix (:) is
// used to define explicit notation for greedy parameters.
func (c *Config) Add(name, description, env string, options ...string) {
	if name == "" || (env == "" && len(options) == 0) {
		return
	}
	c.Lock()
	c.settings = append(c.settings, setting{
		Name:        name,
		Description: description,
		Env:         env,
		Options:     options,
	})
	c.Unlock()
}

// Provides a registration for custom examples of command line use cases,
// automatically prefixed by the application name.
func (c *Config) Example(example string) {
	c.Lock()
	c.examples = append(c.examples, example)
	c.Unlock()
}

// If the instance has a non-empty Description the help will be printed,
// however the application will not be terminated.
func (c *Config) Help() {
	c.help(false)
}

// After Load this will return the full path to the preferred file.
func (c *Config) ConfigFile() string {
	c.RLock()
	defer c.RUnlock()
	return c.configFile
}
