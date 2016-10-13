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

type configuration interface {
	GoString() string
}

type logger interface {
	Info(string, ...interface{})
	Debug(string, ...interface{})
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

type option struct {
	Key         string
	Description string
	Flags       []string
}

type parse struct {
	Greedy bool
	Flag   string
	Option *option
}

type env struct {
	Key         string
	Name        string
	Description string
}

type Gonf struct {
	Logger        logger
	Configuration configuration
	Description   string
	paths         []string
	examples      []string
	file          string
	options       []option
	long          []parse
	short         []parse
	envs          []env
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

func (self *Gonf) cast(m map[string]interface{}) {
	d := reflect.ValueOf(self.Configuration).Elem()
	for i := 0; i < d.NumField(); i++ {
		for k, v := range m {
			if l := strings.Split(d.Type().Field(i).Tag.Get("json"), ",")[0]; (l == "" && d.Type().Field(i).Name == k) || (l != "-" && k == l) {
				if reflect.TypeOf(v).Kind() == reflect.String {
					switch d.Field(i).Kind() {
					case reflect.Bool:
						m[k], _ = strconv.ParseBool(v.(string))
					case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Float32, reflect.Float64:
						m[k], _ = strconv.ParseFloat(v.(string), 64)
					}
				}
			}
		}
	}
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
		if self.Logger != nil {
			self.Logger.Info("Configuration: %#v\n", self.Configuration)
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
	for _, e := range self.envs {
		if v := os.Getenv(e.Name); len(v) > 0 {
			self.set(vars, e.Key, v)
		}
	}
	return vars
}

func (self *Gonf) help(discontinue bool) {
	print(stdout, "[%s]: %s\n\n", appName, self.Description)
	print(stdout, "\nFlags:\n")
	print(stdout, "%-30s\t%s\n", "help, -h, --help", "display help information")
	for _, option := range self.options {
		print(stdout, "%-30s\t%s\n", strings.Join(option.Flags, ", "), option.Description)
	}
	if len(self.examples) > 0 {
		print(stdout, "\nUsage:\n")
		for _, e := range self.examples {
			print(stdout, "%s %s\n", appName, e)
		}
	}
	if discontinue {
		exit(0)
	}
}

func (self *Gonf) parseOptions() map[string]interface{} {
	vars := make(map[string]interface{})

	var skip bool
	for idx, arg := range os.Args {
		if len(self.Description) > 0 && (arg == "help" || arg == "-h" || arg == "--help") {
			self.help(true)
			return nil
		} else if skip || !strings.HasPrefix(arg, "-") || len(arg) == 1 {
			skip = false
			continue
		} else if arg == "--" {
			break
		}

		if strings.HasPrefix(arg, "--") {
			for _, long := range self.long {
				if strings.HasPrefix(arg, "--"+long.Flag) {
					if s := strings.Split(arg, "="); len(s) == 2 {
						if len(s[1]) > 0 {
							self.set(vars, long.Option.Key, s[1])
						} else {
							self.set(vars, long.Option.Key, true)
						}
					} else if idx+1 < len(os.Args) {
						if os.Args[idx+1] != "--" && (!strings.HasPrefix(os.Args[idx+1], "-") || long.Greedy) {
							skip = true
							self.set(vars, long.Option.Key, os.Args[idx+1])
						} else {
							self.set(vars, long.Option.Key, true)
						}
					} else {
						self.set(vars, long.Option.Key, true)
					}
				}
			}
		} else {
			s := strings.TrimPrefix(arg, "-")
			var cskip bool
			for idc, c := range s {
				for _, short := range self.short {
					if string(c) == short.Flag {
						if idc == (len(s) - 1) {
							if idx+1 < len(os.Args) && os.Args[idx+1] != "--" && (!strings.HasPrefix(os.Args[idx+1], "-") || short.Greedy) {
								self.set(vars, short.Option.Key, os.Args[idx+1])
								skip = true
								break
							} else {
								vars[short.Option.Key] = true
							}
						} else {
							self.set(vars, short.Option.Key, string(s[idc+1:]))
							if !short.Greedy {
								for _, si := range self.short {
									if string(s[idc+1]) == si.Flag {
										self.set(vars, short.Option.Key, true)
										break
									}
								}
							} else {
								cskip = true
								break
							}
						}
					}
					if cskip {
						cskip = false
						break
					}
				}
			}
		}
	}

	return vars
}

func (self *Gonf) loadConfig() map[string]interface{} {
	vars := make(map[string]interface{})

	for _, f := range self.paths {
		data, err := readfile(f)
		if err != nil {
			continue
		}
		if e := json.Unmarshal(data, &vars); e == nil {
			return vars
		} else {
			if self.Logger != nil {
				self.Logger.Debug("failed to parse %s (%s)", f, e)
			}
		}
	}

	return vars
}

func (self *Gonf) Load(p ...string) {
	self.paths = append(paths, p...)

	maps := []map[string]interface{}{}

	maps = append(maps, self.parseOptions())
	maps = append(maps, self.parseEnvs())
	maps = append(maps, self.loadConfig())

	self.to(maps...)
}

func (self *Gonf) Env(key, description, name string) {
	if len(key) == 0 || len(name) == 0 {
		return
	}

	self.envs = append(self.envs, env{Key: key, Description: description, Name: name})
}

func (self *Gonf) Option(key, description string, flags ...string) {
	if len(key) == 0 {
		return
	}

	o := option{Key: key, Description: description}

	for _, flag := range flags {
		p := parse{
			Greedy: strings.HasSuffix(flag, ":"),
			Flag:   strings.TrimSuffix(strings.TrimPrefix(strings.TrimPrefix(flag, "-"), "-"), ":"),
			Option: &o,
		}
		if len(p.Flag) == 1 {
			self.short = append(self.short, p)
			o.Flags = append(o.Flags, "-"+p.Flag)
		} else if len(p.Flag) > 1 {
			self.long = append(self.long, p)
			o.Flags = append(o.Flags, "--"+p.Flag)
		}
	}

	if len(o.Flags) == 0 {
		return
	}

	self.options = append(self.options, o)
}

func (self *Gonf) Example(example string) {
	self.examples = append(self.examples, example)
}

func (self *Gonf) Help() {
	self.help(false)
}
