package gonf

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// mockStat (shamelessly "borrowed" from `types_unix.go`)
type mockStat struct {
	name    string
	size    int64
	mode    os.FileMode
	modTime time.Time
	sys     interface{}
	dir     bool
}

func (self *mockStat) Name() string       { return self.name }
func (self *mockStat) Size() int64        { return self.size }
func (self *mockStat) IsDir() bool        { return self.dir }
func (self *mockStat) Mode() os.FileMode  { return self.mode }
func (self *mockStat) ModTime() time.Time { return self.modTime }
func (self *mockStat) Sys() interface{}   { return &self.sys }

type Deeper struct {
	TripleDepth string
}

type Composite struct {
	Deeper
	DepthByOption int
	DepthByEnv    bool
}

type mockConfig struct {
	sync.Mutex

	Composite
	ExplicitComposite Composite

	GetoptSkip                bool
	GetoptGreedy              string
	GetoptShortGreedySpace    string
	GetoptShortGreedyConflict string
	GetoptShort               bool
	GetoptComboShortL         bool
	GetoptComboShortO         bool
	GetoptComboShortN         bool
	GetoptComboShortG         string
	GetoptFirstLong           string
	GetoptSecondLong          string
	GetoptLongBool            bool

	OptionString string
	OptionNumber float32
	OptionBool   bool

	EnvString string
	EnvNumber int
	EnvBool   bool

	EnvByTag    string `json:"envByTag"`
	OptionByTag string `json:"optionByTag"`

	FileUnregisteredTag      string `json:"fileUnregisteredTag"`
	FileUnregisteredProperty string

	EnvOverrideFile   string
	OptionOverrideEnv string
}

var mockError error = errors.New("mock error")
var commentedFileData string = `{
	// this is a comment
	"fileUnregisteredTag": "// this value is safely parsed",
	/*this is
	"FileUnregisteredProperty": "this is ignored"
	also a
	comment*/
	"FileUnregisteredProperty": "/* this value is also safely parsed */"
}`

func TestPlacebo(_ *testing.T) {}

func TestTarget(_ *testing.T) {
	c := &Config{}
	c.Target(nil)
}

func TestAdd(t *testing.T) {
	c := &Config{}
	if c.Add("cli", "registration of option only", "", "--option-only") != nil {
		t.Error("failed: registration of option only...")
	}
	if c.Add("env", "registration of env only", "TEST") != nil {
		t.Error("failed: registration of env only...")
	}
	if c.Add("deep.test", "registration of deep property", "TEST") != nil {
		t.Error("failed: registration of deep property...")
	}
	if c.Add("env", "registration of duplicate name", "TEST") == nil {
		t.Error("failed: registration of duplicate name...")
	}
	if c.Add("", "registration with empty name", "TEST") == nil {
		t.Error("failed: registration with empty name...")
	}
	if c.Add("noOptions", "registration without any env or options", "") == nil {
		t.Error("failed: registration without any env or options...")
	}
	if c.Add(".bad", "registration of with bad name syntax prefix", "TEST") == nil {
		t.Error("failed: registration of with bad name syntax prefix...")
	}
	if c.Add("bad.", "registration of with bad name syntax prefix", "TEST") == nil {
		t.Error("failed: registration of with bad name syntax prefix...")
	}
	if c.Add("bad..name", "registration of with bad name syntax prefix", "TEST") == nil {
		t.Error("failed: registration of with bad name syntax prefix...")
	}
	if c.Add(".", "registration of with bad name syntax prefix", "TEST") == nil {
		t.Error("failed: registration of with bad name syntax prefix...")
	}
}

func TestDescription(_ *testing.T) {
	c := &Config{}
	c.Description("")
}

func TestExample(_ *testing.T) {
	c := &Config{}
	c.Example("") // empty input is discarded
	c.Example("--not-empty")
}

