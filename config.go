package gonf

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	errNilTarget      = errors.New("no configuration target supplied...")
	errEmptyConfig    = errors.New("no configuration file...")
	errNoChanges      = errors.New("the configuration file has not changed...")
	errEmptyName      = errors.New("name cannot be empty...")
	errNoEnvOptions   = errors.New("environment variable must not be empty or at least one command line option is expected...")
	errBadNameSyntax  = errors.New("bad syntax for child properties...")
	errConflictingAdd = errors.New("duplicate option detected...")

	fmtPrintf = fmt.Printf
	readfile  = ioutil.ReadFile
	mkdirall  = os.MkdirAll
	create    = os.Create
	stat      = os.Stat
	exit      = os.Exit
)

type locker interface {
	Lock()
	Unlock()
}

// A simple interface for configuration, which expects a Target pointer to a
// structure which it can apply registered settings against, and a description
// which will enable automatically generated help and register related options.
type Config struct {
	mu             sync.RWMutex
	target         interface{}
	description    string
	configFile     string
	configModified time.Time
	examples       []string
	settings       []setting
}

func (c *Config) isNumeric(t reflect.Kind) bool {
	switch t {
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Float32, reflect.Float64:
		return true
	}
	return false
}

func (c *Config) convert(d reflect.Value, v interface{}) interface{} {
	t := d.Kind()
	in := reflect.TypeOf(v).Kind()
	switch {
	case in == reflect.String && t == reflect.Bool:
		if r, err := strconv.ParseBool(v.(string)); err == nil {
			return r
		}
	case in == reflect.String && c.isNumeric(t):
		if r, err := strconv.ParseFloat(v.(string), 64); err == nil {
			return r
		}
	case in == reflect.Map && t == reflect.Struct:
		if p, ok := v.(map[string]interface{}); ok {
			c.cast(d.Addr().Interface(), p, map[string]interface{}{})
			return p
		}
	}
	return v
}

