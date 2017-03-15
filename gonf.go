/*
Package gonf provides the Gonf utility to combine configuration methods including cli flags, environment variables, and json configuration files at common paths.

It automatically determines the application name, and uses that when searching OS specific paths for configuration files, as well as when printing help output.  It supports sighup config file reloads on non-windows platforms.  It is concurrently safe.
*/
package gonf

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/signal"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
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
	print              = fmt.Fprintf
	stdout   io.Writer = os.Stdout
	exit               = os.Exit
	readfile           = ioutil.ReadFile
	mkdirall           = os.MkdirAll
	create             = os.Create
	stat               = os.Stat
	goos               = runtime.GOOS
	appPath            = os.Args[0]
	appName            = strings.TrimSuffix(filepath.Base(appPath), filepath.Ext(appPath))
	paths    []string
)

func load() {
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

func init() {
	load()
}

type setting struct {
	Name        string
	Description string
	Env         string
	Options     []string
}

func (s *setting) String() string {
	var o string
	if len(s.Options) > 0 {
		o = strings.Replace(strings.Join(s.Options, ", "), ":", "", -1)
	}
	if s.Env != "" {
		o += " (" + s.Env + ")"
	}
	return fmt.Sprintf("%-30s\t%s\n", o, s.Description)
}

func (s *setting) Match(in string) (bool, bool) {
	for _, o := range s.Options {
		if o == in {
			return true, false
		} else if o == in+":" {
			return true, true
		}
	}
	return false, false
}

// Gonf exposes a simple interface and handles all configuration responsibilities
type Gonf struct {
	sync.RWMutex
	Configuration  interface{} // the object to merge and cast configuration onto
	Description    string      // populate this if you want automatic help flags
	configFile     string
	configModified time.Time
	examples       []string
	settings       []setting
	sighup         chan os.Signal
}

func (g *Gonf) merge(maps ...map[string]interface{}) map[string]interface{} {
	m := make(map[string]interface{})
	for _, t := range maps {
		for k, v := range t {
			if _, me := m[k]; me {
				if m1, ok := m[k].(map[string]interface{}); ok {
					if m2, is := v.(map[string]interface{}); is {
						v = g.merge(m1, m2)
					}
				}
			}
			m[k] = v
		}
	}
	return m
}

func (g *Gonf) isNumeric(t reflect.Kind) bool {
	switch t {
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Float32, reflect.Float64:
		return true
	}
	return false
}

func (g *Gonf) cast(o interface{}, m map[string]interface{}, discard map[string]interface{}) {
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
				case in == reflect.String && g.isNumeric(tr):
					m[k], _ = strconv.ParseFloat(v.(string), 64)
				case in == reflect.Map && tr == reflect.Struct:
					if p, ok := v.(map[string]interface{}); ok {
						g.cast(d.Field(i).Addr().Interface(), p, map[string]interface{}{})
					}
				}
			}
		}
	}
	for i := 0; i < d.NumField(); i++ {
		if t := strings.Split(d.Type().Field(i).Tag.Get("json"), ",")[0]; t != "" || d.Field(i).Kind() != reflect.Struct {
			continue
		}
		g.cast(reflect.New(d.Field(i).Type()).Interface(), m, discard)
	}
}

func (g *Gonf) to(data ...map[string]interface{}) {
	g.Lock()
	defer g.Unlock()
	combo := g.merge(data...)
	if g.Configuration != nil {
		if c, e := g.Configuration.(locker); e {
			c.Lock()
			defer c.Unlock()
		}
		g.cast(g.Configuration, combo, map[string]interface{}{})
		final, _ := json.Marshal(combo)
		json.Unmarshal(final, g.Configuration)
		if c, e := g.Configuration.(logger); e {
			c.Info("Configuration: %#v\n", g.Configuration)
		}
	}
}

