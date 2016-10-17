package gonf

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
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

var print func(io.Writer, string, ...interface{}) (int, error) = fmt.Fprintf
var stdout io.Writer = os.Stdout
var exit = os.Exit
var readfile = ioutil.ReadFile
var exts = []string{"", ".json", ".conf"}
var paths []string
var appName string

func load() {
	paths = []string{}
	appName = filepath.Base(os.Args[0])

	if p, e := filepath.EvalSymlinks(os.Args[0]); e == nil {
		if a, e := filepath.Abs(p); e == nil {
			for _, e := range exts[1:] {
				paths = append(paths, filepath.Join(filepath.Dir(a), appName+e))
			}
		}
	}

	if p := os.Getenv("APPDATA"); p != "" {
		for _, e := range exts {
			paths = append(paths, filepath.Join(p, appName, appName+e))
		}
	}

	if xdg := os.Getenv("XDG_CONFIG_DIR"); xdg != "" {
		for _, p := range strings.Split(xdg, ":") {
			for _, e := range exts {
				paths = append(paths, filepath.Join(p, appName, appName+e))
			}
		}
	} else {
		home := os.Getenv("HOME")
		if home == "" {
			if u, err := user.Current(); err == nil {
				home = u.HomeDir
			}
		}
		for _, e := range exts {
			paths = append(paths, filepath.Join(home, ".config", appName, appName+e))
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

func (self *setting) String() string {
	var o string
	if len(self.Options) > 0 {
		o = strings.Replace(strings.Join(self.Options, ", "), ":", "", -1)
	}
	if self.Env != "" {
		o += " (" + self.Env + ")"
	}
	return fmt.Sprintf("%-30s\t%s\n", o, self.Description)
}

func (self *setting) Match(in string) (bool, bool) {
	for _, o := range self.Options {
		if o == in {
			return true, false
		} else if o == in+":" {
			return true, true
		}
	}
	return false, false
}

type Gonf struct {
	Configuration interface{}
	Description   string
	paths         []string
	examples      []string
	file          string
	settings      []setting
}

func (self *Gonf) merge(maps ...map[string]interface{}) map[string]interface{} {
	m := make(map[string]interface{})
	for _, t := range maps {
		for k, v := range t {
			if _, me := m[k]; me {
				if m1, ok := m[k].(map[string]interface{}); ok {
					if m2, is := v.(map[string]interface{}); is {
						v = self.merge(m1, m2)
					}
				}
			}
			m[k] = v
		}
	}
	return m
}

func (self *Gonf) isNumeric(t reflect.Kind) bool {
	switch t {
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Float32, reflect.Float64:
		return true
	}
	return false
}

func (self *Gonf) reCast(o interface{}, m map[string]interface{}) {
	d := reflect.ValueOf(o).Elem()
	var skip bool
	for i := 0; i < d.NumField(); i++ {
		t := strings.Split(d.Type().Field(i).Tag.Get("json"), ",")[0]
		if t == "-" {
			continue
		}
		n := d.Type().Field(i).Name
		tr := d.Field(i).Kind()
		for k, v := range m {
			if t == k || (t == "" && n == k) {
				skip = true
				in := reflect.TypeOf(v).Kind()
				if in == reflect.String {
					if tr == reflect.Bool {
						m[k], _ = strconv.ParseBool(v.(string))
					} else if self.isNumeric(tr) {
						m[k], _ = strconv.ParseFloat(v.(string), 64)
					}
				} else if in == reflect.Map && tr == reflect.Struct {
					if p, ok := v.(map[string]interface{}); ok {
						self.reCast(d.Field(i).Addr().Interface(), p)
					}
				}
			}
		}
		if !skip && t == "" && tr == reflect.Struct {
			self.reCast(reflect.New(d.Field(i).Type()).Interface(), m)
		}
		skip = false
	}
}

func (self *Gonf) cast(m map[string]interface{}) {
	self.reCast(self.Configuration, m)
}

func (self *Gonf) to(data ...map[string]interface{}) {
	combo := self.merge(data...)
	if self.Configuration != nil {
		if c, e := self.Configuration.(locker); e {
			c.Lock()
			defer c.Unlock()
		}
		self.cast(combo)
		final, _ := json.Marshal(combo)
		json.Unmarshal(final, self.Configuration)
		if c, e := self.Configuration.(logger); e {
			c.Info("Configuration: %#v\n", self.Configuration)
		}
	}
}

func (self *Gonf) set(cursor map[string]interface{}, key string, value interface{}) {
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

func (self *Gonf) parseEnvs() map[string]interface{} {
	vars := make(map[string]interface{})
	for _, s := range self.settings {
		if s.Env == "" {
			continue
		}
		if v := os.Getenv(s.Env); len(v) > 0 {
			self.set(vars, s.Name, v)
		}
	}
	return vars
}

func (self *Gonf) help(discontinue bool) {
	print(stdout, "[%s]: %s\n\n", appName, self.Description)
	print(stdout, "\nFlags:\n")
	print(stdout, "%-30s\t%s\n", "help, -h, --help", "display help information")
	for _, o := range self.settings {
		print(stdout, "%s", o)
	}
	if len(self.examples) > 0 {
		print(stdout, "\nUsage:\n")
	}
	for _, e := range self.examples {
		print(stdout, "%s %s\n", appName, e)
	}
	if discontinue {
		exit(0)
	}
}

func (self *Gonf) parseLong(i *int, m map[string]interface{}) {
	var y, g bool
	argv := strings.SplitN(os.Args[*i], "=", 2)
	for _, s := range self.settings {
		if y, g = s.Match(argv[0]); !y {
			continue
		}
		switch {
		case len(argv) == 1 && *i+1 < len(os.Args) && os.Args[*i+1] != "--" && (!strings.HasPrefix(os.Args[*i+1], "-") || g):
			*i++
			self.set(m, s.Name, os.Args[*i])
		case len(argv) == 2 && argv[1] != "":
			self.set(m, s.Name, argv[1])
		default:
			self.set(m, s.Name, true)
		}
	}
}

func (self *Gonf) parseShort(i *int, m map[string]interface{}) {
	var y, g bool
	a := strings.TrimPrefix(os.Args[*i], "-")
	for ci, c := range a {
		for _, s := range self.settings {
			if y, g = s.Match("-" + string(c)); !y {
				continue
			}
			switch {
			case ci+1 >= len(a) && *i+1 < len(os.Args) && os.Args[*i+1] != "--" && (!strings.HasPrefix(os.Args[*i+1], "-") || g):
				*i++
				self.set(m, s.Name, os.Args[*i])
			case ci+1 < len(a) && g:
				self.set(m, s.Name, a[ci+1:])
				return
			default:
				self.set(m, s.Name, true)
			}
		}
	}
}

func (self *Gonf) parseOptions() map[string]interface{} {
	vars := map[string]interface{}{}
	for i := 0; i < len(os.Args); i++ {
		if arg := os.Args[i]; arg == "--" {
			break
		} else if len(self.Description) > 0 && (arg == "help" || arg == "-h" || arg == "--help") {
			self.help(true)
			return nil
		} else if len(arg) == 1 || !strings.HasPrefix(arg, "-") {
			continue
		}

		if arg := os.Args[i]; strings.HasPrefix(arg, "--") {
			self.parseLong(&i, vars)
		} else {
			self.parseShort(&i, vars)
		}
	}
	return vars
}

func (self *Gonf) parseFiles() map[string]interface{} {
	vars := make(map[string]interface{})

	for _, f := range self.paths {
		data, err := readfile(f)
		if err != nil {
			continue
		}
		if e := json.Unmarshal(data, &vars); e == nil {
			return vars
		} else if c, ok := self.Configuration.(logger); ok {
			c.Debug("failed to parse %s (%s)", f, e)
		}
	}

	return vars
}

func (self *Gonf) Load(p ...string) {
	self.paths = append(paths, p...)

	maps := []map[string]interface{}{}

	maps = append(maps, self.parseOptions())
	maps = append(maps, self.parseEnvs())
	maps = append(maps, self.parseFiles())

	self.to(maps...)

	if c, e := self.Configuration.(callbacker); e {
		c.Callback()
	}
}

func (self *Gonf) Add(name, description, env string, options ...string) {
	if name == "" || (env == "" && len(options) == 0) {
		return
	}
	self.settings = append(self.settings, setting{
		Name:        name,
		Description: description,
		Env:         env,
		Options:     options,
	})
}

func (self *Gonf) Example(example string) {
	self.examples = append(self.examples, example)
}

func (self *Gonf) Help() {
	self.help(false)
}