func (c *Config) cast(o interface{}, m map[string]interface{}, discard map[string]interface{}) {
	d := reflect.ValueOf(o).Elem()
	for k, v := range m {
		if _, ok := discard[k]; ok {
			continue
		}
		for i := 0; i < d.NumField(); i++ {
			if n := strings.Split(d.Type().Field(i).Tag.Get("json"), ",")[0]; n == "-" || n != k {
				continue
			}
			discard[k] = struct{}{}
			m[k] = c.convert(d.Field(i), v)
		}
		if _, ok := discard[k]; ok {
			continue
		}
		for i := 0; i < d.NumField(); i++ {
			if k != d.Type().Field(i).Name {
				continue
			}
			discard[k] = struct{}{}
			m[k] = c.convert(d.Field(i), v)
		}
	}
	for i := 0; i < d.NumField(); i++ {
		if t := strings.Split(d.Type().Field(i).Tag.Get("json"), ",")[0]; t != "" || d.Field(i).Kind() != reflect.Struct || !d.Type().Field(i).Anonymous || len(m) <= len(discard) {
			continue
		}
		c.cast(reflect.New(d.Field(i).Type()).Interface(), m, discard)
	}
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

func (c *Config) to(data ...map[string]interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.target == nil {
		return errNilTarget
	}
	combo := c.merge(data...)
	if l, e := c.target.(locker); e {
		l.Lock()
		defer l.Unlock()
	}
	c.cast(c.target, combo, map[string]interface{}{})
	final, _ := json.Marshal(combo)
	return json.Unmarshal(final, c.target)
}

func (c *Config) set(cursor map[string]interface{}, key string, value interface{}) {
	keys := strings.Split(key, ".")
	for i, k := range keys {
		if i+1 == len(keys) {
			cursor[k] = value
		} else {
			if _, ok := cursor[k]; !ok {
				cursor[k] = map[string]interface{}{}
			}
			cursor = cursor[k].(map[string]interface{})
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
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.description == "" {
		return
	}
	fmtPrintf("[%s]\nDescription:\n\t%s\n", appName, c.description)
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

func (c *Config) readFile() (map[string]interface{}, error) {
	vars := make(map[string]interface{})
	c.mu.Lock()
	defer c.mu.Unlock()
	modTime := c.configModified
	if fi, err := stat(c.configFile); err == nil {
		if modTime = fi.ModTime(); c.configModified.Equal(modTime) {
			return vars, errNoChanges
		}
	}
	data, err := readfile(c.configFile)
	if err != nil {
		return vars, err
	}
	c.configModified = modTime
	err = json.Unmarshal(c.comment(data), &vars)
	return vars, err
}

func (c *Config) parseFiles(filenames ...string) (map[string]interface{}, error) {
	vars := make(map[string]interface{})
	for _, f := range filenames {
		if filepath.IsAbs(f) {
			c.mu.Lock()
			c.configFile = f
			c.mu.Unlock()
			if vars, err := c.readFile(); err == nil {
				return vars, nil
			}
		} else {
			for _, p := range paths {
				c.mu.Lock()
				c.configFile = filepath.Join(p, f)
				c.mu.Unlock()
				if vars, err := c.readFile(); err == nil {
					return vars, nil
				}
			}
		}
	}
	c.mu.Lock()
	c.configFile = filepath.Join(paths[len(paths)-1], filenames[0])
	c.mu.Unlock()
	return vars, c.Save()
}

// Set the configuration target using this method.
func (c *Config) Target(t interface{}) {
	c.mu.Lock()
	c.target = t
	c.mu.Unlock()
}

// You can register any json tag or public property by name here.  To deal with
// depth you can use dot-notation (eg. parent.child).  The description, while
// optional, will be used to generate help information.
//
// The name must not be empty, and a non-empty environment variable or at least
// one command line option must be supplied. If the parameters are invalid,
// or the name has already been registered, an error will be returned. Finally
// if the name has bad syntax for child properties an error will be returned.
//
// Duplicate environment variables or command line options are accepted.
//
// As defined by some POSIX getopt implementations, a colon suffix (:) is used
// to define explicit notation for greedy parameters, which can help deal with
// single-character command line options when the supplied value matches
// another registered single-character command line option.
func (c *Config) Add(name, description, env string, options ...string) error {
	if name == "" {
		return errEmptyName
	} else if env == "" && len(options) == 0 {
		return errNoEnvOptions
	} else if name == "." || strings.HasPrefix(name, ".") || strings.HasSuffix(name, ".") || strings.Contains(name, "..") {
		return errBadNameSyntax
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, s := range c.settings {
		if s.Name == name {
			return errConflictingAdd
		}
	}
	c.settings = append(c.settings, setting{
		Name:        name,
		Description: description,
		Env:         env,
		Options:     options,
	})
	return nil
}

// To enable automated help, set a non-empty description.
func (c *Config) Description(d string) {
	c.mu.Lock()
	c.description = d
	c.mu.Unlock()
}

// Provides a registration for custom examples of command line use cases,
// automatically prefixed by the application name.
func (c *Config) Example(example string) {
	if example == "" {
		return
	}
	c.mu.Lock()
	c.examples = append(c.examples, example)
	c.mu.Unlock()
}

// This is the primary function of the application, which abstracts the entire
// logical process of merging inputs from command line, environment variables
// and json file data onto the configuration target.
//
// A POSIX compatible getopt command line parser is run first to deal with
// optional help flags and terminate prior to any file system access.
//
// Custom paths may be supplied, both relative to the system paths or absolute
// for full control.  Empty names will be discarded and ignored.  The default
// name used is the application name as a directory then again as a .json file.
// If no file is found, it uses the first name supplied (or the default) plus
// the default userspace path (unless the file name is absolute) to save the
// defaults on the configuration target.
//
// Data loaded from a file is applied directly and follows the same rules as
// json unmarshal.  This means tags first, then property names, finally any
// non-ambiguous properties matching anonynous composite structures.  File
// configuration is generally useful when you have complex data structures
// which cannot easily be represented using strings.
//
// While json does not provide support for comments, if // or /**/ comments
// are found they will be safely filtered from the file (unless inside quotes).
//
// Both command line options and environment variables are converted to the
// configuration targets expected types using reflection prior to being run
// through json unmarshal.
//
// If any steps fail, the errors will be collected and aggregated for the
// response, however the system will still make a complete attempt to load
// which means the errors may be treated as non-critical.
//
// The operation is concurrently safe, and performs a lock prior to running
// any steps that touch its own properties.  If the target supports mutex
// locking it will lock while applying configuration.
//
// Finally, it returns with an aggregate of any errors that were encountered
// giving the developer the option of printing them or terminating.
func (c *Config) Load(filenames ...string) error {
	opts := c.parseOptions()
	for i := len(filenames) - 1; i >= 0; i-- {
		if filenames[i] == "" {
			filenames = append(filenames[:i], filenames[i+1:]...)
		}
	}
	files, err := c.parseFiles(append(filenames, filepath.Join(appName, appName+".json"))...)
	if e := c.to(files, c.parseEnvs(), opts); e != nil {
		if err != nil {
			return fmt.Errorf("%s\n%s", err.Error(), e.Error())
		}
		return e
	}
	return err
}

// Used to manually reload changes from the configuration file, if the file has
// been modified since the last attempt to load it.
func (c *Config) Reload() error {
	if c.ConfigFile() == "" {
		return errEmptyConfig
	}
	v, err := c.readFile()
	if err == nil && len(v) > 0 {
		return c.to(v)
	}
	return err
}

// For cases where you want to persist changes to the configuration target,
// this function will save an intended readable json file to the ConfigFile
// identified during Load, or it will return an error if any step fails.
func (c *Config) Save() error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.configFile == "" {
		return errEmptyConfig
	}
	mkdirall(filepath.Dir(c.configFile), 0775)
	f, err := create(c.configFile)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(f)
	enc.SetIndent("", "\t")
	if err := enc.Encode(c.target); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// If the instance has a non-empty Description the help will be printed,
// however the application will not be terminated.
func (c *Config) Help() {
	c.help(false)
}

// After Load this will return the full path to the preferred file.
func (c *Config) ConfigFile() string {
	c.mu.RLock()
	cf := c.configFile
	c.mu.RUnlock()
	return cf
}