func (g *Gonf) set(cursor map[string]interface{}, key string, value interface{}) {
	keys := strings.Split(key, ".")
	for i, k := range keys {
		if i+1 == len(keys) {
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

func (g *Gonf) parseEnvs() map[string]interface{} {
	vars := make(map[string]interface{})
	for _, s := range g.settings {
		if s.Env == "" {
			continue
		}
		if v := os.Getenv(s.Env); len(v) > 0 {
			g.set(vars, s.Name, v)
		}
	}
	return vars
}

func (g *Gonf) help(discontinue bool) {
	g.RLock()
	defer g.RUnlock()
	print(stdout, "[%s]\nDescription:\n\t%s\n", appName, g.Description)
	print(stdout, "\n\nFlags:\n\n")
	print(stdout, "\t%s\n\t\t%s\n\n", "help, -h, --help", "display help information")
	for _, o := range g.settings {
		print(stdout, "\t%s\n\t\t%s\n\n", strings.Join(o.Options, ", "), o.Description)
	}
	if len(g.examples) > 0 {
		print(stdout, "\n\nUsage:\n\n")
	}
	for _, e := range g.examples {
		print(stdout, "\t%s %s\n", appName, e)
	}
	if discontinue {
		exit(0)
	}
}

func (g *Gonf) parseLong(i *int, m map[string]interface{}) {
	var y, greedy bool
	argv := strings.SplitN(os.Args[*i], "=", 2)
	for _, s := range g.settings {
		if y, greedy = s.Match(argv[0]); !y {
			continue
		}
		switch {
		case len(argv) == 1 && *i+1 < len(os.Args) && os.Args[*i+1] != "--" && (!strings.HasPrefix(os.Args[*i+1], "-") || greedy):
			*i++
			g.set(m, s.Name, os.Args[*i])
		case len(argv) == 2 && argv[1] != "":
			g.set(m, s.Name, argv[1])
		default:
			g.set(m, s.Name, true)
		}
	}
}

func (g *Gonf) parseShort(i *int, m map[string]interface{}) {
	var y, greedy bool
	a := strings.TrimPrefix(os.Args[*i], "-")
	for ci, c := range a {
		for _, s := range g.settings {
			if y, greedy = s.Match("-" + string(c)); !y {
				continue
			}
			switch {
			case ci+1 >= len(a) && *i+1 < len(os.Args) && os.Args[*i+1] != "--" && (!strings.HasPrefix(os.Args[*i+1], "-") || greedy):
				*i++
				g.set(m, s.Name, os.Args[*i])
			case ci+1 < len(a) && greedy:
				g.set(m, s.Name, a[ci+1:])
				return
			default:
				g.set(m, s.Name, true)
			}
		}
	}
}

func (g *Gonf) parseOptions() map[string]interface{} {
	vars := map[string]interface{}{}
	for i := 0; i < len(os.Args); i++ {
		if arg := os.Args[i]; arg == "--" {
			break
		} else if len(g.Description) > 0 && (arg == "help" || arg == "-h" || arg == "--help") {
			g.help(true)
			return nil
		} else if len(arg) == 1 || !strings.HasPrefix(arg, "-") {
			continue
		}

		if arg := os.Args[i]; strings.HasPrefix(arg, "--") {
			g.parseLong(&i, vars)
		} else {
			g.parseShort(&i, vars)
		}
	}
	return vars
}

func (g *Gonf) comment(data []byte) []byte {
	re := regexp.MustCompile(`(?:/\*[^*]*\*+(?:[^/*][^*]*\*+)*/|//[^\n]*(?:\n|$)|#[^\n]*(?:\n|$))|("[^"\\]*(?:\\[\S\s][^"\\]*)*"|'[^'\\]*(?:\\[\S\s][^'\\]*)*'|[\S\s][^/"'\\]*)`)
	return re.ReplaceAll(data, []byte("$1"))
}

func (g *Gonf) readfile() (map[string]interface{}, error) {
	vars := make(map[string]interface{})
	g.Lock()
	defer g.Unlock()

	modTime := g.configModified
	if fi, err := stat(g.configFile); err == nil {
		if modTime = fi.ModTime(); modTime == g.configModified {
			return vars, nil
		}
	}

	data, err := readfile(g.configFile)
	if err != nil {
		return vars, err
	}

	if err := json.Unmarshal(g.comment(data), &vars); err != nil {
		if c, ok := g.Configuration.(logger); ok {
			c.Debug("failed to parse %s (%s)", g.configFile, err)
		}
		return vars, err
	}

	g.configModified = modTime
	return vars, nil
}

func (g *Gonf) parseFiles(filenames ...string) map[string]interface{} {
	vars := make(map[string]interface{})

	if g.ConfigFile() != "" {
		vars, _ := g.readfile()
		return vars
	}

	for _, p := range paths {
		for _, f := range filenames {
			g.Lock()
			g.configFile = filepath.Join(p, f)
			g.Unlock()
			if vars, err := g.readfile(); err == nil {
				return vars
			}
		}
	}

	g.Lock()
	g.configFile = filepath.Join(paths[len(paths)-1], filenames[0])
	g.Unlock()
	g.Save()
	return vars
}

func (g *Gonf) signal() {
	for _ = range g.sighup {
		g.Reload()
	}
}

// reload the configuration file ontop of the existing configuration
func (g *Gonf) Reload() {
	if v, err := g.readfile(); err == nil && len(v) > 0 {
		g.to(v)
		if c, e := g.Configuration.(callbacker); e {
			c.Callback()
		}
	}
}

// save the current configuration state back into the loaded file
func (g *Gonf) Save() {
	g.RLock()
	defer g.RUnlock()
	if g.configFile == "" {
		return
	}

	mkdirall(filepath.Dir(g.configFile), 0775)
	f, err := create(g.configFile)
	if err != nil {
		if c, ok := g.Configuration.(logger); ok {
			c.Debug("failed to save %s (%s)", g.configFile, err)
		}
		return
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "\t")
	enc.Encode(g.Configuration)
}

// load configuration, with optional filenames to look for before using OS-specific standard paths
func (g *Gonf) Load(filenames ...string) {
	for i := len(filenames) - 1; i >= 0; i-- {
		if filenames[i] == "" {
			filenames = append(filenames[:i], filenames[i+1:]...)
		}
	}

	g.to(g.parseFiles(append(filenames, appName+".json")...), g.parseEnvs(), g.parseOptions())
	if c, e := g.Configuration.(callbacker); e {
		c.Callback()
	}

	g.Lock()
	defer g.Unlock()
	if goos != "windows" && g.sighup == nil {
		g.sighup = make(chan os.Signal)
		go g.signal()
		signal.Notify(g.sighup, syscall.SIGHUP)
	}
}

// add a new configuration option
func (g *Gonf) Add(name, description, env string, options ...string) {
	if name == "" || (env == "" && len(options) == 0) {
		return
	}
	g.Lock()
	g.settings = append(g.settings, setting{
		Name:        name,
		Description: description,
		Env:         env,
		Options:     options,
	})
	g.Unlock()
}

// provide examples of cli flags (minus the tool name)
func (g *Gonf) Example(example string) {
	g.Lock()
	g.examples = append(g.examples, example)
	g.Unlock()
}

// print automated help manually without exiting
func (g *Gonf) Help() {
	g.help(false)
}

// returns the file Gonf is using
func (g *Gonf) ConfigFile() string {
	g.RLock()
	defer g.RUnlock()
	return g.configFile
}