func TestLoad(t *testing.T) {
	d, e := ioutil.TempDir(os.TempDir(), "gonf")
	if e != nil {
		t.Error("failed to acquire temporary directory...")
	}
	cf := filepath.Join(d, "gonf.json")
	defer os.Remove(cf)

	// clear all inputs so we start from a blank slate
	os.Args = []string{}
	os.Clearenv()

	// set defaults for overrides
	var fileStat = &mockStat{modTime: time.Now()}
	var statError error
	var createFile *os.File
	var createError error
	var readfileError error
	var readfileData []byte
	var exitCode int = 1
	var fmtPrintfData string = ""

	// define overrides
	stat = func(_ string) (os.FileInfo, error) { return fileStat, statError }
	create = func(string) (*os.File, error) { return createFile, createError }
	readfile = func(string) ([]byte, error) { return readfileData, readfileError }
	mkdirall = func(string, os.FileMode) error { return nil }
	exit = func(i int) { exitCode = i }
	fmtPrintf = func(f string, a ...interface{}) (int, error) {
		fmtPrintfData = fmt.Sprintf(f, a...)
		return len(fmtPrintfData), nil
	}

	c := &Config{}
	mc := &mockConfig{}

	// test nil target /w empty, absolute, and relative name overrides
	if c.Load("", cf, "test.gonf.json") == nil {
		t.Error("failed to identify nil target...")
	}
	c.Target(mc)

	// test with matching modtime
	c.configModified = fileStat.ModTime()
	if c.Load(cf) == nil {
		t.Error("failed to capture unchanged file error...")
	}
	statError = mockError

	// test read file error
	readfileError = mockError
	if c.Load(cf) == nil {
		t.Error("failed to capture read file error...")
	}
	readfileError = nil

	// test read file bad json
	readfileData = []byte("this is not json")
	if c.Load(cf) == nil {
		t.Error("failed to capture json parse error...")
	}

	// test read file with file comments and unregistered tags using absolute path
	readfileData = []byte(commentedFileData)
	if c.Load(cf) != nil || mc.FileUnregisteredProperty != "/* this value is also safely parsed */" || mc.FileUnregisteredTag != "// this value is safely parsed" {
		t.Error("failed to filter comments with absolute file...")
	}

	// test read file with file comments and unregistered tags using (default) relative path
	if c.Load() != nil || mc.FileUnregisteredProperty != "/* this value is also safely parsed */" || mc.FileUnregisteredTag != "// this value is safely parsed" {
		t.Error("failed to filter comments with relative file...")
	}

	// test registration by tag
	c.Add("envByTag", "testing environment by tag", "ENV_BY_TAG")
	c.Add("optionByTag", "testing option by tag", "", "--option-by-tag")
	os.Args = []string{"--option-by-tag=test"}
	os.Setenv("ENV_BY_TAG", "test")
	if c.Load() != nil || mc.EnvByTag != "test" || mc.OptionByTag != "test" {
		t.Error("failed to parse input by tags...")
	}
	os.Clearenv()

	// test posix getopt complaint behavior
	c.Add("GetoptSkip", "", "", "-s", "--skip")
	c.Add("GetoptGreedy", "", "", "-e:")
	c.Add("GetoptShortGreedySpace", "", "", "-a:")
	c.Add("GetoptShortGreedyConflict", "", "", "-c:")
	c.Add("GetoptShort", "", "", "-b")
	c.Add("GetoptComboShortL", "", "", "-l")
	c.Add("GetoptComboShortO", "", "", "-o")
	c.Add("GetoptComboShortN", "", "", "-n")
	c.Add("GetoptComboShortG", "", "", "-g:")
	c.Add("GetoptFirstLong", "", "", "--long")
	c.Add("GetoptSecondLong", "", "", "--second")
	c.Add("GetoptLongBool", "", "", "--long-bool")
	os.Args = []string{"-b", "-elong", "-a", "space", "-eshort", "-longshortl-o-n-g", "--long-bool", "--long=testlong1", "--second", "testlong2", "-", "--", "-s", "--skip"}
	if c.Load() != nil || mc.GetoptFirstLong != "testlong1" || mc.GetoptSecondLong != "testlong2" ||
		!mc.GetoptComboShortL || !mc.GetoptComboShortO || !mc.GetoptComboShortN ||
		mc.GetoptComboShortG != "shortl-o-n-g" || !mc.GetoptShort ||
		mc.GetoptShortGreedySpace != "space" || mc.GetoptGreedy != "short" ||
		mc.GetoptShortGreedyConflict != "" || mc.GetoptSkip || !mc.GetoptLongBool {
		t.Error("failed getopt behavior test...")
	}

	// test duplicate registrations with casting
	c.Add("OptionString", "string type via command line option", "", "--optionDuplicate")
	c.Add("OptionNumber", "", "", "--optionDuplicate")
	c.Add("OptionBool", "demonstrate boolean casting and duplicate command line option registration", "", "--optionBool")
	c.Add("EnvString", "string type via environment variable", "ENV_DUPLICATE")
	c.Add("EnvNumber", "demonstrate numeric casting and duplicate environment variable registration", "ENV_DUPLICATE")
	c.Add("EnvBool", "demonstrate boolean casting and duplicate environment variable registration", "ENV_BOOL")
	os.Args = []string{"--optionDuplicate=1.3", "--optionBool"}
	os.Setenv("ENV_BOOL", "true")
	os.Setenv("ENV_DUPLICATE", "12")
	if c.Load(cf) != nil || mc.OptionString != "1.3" || mc.OptionNumber != 1.3 ||
		!mc.OptionBool || mc.EnvString != "12" || mc.EnvNumber != 12 || !mc.EnvBool {
		t.Error("failed to cast or to correctly parse duplicate registrations of environment variables or command line options...")
	}

	// test casting success cases
	c.Add("EnvBool", "", "ENV_BOOL")
	os.Setenv("ENV_BOOL", "true")
	if c.Load(cf) != nil || !mc.EnvBool {
		t.Error("failed to properly cast data types")
	}

	// test casting failure case
	os.Args = []string{"--optionDuplicate=notanumber"}
	if c.Load() == nil {
		t.Error("failed to capture json unmarshal error...")
	}

	// test explicit depth with casting and implicit composite properties
	c.Add("DepthByOption", "", "", "--depth")
	c.Add("DepthByEnv", "", "ENV_DEPTH")
	c.Add("ExplicitComposite.DepthByOption", "", "", "--depth-explicit")
	c.Add("ExplicitComposite.DepthByEnv", "", "ENV_DEPTH_EXPLICIT")
	c.Add("ExplicitComposite.Deeper.TripleDepth", "", "ENV_TRIPLE")
	c.Add("TripleDepth", "", "ENV_TRIPLE")
	os.Args = []string{"--depth", "12", "--depth-explicit=22"}
	os.Setenv("ENV_DEPTH", "true")
	os.Setenv("ENV_DEPTH_EXPLICIT", "true")
	os.Setenv("ENV_TRIPLE", "depth")
	if c.Load(cf) != nil || !mc.Composite.DepthByEnv || !mc.ExplicitComposite.DepthByEnv ||
		mc.Composite.DepthByOption != 12 || mc.ExplicitComposite.DepthByOption != 22 ||
		mc.Composite.Deeper.TripleDepth != "depth" {
		t.Error("failed to handle depth with casting...")
	}

	// test overrides by input type
	readfileData = []byte(`{"EnvOverrideFile": "one"}`)
	c.Add("EnvOverrideFile", "", "ENV_OVERRIDE_FILE")
	c.Add("OptionOverrideEnv", "", "ENV_OVERRIDDEN", "--option-override")
	os.Args = []string{"--option-override=four"}
	os.Setenv("ENV_OVERRIDE_FILE", "two")
	os.Setenv("ENV_OVERRIDDEN", "three")
	if c.Load(cf) != nil || mc.EnvOverrideFile != "two" || mc.OptionOverrideEnv != "four" {
		t.Error("failed to properly order input overrides...")
	}

	// test help without description
	exitCode = 1
	os.Args = []string{"--help"}
	if c.Load(cf) != nil || exitCode != 1 {
		t.Error("failed to process -h help and exit...")
	}

	// test help with `-h`
	exitCode = 1
	c.Description("test help")
	c.Example("test")
	os.Args = []string{"--help"}
	if c.Load(cf) != nil || exitCode != 0 {
		t.Error("failed to process -h help and exit...")
	}

	// test help with `--help`
	exitCode = 1
	os.Args = []string{"--help"}
	if c.Load(cf) != nil || exitCode != 0 {
		t.Error("failed to process --help help and exit...")
	}

	// test help with `help`
	exitCode = 1
	os.Args = []string{"help"}
	if c.Load(cf) != nil || exitCode != 0 {
		t.Error("failed to process help and exit...")
	}
}

func TestReload(t *testing.T) {
	c := &Config{}

	var fileStat = &mockStat{modTime: time.Now()}
	var statError error
	var readfileError error
	var readfileData []byte = []byte("this is not json")

	stat = func(_ string) (os.FileInfo, error) { return fileStat, statError }
	readfile = func(string) ([]byte, error) { return readfileData, readfileError }

	// test without configFile
	if c.Reload() == nil {
		t.Error("failed to identify empty configuration file name...")
	}
	c.configFile = "test.gonf.json"

	// test with matching modTime
	c.configModified = fileStat.ModTime()
	if c.Reload() == nil {
		t.Error("failed to capture unchanged file error...")
	}
	statError = mockError

	// test failure to read file
	readfileError = mockError
	if c.Reload() == nil {
		t.Error("failed to catch error reading file...")
	}
	readfileError = nil

	// test failure to parse
	if c.Reload() == nil {
		t.Error("failed to capture parse error")
	}
	readfileData = []byte(`{"key": "value"}`)

	// test nil target
	if c.Reload() == nil {
		t.Error("failed to identify nil target...")
	}
	c.Target(&mockConfig{})

	// test expecting success
	if e := c.Reload(); e != nil {
		t.Error("failed to successfully parse, %s\n", e)
	}
}

func TestSave(t *testing.T) {
	d, e := ioutil.TempDir(os.TempDir(), "gonf")
	if e != nil {
		t.Error("failed to acquire temporary directory...")
	}
	cf := filepath.Join(d, "gonf.json")
	defer os.Remove(cf)

	var createError error
	var createFile *os.File
	create = func(string) (*os.File, error) { return createFile, createError }

	c := &Config{}

	// test with empty configFile
	if c.Save() == nil {
		t.Error("failed to identify empty configuration file name...")
	}

	// test bypass create error but nil target (eg. fail json encode?)
	c.configFile = cf
	if c.Save() == nil {
		t.Error("failed to capture encoder error...")
	}

	// test create error behavior
	createError = mockError
	if c.Save() == nil {
		t.Error("failed to capture create error...")
	}

	// test with (valid) nil target
	createFile, createError = os.Create(cf)
	if c.Save() != nil {
		t.Error("failed to open temporary file for success scenarior...")
	}
}

func TestHelp(t *testing.T) {
	var exitCode int = 1
	var fmtPrintfData string = ""

	fmtPrintf = func(f string, a ...interface{}) (int, error) {
		fmtPrintfData = fmt.Sprintf(f, a...)
		return len(fmtPrintfData), nil
	}
	exit = func(i int) { exitCode = i }

	c := &Config{}
	c.Add("key", "description", "env", "--option")
	c.Example("--option something")

	// without description
	c.Help()
	if fmtPrintfData != "" && exitCode != 0 {
		t.FailNow()
	}

	// with description
	c.Description("testing help output")
	c.Help()
	if fmtPrintfData == "" && exitCode != 0 {
		t.FailNow()
	}
}

func TestConfigFile(t *testing.T) {
	c := &Config{}
	if c.ConfigFile() != "" {
		t.FailNow()
	}
}
