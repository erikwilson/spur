package cli

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"reflect"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/rancher/spur/generic"
)

var boolFlagTests = []struct {
	name     string
	expected string
}{
	{"help", "--help\t(default: false)"},
	{"h", "-h\t(default: false)"},
}

func resetEnv(env []string) {
	for _, e := range env {
		fields := strings.SplitN(e, "=", 2)
		os.Setenv(fields[0], fields[1])
	}
}

func TestBoolFlagHelpOutput(t *testing.T) {
	for _, test := range boolFlagTests {
		fl := &BoolFlag{Name: test.name}
		output := FlagToString(fl)

		if output != test.expected {
			t.Errorf("%q does not match %q", output, test.expected)
		}
	}
}

func TestBoolFlagApply_SetsAllNames(t *testing.T) {
	v := false
	fl := BoolFlag{Name: "wat", Aliases: []string{"W", "huh"}, Destination: &v}
	set := flag.NewFlagSet("test", 0)
	fl.Apply(set)

	err := set.Parse([]string{"--wat", "-W", "--huh"})
	expect(t, err, nil)
	expect(t, v, true)
}

func TestFlagsFromEnv(t *testing.T) {
	timeUnixString := "526"
	timeUnix := time.Unix(526, 0)
	timeRFC3339String := "2020-05-25T20:20:20-05:00"
	timeRFC3339, _ := time.Parse(time.RFC3339, timeRFC3339String)
	timeKitchenString := "3:00AM"
	timeKitchen, _ := time.Parse(time.Kitchen, timeKitchenString)
	timeCustomString := "Friday"
	timeCustom, _ := time.Parse(timeCustomString, timeCustomString)

	generic.TimeLayouts = append(generic.TimeLayouts, timeCustomString)

	stringToTime := generic.FromStringMap["time.Time"]
	generic.FromStringMap["time.Time"] = func(s string) (interface{}, error) {
		if v, err := stringToTime(s); err == nil {
			return v, nil
		}
		v, err := strconv.ParseInt(s, 0, 64)
		return time.Unix(v, 0), err
	}

	var flagTests = []struct {
		input     string
		output    interface{}
		flag      Flag
		errRegexp string
	}{
		{timeUnixString, timeUnix, &TimeFlag{Name: "time", EnvVars: []string{"TIME"}}, ""},
		{timeRFC3339String, timeRFC3339, &TimeFlag{Name: "time", EnvVars: []string{"TIME"}}, ""},
		{timeKitchenString, timeKitchen, &TimeFlag{Name: "time", EnvVars: []string{"TIME"}}, ""},
		{timeCustomString, timeCustom, &TimeFlag{Name: "time", EnvVars: []string{"TIME"}}, ""},
		{"foobar", false, &TimeFlag{Name: "time", EnvVars: []string{"TIME"}}, `could not parse "foobar" as time value for flag time: .*`},

		{"8m46s", 526 * time.Second, &DurationFlag{Name: "time", EnvVars: []string{"TIME"}}, ""},
		{"foobar", false, &DurationFlag{Name: "time", EnvVars: []string{"TIME"}}, `could not parse "foobar" as duration value for flag time: .*`},

		{"", false, &BoolFlag{Name: "debug", EnvVars: []string{"DEBUG"}}, ""},
		{"1", true, &BoolFlag{Name: "debug", EnvVars: []string{"DEBUG"}}, ""},
		{"false", false, &BoolFlag{Name: "debug", EnvVars: []string{"DEBUG"}}, ""},
		{"foobar", true, &BoolFlag{Name: "debug", EnvVars: []string{"DEBUG"}}, `could not parse "foobar" as bool value for flag debug: .*`},

		{"1.2", 1.2, &Float64Flag{Name: "seconds", EnvVars: []string{"SECONDS"}}, ""},
		{"1", 1.0, &Float64Flag{Name: "seconds", EnvVars: []string{"SECONDS"}}, ""},
		{"foobar", 0, &Float64Flag{Name: "seconds", EnvVars: []string{"SECONDS"}}, `could not parse "foobar" as float64 value for flag seconds: .*`},

		{"1", int64(1), &Int64Flag{Name: "seconds", EnvVars: []string{"SECONDS"}}, ""},
		{"1.2", 0, &Int64Flag{Name: "seconds", EnvVars: []string{"SECONDS"}}, `could not parse "1.2" as int64 value for flag seconds: .*`},
		{"foobar", 0, &Int64Flag{Name: "seconds", EnvVars: []string{"SECONDS"}}, `could not parse "foobar" as int64 value for flag seconds: .*`},

		{"1", 1, &IntFlag{Name: "seconds", EnvVars: []string{"SECONDS"}}, ""},
		{"1.2", 0, &IntFlag{Name: "seconds", EnvVars: []string{"SECONDS"}}, `could not parse "1.2" as int value for flag seconds: .*`},
		{"foobar", 0, &IntFlag{Name: "seconds", EnvVars: []string{"SECONDS"}}, `could not parse "foobar" as int value for flag seconds: .*`},

		{"1,2", []int{1, 2}, &IntSliceFlag{Name: "seconds", EnvVars: []string{"SECONDS"}}, ""},
		{"1.2,2", []int{}, &IntSliceFlag{Name: "seconds", EnvVars: []string{"SECONDS"}}, `could not parse "1.2,2" as int slice value for flag seconds: .*`},
		{"foobar", []int{}, &IntSliceFlag{Name: "seconds", EnvVars: []string{"SECONDS"}}, `could not parse "foobar" as int slice value for flag seconds: .*`},

		{"1,2", []int64{1, 2}, &Int64SliceFlag{Name: "seconds", EnvVars: []string{"SECONDS"}}, ""},
		{"1.2,2", []int64{}, &Int64SliceFlag{Name: "seconds", EnvVars: []string{"SECONDS"}}, `could not parse "1.2,2" as int64 slice value for flag seconds: .*`},
		{"foobar", []int64{}, &Int64SliceFlag{Name: "seconds", EnvVars: []string{"SECONDS"}}, `could not parse "foobar" as int64 slice value for flag seconds: .*`},

		{"foo", "foo", &StringFlag{Name: "name", EnvVars: []string{"NAME"}}, ""},

		{"foo,bar", []string{"foo", "bar"}, &StringSliceFlag{Name: "names", EnvVars: []string{"NAMES"}}, ""},

		{"1", uint(1), &UintFlag{Name: "seconds", EnvVars: []string{"SECONDS"}}, ""},
		{"1.2", 0, &UintFlag{Name: "seconds", EnvVars: []string{"SECONDS"}}, `could not parse "1.2" as uint value for flag seconds: .*`},
		{"foobar", 0, &UintFlag{Name: "seconds", EnvVars: []string{"SECONDS"}}, `could not parse "foobar" as uint value for flag seconds: .*`},

		{"1", uint64(1), &Uint64Flag{Name: "seconds", EnvVars: []string{"SECONDS"}}, ""},
		{"1.2", 0, &Uint64Flag{Name: "seconds", EnvVars: []string{"SECONDS"}}, `could not parse "1.2" as uint64 value for flag seconds: .*`},
		{"foobar", 0, &Uint64Flag{Name: "seconds", EnvVars: []string{"SECONDS"}}, `could not parse "foobar" as uint64 value for flag seconds: .*`},

		{"foo,bar", &Parser{"foo", "bar"}, &GenericFlag{Name: "names", Value: &Parser{}, EnvVars: []string{"NAMES"}}, ""},
	}

	for i, test := range flagTests {
		defer resetEnv(os.Environ())
		os.Clearenv()
		envVars, _ := getFlagEnvVars(test.flag)
		os.Setenv(envVars[0], test.input)

		a := App{
			Flags: []Flag{test.flag},
			Action: func(ctx *Context) error {
				if !reflect.DeepEqual(ctx.Value(FlagNames(test.flag)[0]), test.output) {
					t.Errorf("ex:%01d expected %q to be parsed as %#v, instead was %#v", i, test.input, test.output, ctx.Value(FlagNames(test.flag)[0]))
				}
				return nil
			},
		}

		err := a.Run([]string{"run"})

		if test.errRegexp != "" {
			if err == nil {
				t.Errorf("expected error to match %q, got none", test.errRegexp)
			} else {
				if matched, _ := regexp.MatchString(test.errRegexp, err.Error()); !matched {
					t.Errorf("expected error to match %q, got error %s", test.errRegexp, err)
				}
			}
		} else {
			if err != nil && test.errRegexp == "" {
				t.Errorf("expected no error got %q", err)
			}
		}
	}
}

var stringFlagTests = []struct {
	name     string
	aliases  []string
	usage    string
	value    string
	expected string
}{
	{"foo", nil, "", "", "--foo value\t"},
	{"f", nil, "", "", "-f value\t"},
	{"f", nil, "The total `foo` desired", "all", "-f foo\tThe total foo desired (default: \"all\")"},
	{"test", nil, "", "Something", "--test value\t(default: \"Something\")"},
	{"config", []string{"c"}, "Load configuration from `FILE`", "", "--config FILE, -c FILE\tLoad configuration from FILE"},
	{"config", []string{"c"}, "Load configuration from `CONFIG`", "config.json", "--config CONFIG, -c CONFIG\tLoad configuration from CONFIG (default: \"config.json\")"},
}

func TestStringFlagHelpOutput(t *testing.T) {
	for _, test := range stringFlagTests {
		fl := &StringFlag{Name: test.name, Aliases: test.aliases, Usage: test.usage, Value: test.value}
		output := FlagToString(fl)

		if output != test.expected {
			t.Errorf("%q does not match %q", output, test.expected)
		}
	}
}

func TestStringFlagDefaultText(t *testing.T) {
	fl := &StringFlag{Name: "foo", Aliases: nil, Usage: "amount of `foo` requested", Value: "none", DefaultText: "all of it"}
	expected := "--foo foo\tamount of foo requested (default: all of it)"
	output := FlagToString(fl)

	if output != expected {
		t.Errorf("%q does not match %q", output, expected)
	}
}

func TestStringFlagWithEnvVarHelpOutput(t *testing.T) {
	defer resetEnv(os.Environ())
	os.Clearenv()
	os.Setenv("APP_FOO", "derp")

	for _, test := range stringFlagTests {
		fl := &StringFlag{Name: test.name, Aliases: test.aliases, Value: test.value, EnvVars: []string{"APP_FOO"}}
		output := FlagToString(fl)

		expectedSuffix := " [$APP_FOO]"
		if runtime.GOOS == "windows" {
			expectedSuffix = " [%APP_FOO%]"
		}
		if !strings.HasSuffix(output, expectedSuffix) {
			t.Errorf("%s does not end with"+expectedSuffix, output)
		}
	}
}

var prefixStringFlagTests = []struct {
	name     string
	aliases  []string
	usage    string
	value    string
	prefixer FlagNamePrefixFunc
	expected string
}{
	{name: "foo", usage: "", value: "", prefixer: func(a []string, b string) string {
		return fmt.Sprintf("name: %s, ph: %s", a, b)
	}, expected: "name: foo, ph: value\t"},
	{name: "f", usage: "", value: "", prefixer: func(a []string, b string) string {
		return fmt.Sprintf("name: %s, ph: %s", a, b)
	}, expected: "name: f, ph: value\t"},
	{name: "f", usage: "The total `foo` desired", value: "all", prefixer: func(a []string, b string) string {
		return fmt.Sprintf("name: %s, ph: %s", a, b)
	}, expected: "name: f, ph: foo\tThe total foo desired (default: \"all\")"},
	{name: "test", usage: "", value: "Something", prefixer: func(a []string, b string) string {
		return fmt.Sprintf("name: %s, ph: %s", a, b)
	}, expected: "name: test, ph: value\t(default: \"Something\")"},
	{name: "config", aliases: []string{"c"}, usage: "Load configuration from `FILE`", value: "", prefixer: func(a []string, b string) string {
		return fmt.Sprintf("name: %s, ph: %s", a, b)
	}, expected: "name: config,c, ph: FILE\tLoad configuration from FILE"},
	{name: "config", aliases: []string{"c"}, usage: "Load configuration from `CONFIG`", value: "config.json", prefixer: func(a []string, b string) string {
		return fmt.Sprintf("name: %s, ph: %s", a, b)
	}, expected: "name: config,c, ph: CONFIG\tLoad configuration from CONFIG (default: \"config.json\")"},
}

func TestStringFlagApply_SetsAllNames(t *testing.T) {
	v := "mmm"
	fl := StringFlag{Name: "hay", Aliases: []string{"H", "hayyy"}, Destination: &v}
	set := flag.NewFlagSet("test", 0)
	fl.Apply(set)

	err := set.Parse([]string{"--hay", "u", "-H", "yuu", "--hayyy", "YUUUU"})
	expect(t, err, nil)
	expect(t, v, "YUUUU")
}

var pathFlagTests = []struct {
	name     string
	aliases  []string
	usage    string
	value    string
	expected string
}{
	{"f", nil, "", "", "-f value\t"},
	{"f", nil, "Path is the `path` of file", "/path/to/file", "-f path\tPath is the path of file (default: \"/path/to/file\")"},
}

var stringSliceFlagTests = []struct {
	name     string
	aliases  []string
	value    StringSlice
	expected string
}{
	{"foo", nil, []string{""}, "--foo value\t"},
	{"f", nil, []string{""}, "-f value\t"},
	{"f", nil, []string{"Lipstick"}, "-f value\t(default: \"Lipstick\")"},
	{"test", nil, []string{"Something"}, "--test value\t(default: \"Something\")"},
	{"dee", []string{"d"}, []string{"Inka", "Dinka", "dooo"}, "--dee value, -d value\t(default: \"Inka\", \"Dinka\", \"dooo\")"},
}

func TestStringSliceFlagHelpOutput(t *testing.T) {
	for _, test := range stringSliceFlagTests {
		f := &StringSliceFlag{Name: test.name, Aliases: test.aliases, Value: test.value}
		output := FlagToString(f)

		if output != test.expected {
			t.Errorf("%q does not match %q", output, test.expected)
		}
	}
}

func TestStringSliceFlagWithEnvVarHelpOutput(t *testing.T) {
	defer resetEnv(os.Environ())
	os.Clearenv()
	os.Setenv("APP_QWWX", "11,4")

	for _, test := range stringSliceFlagTests {
		fl := &StringSliceFlag{Name: test.name, Aliases: test.aliases, Value: test.value, EnvVars: []string{"APP_QWWX"}}
		output := FlagToString(fl)

		expectedSuffix := " [$APP_QWWX]"
		if runtime.GOOS == "windows" {
			expectedSuffix = " [%APP_QWWX%]"
		}
		if !strings.HasSuffix(output, expectedSuffix) {
			t.Errorf("%q does not end with"+expectedSuffix, output)
		}
	}
}

func TestStringSliceFlagApply_SetsAllNames(t *testing.T) {
	fl := StringSliceFlag{Name: "goat", Aliases: []string{"G", "gooots"}}
	set := flag.NewFlagSet("test", 0)
	fl.Apply(set)

	err := set.Parse([]string{"--goat", "aaa", "-G", "bbb", "--gooots", "eeeee"})
	expect(t, err, nil)
}

func TestStringSliceFlagApply_DefaultValueWithDestination(t *testing.T) {
	defValue := []string{"UA", "US"}
	dest := []string{"CA"}
	fl := StringSliceFlag{Name: "country", Value: defValue, Destination: &dest}
	set := flag.NewFlagSet("test", 0)
	fl.Apply(set)

	err := set.Parse([]string{})
	expect(t, err, nil)
	expect(t, defValue, dest)
}

var intFlagTests = []struct {
	name     string
	expected string
}{
	{"hats", "--hats value\t(default: 9)"},
	{"H", "-H value\t(default: 9)"},
}

func TestIntFlagHelpOutput(t *testing.T) {
	for _, test := range intFlagTests {
		fl := &IntFlag{Name: test.name, Value: 9}
		output := FlagToString(fl)

		if output != test.expected {
			t.Errorf("%s does not match %s", output, test.expected)
		}
	}
}

func TestIntFlagWithEnvVarHelpOutput(t *testing.T) {
	defer resetEnv(os.Environ())
	os.Clearenv()
	os.Setenv("APP_BAR", "2")

	for _, test := range intFlagTests {
		fl := &IntFlag{Name: test.name, EnvVars: []string{"APP_BAR"}}
		output := FlagToString(fl)

		expectedSuffix := " [$APP_BAR]"
		if runtime.GOOS == "windows" {
			expectedSuffix = " [%APP_BAR%]"
		}
		if !strings.HasSuffix(output, expectedSuffix) {
			t.Errorf("%s does not end with"+expectedSuffix, output)
		}
	}
}

func TestIntFlagApply_SetsAllNames(t *testing.T) {
	v := 3
	fl := IntFlag{Name: "banana", Aliases: []string{"B", "banannanana"}, Destination: &v}
	set := flag.NewFlagSet("test", 0)
	fl.Apply(set)

	err := set.Parse([]string{"--banana", "1", "-B", "2", "--banannanana", "5"})
	expect(t, err, nil)
	expect(t, v, 5)
}

var int64FlagTests = []struct {
	name     string
	expected string
}{
	{"hats", "--hats value\t(default: 8589934592)"},
	{"H", "-H value\t(default: 8589934592)"},
}

func TestInt64FlagHelpOutput(t *testing.T) {
	for _, test := range int64FlagTests {
		fl := &Int64Flag{Name: test.name, Value: 8589934592}
		output := FlagToString(fl)

		if output != test.expected {
			t.Errorf("%s does not match %s", output, test.expected)
		}
	}
}

func TestInt64FlagWithEnvVarHelpOutput(t *testing.T) {
	defer resetEnv(os.Environ())
	os.Clearenv()
	os.Setenv("APP_BAR", "2")

	for _, test := range int64FlagTests {
		fl := &IntFlag{Name: test.name, EnvVars: []string{"APP_BAR"}}
		output := FlagToString(fl)

		expectedSuffix := " [$APP_BAR]"
		if runtime.GOOS == "windows" {
			expectedSuffix = " [%APP_BAR%]"
		}
		if !strings.HasSuffix(output, expectedSuffix) {
			t.Errorf("%s does not end with"+expectedSuffix, output)
		}
	}
}

var uintFlagTests = []struct {
	name     string
	expected string
}{
	{"nerfs", "--nerfs value\t(default: 41)"},
	{"N", "-N value\t(default: 41)"},
}

func TestUintFlagHelpOutput(t *testing.T) {
	for _, test := range uintFlagTests {
		fl := &UintFlag{Name: test.name, Value: 41}
		output := FlagToString(fl)

		if output != test.expected {
			t.Errorf("%s does not match %s", output, test.expected)
		}
	}
}

func TestUintFlagWithEnvVarHelpOutput(t *testing.T) {
	defer resetEnv(os.Environ())
	os.Clearenv()
	os.Setenv("APP_BAR", "2")

	for _, test := range uintFlagTests {
		fl := &UintFlag{Name: test.name, EnvVars: []string{"APP_BAR"}}
		output := FlagToString(fl)

		expectedSuffix := " [$APP_BAR]"
		if runtime.GOOS == "windows" {
			expectedSuffix = " [%APP_BAR%]"
		}
		if !strings.HasSuffix(output, expectedSuffix) {
			t.Errorf("%s does not end with"+expectedSuffix, output)
		}
	}
}

var uint64FlagTests = []struct {
	name     string
	expected string
}{
	{"gerfs", "--gerfs value\t(default: 8589934582)"},
	{"G", "-G value\t(default: 8589934582)"},
}

func TestUint64FlagHelpOutput(t *testing.T) {
	for _, test := range uint64FlagTests {
		fl := &Uint64Flag{Name: test.name, Value: 8589934582}
		output := FlagToString(fl)

		if output != test.expected {
			t.Errorf("%s does not match %s", output, test.expected)
		}
	}
}

func TestUint64FlagWithEnvVarHelpOutput(t *testing.T) {
	defer resetEnv(os.Environ())
	os.Clearenv()
	os.Setenv("APP_BAR", "2")

	for _, test := range uint64FlagTests {
		fl := &UintFlag{Name: test.name, EnvVars: []string{"APP_BAR"}}
		output := FlagToString(fl)

		expectedSuffix := " [$APP_BAR]"
		if runtime.GOOS == "windows" {
			expectedSuffix = " [%APP_BAR%]"
		}
		if !strings.HasSuffix(output, expectedSuffix) {
			t.Errorf("%s does not end with"+expectedSuffix, output)
		}
	}
}

var durationFlagTests = []struct {
	name     string
	expected string
}{
	{"hooting", "--hooting value\t(default: 1s)"},
	{"H", "-H value\t(default: 1s)"},
}

func TestDurationFlagHelpOutput(t *testing.T) {
	for _, test := range durationFlagTests {
		fl := &DurationFlag{Name: test.name, Value: 1 * time.Second}
		output := FlagToString(fl)

		if output != test.expected {
			t.Errorf("%q does not match %q", output, test.expected)
		}
	}
}

func TestDurationFlagWithEnvVarHelpOutput(t *testing.T) {
	defer resetEnv(os.Environ())
	os.Clearenv()
	os.Setenv("APP_BAR", "2h3m6s")

	for _, test := range durationFlagTests {
		fl := &DurationFlag{Name: test.name, EnvVars: []string{"APP_BAR"}}
		output := FlagToString(fl)

		expectedSuffix := " [$APP_BAR]"
		if runtime.GOOS == "windows" {
			expectedSuffix = " [%APP_BAR%]"
		}
		if !strings.HasSuffix(output, expectedSuffix) {
			t.Errorf("%s does not end with"+expectedSuffix, output)
		}
	}
}

func TestDurationFlagApply_SetsAllNames(t *testing.T) {
	v := time.Second * 20
	fl := DurationFlag{Name: "howmuch", Aliases: []string{"H", "whyyy"}, Destination: &v}
	set := flag.NewFlagSet("test", 0)
	fl.Apply(set)

	err := set.Parse([]string{"--howmuch", "30s", "-H", "5m", "--whyyy", "30h"})
	expect(t, err, nil)
	expect(t, v, time.Hour*30)
}

var intSliceFlagTests = []struct {
	name     string
	aliases  []string
	value    IntSlice
	expected string
}{
	{"heads", nil, []int{}, "--heads value\t"},
	{"H", nil, []int{}, "-H value\t"},
	{"H", []string{"heads"}, []int{9, 3}, "-H value, --heads value\t(default: 9, 3)"},
}

func TestIntSliceFlagHelpOutput(t *testing.T) {
	for _, test := range intSliceFlagTests {
		fl := &IntSliceFlag{Name: test.name, Aliases: test.aliases, Value: test.value}
		output := FlagToString(fl)

		if output != test.expected {
			t.Errorf("%q does not match %q", output, test.expected)
		}
	}
}

func TestIntSliceFlagWithEnvVarHelpOutput(t *testing.T) {
	defer resetEnv(os.Environ())
	os.Clearenv()
	os.Setenv("APP_SMURF", "42,3")

	for _, test := range intSliceFlagTests {
		fl := &IntSliceFlag{Name: test.name, Aliases: test.aliases, Value: test.value, EnvVars: []string{"APP_SMURF"}}
		output := FlagToString(fl)

		expectedSuffix := " [$APP_SMURF]"
		if runtime.GOOS == "windows" {
			expectedSuffix = " [%APP_SMURF%]"
		}
		if !strings.HasSuffix(output, expectedSuffix) {
			t.Errorf("%q does not end with"+expectedSuffix, output)
		}
	}
}

func TestIntSliceFlagApply_SetsAllNames(t *testing.T) {
	fl := IntSliceFlag{Name: "bits", Aliases: []string{"B", "bips"}}
	set := flag.NewFlagSet("test", 0)
	fl.Apply(set)

	err := set.Parse([]string{"--bits", "23", "-B", "3", "--bips", "99"})
	expect(t, err, nil)
}

var int64SliceFlagTests = []struct {
	name     string
	aliases  []string
	value    Int64Slice
	expected string
}{
	{"heads", nil, []int64{}, "--heads value\t"},
	{"H", nil, []int64{}, "-H value\t"},
	{"heads", []string{"H"}, []int64{int64(2), int64(17179869184)},
		"--heads value, -H value\t(default: 2, 17179869184)"},
}

func TestInt64SliceFlagHelpOutput(t *testing.T) {
	for _, test := range int64SliceFlagTests {
		fl := &Int64SliceFlag{Name: test.name, Aliases: test.aliases, Value: test.value}
		output := FlagToString(fl)

		if output != test.expected {
			t.Errorf("%q does not match %q", output, test.expected)
		}
	}
}

func TestInt64SliceFlagWithEnvVarHelpOutput(t *testing.T) {
	defer resetEnv(os.Environ())
	os.Clearenv()
	os.Setenv("APP_SMURF", "42,17179869184")

	for _, test := range int64SliceFlagTests {
		fl := &Int64SliceFlag{Name: test.name, Value: test.value, EnvVars: []string{"APP_SMURF"}}
		output := FlagToString(fl)

		expectedSuffix := " [$APP_SMURF]"
		if runtime.GOOS == "windows" {
			expectedSuffix = " [%APP_SMURF%]"
		}
		if !strings.HasSuffix(output, expectedSuffix) {
			t.Errorf("%q does not end with"+expectedSuffix, output)
		}
	}
}

var float64FlagTests = []struct {
	name     string
	expected string
}{
	{"hooting", "--hooting value\t(default: 0.1)"},
	{"H", "-H value\t(default: 0.1)"},
}

func TestFloat64FlagHelpOutput(t *testing.T) {
	for _, test := range float64FlagTests {
		f := &Float64Flag{Name: test.name, Value: 0.1}
		output := FlagToString(f)

		if output != test.expected {
			t.Errorf("%q does not match %q", output, test.expected)
		}
	}
}

func TestFloat64FlagWithEnvVarHelpOutput(t *testing.T) {
	defer resetEnv(os.Environ())
	os.Clearenv()
	os.Setenv("APP_BAZ", "99.4")

	for _, test := range float64FlagTests {
		fl := &Float64Flag{Name: test.name, EnvVars: []string{"APP_BAZ"}}
		output := FlagToString(fl)

		expectedSuffix := " [$APP_BAZ]"
		if runtime.GOOS == "windows" {
			expectedSuffix = " [%APP_BAZ%]"
		}
		if !strings.HasSuffix(output, expectedSuffix) {
			t.Errorf("%s does not end with"+expectedSuffix, output)
		}
	}
}

func TestFloat64FlagApply_SetsAllNames(t *testing.T) {
	v := 99.1
	fl := Float64Flag{Name: "noodles", Aliases: []string{"N", "nurbles"}, Destination: &v}
	set := flag.NewFlagSet("test", 0)
	fl.Apply(set)

	err := set.Parse([]string{"--noodles", "1.3", "-N", "11", "--nurbles", "43.33333"})
	expect(t, err, nil)
	expect(t, v, float64(43.33333))
}

var float64SliceFlagTests = []struct {
	name     string
	aliases  []string
	value    Float64Slice
	expected string
}{
	{"heads", nil, []float64{}, "--heads value\t"},
	{"H", nil, []float64{}, "-H value\t"},
	{"heads", []string{"H"}, []float64{0.1234, -10.5},
		"--heads value, -H value\t(default: 0.1234, -10.5)"},
}

func TestFloat64SliceFlagHelpOutput(t *testing.T) {
	for _, test := range float64SliceFlagTests {
		fl := &Float64SliceFlag{Name: test.name, Aliases: test.aliases, Value: test.value}
		output := FlagToString(fl)

		if output != test.expected {
			t.Errorf("%q does not match %q", output, test.expected)
		}
	}
}

func TestFloat64SliceFlagWithEnvVarHelpOutput(t *testing.T) {
	defer resetEnv(os.Environ())
	os.Clearenv()
	os.Setenv("APP_SMURF", "0.1234,-10.5")
	for _, test := range float64SliceFlagTests {
		fl := &Float64SliceFlag{Name: test.name, Value: test.value, EnvVars: []string{"APP_SMURF"}}
		output := FlagToString(fl)

		expectedSuffix := " [$APP_SMURF]"
		if runtime.GOOS == "windows" {
			expectedSuffix = " [%APP_SMURF%]"
		}
		if !strings.HasSuffix(output, expectedSuffix) {
			t.Errorf("%q does not end with"+expectedSuffix, output)
		}
	}
}

var genericFlagTests = []struct {
	name     string
	value    Generic
	expected string
}{
	{"toads", &Parser{"abc", "def"}, "--toads value\ttest flag (default: abc,def)"},
	{"t", &Parser{"abc", "def"}, "-t value\ttest flag (default: abc,def)"},
}

func TestGenericFlagHelpOutput(t *testing.T) {
	for _, test := range genericFlagTests {
		fl := &GenericFlag{Name: test.name, Value: test.value, Usage: "test flag"}
		output := FlagToString(fl)

		if output != test.expected {
			t.Errorf("%q does not match %q", output, test.expected)
		}
	}
}

func TestGenericFlagWithEnvVarHelpOutput(t *testing.T) {
	defer resetEnv(os.Environ())
	os.Clearenv()
	os.Setenv("APP_ZAP", "3")

	for _, test := range genericFlagTests {
		fl := &GenericFlag{Name: test.name, EnvVars: []string{"APP_ZAP"}}
		output := FlagToString(fl)

		expectedSuffix := " [$APP_ZAP]"
		if runtime.GOOS == "windows" {
			expectedSuffix = " [%APP_ZAP%]"
		}
		if !strings.HasSuffix(output, expectedSuffix) {
			t.Errorf("%s does not end with"+expectedSuffix, output)
		}
	}
}

func TestGenericFlagApply_SetsAllNames(t *testing.T) {
	fl := GenericFlag{Name: "orbs", Aliases: []string{"O", "obrs"}, Value: &Parser{}}
	set := flag.NewFlagSet("test", 0)
	fl.Apply(set)

	err := set.Parse([]string{"--orbs", "eleventy,3", "-O", "4,bloop", "--obrs", "19,s"})
	expect(t, err, nil)
}

func TestParseMultiString(t *testing.T) {
	err := (&App{
		Flags: []Flag{
			&StringFlag{Name: "serve", Aliases: []string{"s"}},
		},
		Action: func(ctx *Context) error {
			if ctx.String("serve") != "10" {
				t.Errorf("main name not set")
			}
			if ctx.String("s") != "10" {
				t.Errorf("short name not set")
			}
			return nil
		},
	}).Run([]string{"run", "-s", "10"})
	if !reflect.DeepEqual(err, nil) {
		t.Errorf("test failure: %v", err)
	}
}

func TestParseDestinationString(t *testing.T) {
	var dest string
	err := (&App{
		Flags: []Flag{
			&StringFlag{
				Name:        "dest",
				Destination: &dest,
			},
		},
		Action: func(ctx *Context) error {
			if dest != "10" {
				t.Errorf("expected destination String 10")
			}
			return nil
		},
	}).Run([]string{"run", "--dest", "10"})
	if !reflect.DeepEqual(err, nil) {
		t.Errorf("test failure: %v", err)
	}
}

func TestParseMultiStringFromEnv(t *testing.T) {
	defer resetEnv(os.Environ())
	os.Clearenv()
	os.Setenv("APP_COUNT", "20")
	err := (&App{
		Flags: []Flag{
			&StringFlag{Name: "count", Aliases: []string{"c"}, EnvVars: []string{"APP_COUNT"}},
		},
		Action: func(ctx *Context) error {
			if ctx.String("count") != "20" {
				t.Errorf("main name not set")
			}
			if ctx.String("c") != "20" {
				t.Errorf("short name not set")
			}
			return nil
		},
	}).Run([]string{"run"})
	if !reflect.DeepEqual(err, nil) {
		t.Errorf("test failure: %v", err)
	}
}

func TestParseMultiStringFromEnvCascade(t *testing.T) {
	defer resetEnv(os.Environ())
	os.Clearenv()
	os.Setenv("APP_COUNT", "20")
	err := (&App{
		Flags: []Flag{
			&StringFlag{Name: "count", Aliases: []string{"c"}, EnvVars: []string{"COMPAT_COUNT", "APP_COUNT"}},
		},
		Action: func(ctx *Context) error {
			if ctx.String("count") != "20" {
				t.Errorf("main name not set")
			}
			if ctx.String("c") != "20" {
				t.Errorf("short name not set")
			}
			return nil
		},
	}).Run([]string{"run"})
	if !reflect.DeepEqual(err, nil) {
		t.Errorf("test failure: %v", err)
	}
}

func TestParseMultiStringSlice(t *testing.T) {
	err := (&App{
		Flags: []Flag{
			&StringSliceFlag{Name: "serve", Aliases: []string{"s"}, Value: []string{}},
		},
		Action: func(ctx *Context) error {
			expected := []string{"10", "20"}
			if !reflect.DeepEqual(ctx.StringSlice("serve"), expected) {
				t.Errorf("main name not set: %v != %v", expected, ctx.StringSlice("serve"))
			}
			if !reflect.DeepEqual(ctx.StringSlice("s"), expected) {
				t.Errorf("short name not set: %v != %v", expected, ctx.StringSlice("s"))
			}
			return nil
		},
	}).Run([]string{"run", "-s", "10", "-s", "20"})
	if !reflect.DeepEqual(err, nil) {
		t.Errorf("test failure: %v", err)
	}
}

func TestParseMultiStringSliceWithDefaults(t *testing.T) {
	err := (&App{
		Flags: []Flag{
			&StringSliceFlag{Name: "serve", Aliases: []string{"s"}, Value: []string{"9", "2"}},
		},
		Action: func(ctx *Context) error {
			expected := []string{"10", "20"}
			if !reflect.DeepEqual(ctx.StringSlice("serve"), expected) {
				t.Errorf("main name not set: %v != %v", expected, ctx.StringSlice("serve"))
			}
			if !reflect.DeepEqual(ctx.StringSlice("s"), expected) {
				t.Errorf("short name not set: %v != %v", expected, ctx.StringSlice("s"))
			}
			return nil
		},
	}).Run([]string{"run", "-s", "10", "-s", "20"})
	if !reflect.DeepEqual(err, nil) {
		t.Errorf("test failure: %v", err)
	}
}

func TestParseMultiStringSliceWithDestination(t *testing.T) {
	dest := &StringSlice{}
	err := (&App{
		Flags: []Flag{
			&StringSliceFlag{Name: "serve", Aliases: []string{"s"}, Destination: dest},
		},
		Action: func(ctx *Context) error {
			expected := []string{"10", "20"}
			if !reflect.DeepEqual(*dest, expected) {
				t.Errorf("main name not set: %v != %v", expected, ctx.StringSlice("serve"))
			}
			if !reflect.DeepEqual(*dest, expected) {
				t.Errorf("short name not set: %v != %v", expected, ctx.StringSlice("s"))
			}
			return nil
		},
	}).Run([]string{"run", "-s", "10", "-s", "20"})
	if !reflect.DeepEqual(err, nil) {
		t.Errorf("test failure: %v", err)
	}
}

func TestParseMultiStringSliceWithDestinationAndEnv(t *testing.T) {
	defer resetEnv(os.Environ())
	os.Clearenv()
	os.Setenv("APP_INTERVALS", "20,30,40")

	dest := &StringSlice{}
	err := (&App{
		Flags: []Flag{
			&StringSliceFlag{Name: "serve", Aliases: []string{"s"}, Destination: dest, EnvVars: []string{"APP_INTERVALS"}},
		},
		Action: func(ctx *Context) error {
			expected := []string{"10", "20"}
			if !reflect.DeepEqual(*dest, expected) {
				t.Errorf("main name not set: %v != %v", expected, ctx.StringSlice("serve"))
			}
			if !reflect.DeepEqual(*dest, expected) {
				t.Errorf("short name not set: %v != %v", expected, ctx.StringSlice("s"))
			}
			return nil
		},
	}).Run([]string{"run", "-s", "10", "-s", "20"})
	if !reflect.DeepEqual(err, nil) {
		t.Errorf("test failure: %v", err)
	}
}

func TestParseMultiStringSliceWithDefaultsUnset(t *testing.T) {
	err := (&App{
		Flags: []Flag{
			&StringSliceFlag{Name: "serve", Aliases: []string{"s"}, Value: []string{"9", "2"}},
		},
		Action: func(ctx *Context) error {
			if !reflect.DeepEqual(ctx.StringSlice("serve"), []string{"9", "2"}) {
				t.Errorf("main name not set: %v", ctx.StringSlice("serve"))
			}
			if !reflect.DeepEqual(ctx.StringSlice("s"), []string{"9", "2"}) {
				t.Errorf("short name not set: %v", ctx.StringSlice("s"))
			}
			return nil
		},
	}).Run([]string{"run"})
	if !reflect.DeepEqual(err, nil) {
		t.Errorf("test failure: %v", err)
	}
}

func TestParseMultiStringSliceFromEnv(t *testing.T) {
	defer resetEnv(os.Environ())
	os.Clearenv()
	os.Setenv("APP_INTERVALS", "20,30,40")
	dest := []string{"hello", "world"}
	flag := &StringSliceFlag{Name: "intervals", Aliases: []string{"i"}, Value: []string{"ok"}, Destination: &dest, EnvVars: []string{"APP_INTERVALS"}}
	err := (&App{
		Flags: []Flag{flag},
		Action: func(ctx *Context) error {
			if !reflect.DeepEqual(ctx.StringSlice("intervals"), []string{"20", "30", "40"}) {
				t.Errorf("main name not set from env")
			}
			if !reflect.DeepEqual(ctx.StringSlice("i"), []string{"20", "30", "40"}) {
				t.Errorf("short name not set from env")
			}
			if !reflect.DeepEqual(dest, []string{"20", "30", "40"}) {
				t.Errorf("destination not set from env")
			}
			if !reflect.DeepEqual(flag.Value, []string{"ok"}) {
				t.Errorf("value has been set")
			}
			return nil
		},
	}).Run([]string{"run"})
	if !reflect.DeepEqual(err, nil) {
		t.Errorf("test failure: %v", err)
	}
}

func TestParseMultiStringSliceFromEnvWithDefaults(t *testing.T) {
	defer resetEnv(os.Environ())
	os.Clearenv()
	os.Setenv("APP_INTERVALS", "20,30,40")

	err := (&App{
		Flags: []Flag{
			&StringSliceFlag{Name: "intervals", Aliases: []string{"i"}, Value: []string{"1", "2", "5"}, EnvVars: []string{"APP_INTERVALS"}},
		},
		Action: func(ctx *Context) error {
			if !reflect.DeepEqual(ctx.StringSlice("intervals"), []string{"20", "30", "40"}) {
				t.Errorf("main name not set from env")
			}
			if !reflect.DeepEqual(ctx.StringSlice("i"), []string{"20", "30", "40"}) {
				t.Errorf("short name not set from env")
			}
			return nil
		},
	}).Run([]string{"run"})
	if !reflect.DeepEqual(err, nil) {
		t.Errorf("test failure: %v", err)
	}
}

func TestParseMultiStringSliceFromEnvCascade(t *testing.T) {
	defer resetEnv(os.Environ())
	os.Clearenv()
	os.Setenv("APP_INTERVALS", "20,30,40")

	err := (&App{
		Flags: []Flag{
			&StringSliceFlag{Name: "intervals", Aliases: []string{"i"}, Value: []string{}, EnvVars: []string{"COMPAT_INTERVALS", "APP_INTERVALS"}},
		},
		Action: func(ctx *Context) error {
			if !reflect.DeepEqual(ctx.StringSlice("intervals"), []string{"20", "30", "40"}) {
				t.Errorf("main name not set from env")
			}
			if !reflect.DeepEqual(ctx.StringSlice("i"), []string{"20", "30", "40"}) {
				t.Errorf("short name not set from env")
			}
			return nil
		},
	}).Run([]string{"run"})
	if !reflect.DeepEqual(err, nil) {
		t.Errorf("test failure: %v", err)
	}
}

func TestParseMultiStringSliceFromEnvCascadeWithDefaults(t *testing.T) {
	defer resetEnv(os.Environ())
	os.Clearenv()
	os.Setenv("APP_INTERVALS", "20,30,40")

	err := (&App{
		Flags: []Flag{
			&StringSliceFlag{Name: "intervals", Aliases: []string{"i"}, Value: []string{"1", "2", "5"}, EnvVars: []string{"COMPAT_INTERVALS", "APP_INTERVALS"}},
		},
		Action: func(ctx *Context) error {
			if !reflect.DeepEqual(ctx.StringSlice("intervals"), []string{"20", "30", "40"}) {
				t.Errorf("main name not set from env")
			}
			if !reflect.DeepEqual(ctx.StringSlice("i"), []string{"20", "30", "40"}) {
				t.Errorf("short name not set from env")
			}
			return nil
		},
	}).Run([]string{"run"})
	if !reflect.DeepEqual(err, nil) {
		t.Errorf("test failure: %v", err)
	}
}

func TestParseMultiStringSliceFromEnvWithDestination(t *testing.T) {
	defer resetEnv(os.Environ())
	os.Clearenv()
	os.Setenv("APP_INTERVALS", "20,30,40")

	dest := &StringSlice{}
	err := (&App{
		Flags: []Flag{
			&StringSliceFlag{Name: "intervals", Aliases: []string{"i"}, Destination: dest, EnvVars: []string{"APP_INTERVALS"}},
		},
		Action: func(ctx *Context) error {
			if !reflect.DeepEqual(*dest, []string{"20", "30", "40"}) {
				t.Errorf("main name not set from env")
			}
			if !reflect.DeepEqual(*dest, []string{"20", "30", "40"}) {
				t.Errorf("short name not set from env")
			}
			return nil
		},
	}).Run([]string{"run"})
	if !reflect.DeepEqual(err, nil) {
		t.Errorf("test failure: %v", err)
	}
}

func TestParseMultiInt(t *testing.T) {
	err := (&App{
		Flags: []Flag{
			&IntFlag{Name: "serve", Aliases: []string{"s"}},
		},
		Action: func(ctx *Context) error {
			if ctx.Int("serve") != 10 {
				t.Errorf("main name not set")
			}
			if ctx.Int("s") != 10 {
				t.Errorf("short name not set")
			}
			return nil
		},
	}).Run([]string{"run", "-s", "10"})
	if !reflect.DeepEqual(err, nil) {
		t.Errorf("test failure: %v", err)
	}
}

func TestParseDestinationInt(t *testing.T) {
	var dest int
	err := (&App{
		Flags: []Flag{
			&IntFlag{
				Name:        "dest",
				Destination: &dest,
			},
		},
		Action: func(ctx *Context) error {
			if dest != 10 {
				t.Errorf("expected destination Int 10")
			}
			return nil
		},
	}).Run([]string{"run", "--dest", "10"})
	if !reflect.DeepEqual(err, nil) {
		t.Errorf("test failure: %v", err)
	}
}

func TestParseMultiIntFromEnv(t *testing.T) {
	defer resetEnv(os.Environ())
	os.Clearenv()
	os.Setenv("APP_TIMEOUT_SECONDS", "10")
	err := (&App{
		Flags: []Flag{
			&IntFlag{Name: "timeout", Aliases: []string{"t"}, EnvVars: []string{"APP_TIMEOUT_SECONDS"}},
		},
		Action: func(ctx *Context) error {
			if ctx.Int("timeout") != 10 {
				t.Errorf("main name not set")
			}
			if ctx.Int("t") != 10 {
				t.Errorf("short name not set")
			}
			return nil
		},
	}).Run([]string{"run"})
	if !reflect.DeepEqual(err, nil) {
		t.Errorf("test failure: %v", err)
	}
}

func TestParseMultiIntFromEnvCascade(t *testing.T) {
	defer resetEnv(os.Environ())
	os.Clearenv()
	os.Setenv("APP_TIMEOUT_SECONDS", "10")
	err := (&App{
		Flags: []Flag{
			&IntFlag{Name: "timeout", Aliases: []string{"t"}, EnvVars: []string{"COMPAT_TIMEOUT_SECONDS", "APP_TIMEOUT_SECONDS"}},
		},
		Action: func(ctx *Context) error {
			if ctx.Int("timeout") != 10 {
				t.Errorf("main name not set")
			}
			if ctx.Int("t") != 10 {
				t.Errorf("short name not set")
			}
			return nil
		},
	}).Run([]string{"run"})
	if !reflect.DeepEqual(err, nil) {
		t.Errorf("test failure: %v", err)
	}
}

func TestParseMultiIntSlice(t *testing.T) {
	err := (&App{
		Flags: []Flag{
			&IntSliceFlag{Name: "serve", Aliases: []string{"s"}, Value: []int{}},
		},
		Action: func(ctx *Context) error {
			if !reflect.DeepEqual(ctx.IntSlice("serve"), []int{10, 20}) {
				t.Errorf("main name not set")
			}
			if !reflect.DeepEqual(ctx.IntSlice("s"), []int{10, 20}) {
				t.Errorf("short name not set")
			}
			return nil
		},
	}).Run([]string{"run", "-s", "10", "-s", "20"})
	if !reflect.DeepEqual(err, nil) {
		t.Errorf("test failure: %v", err)
	}
}

func TestParseMultiIntSliceWithDefaults(t *testing.T) {
	err := (&App{
		Flags: []Flag{
			&IntSliceFlag{Name: "serve", Aliases: []string{"s"}, Value: []int{9, 2}},
		},
		Action: func(ctx *Context) error {
			if !reflect.DeepEqual(ctx.IntSlice("serve"), []int{10, 20}) {
				t.Errorf("main name not set")
			}
			if !reflect.DeepEqual(ctx.IntSlice("s"), []int{10, 20}) {
				t.Errorf("short name not set")
			}
			return nil
		},
	}).Run([]string{"run", "-s", "10", "-s", "20"})
	if !reflect.DeepEqual(err, nil) {
		t.Errorf("test failure: %v", err)
	}
}

func TestParseMultiIntSliceWithDefaultsUnset(t *testing.T) {
	err := (&App{
		Flags: []Flag{
			&IntSliceFlag{Name: "serve", Aliases: []string{"s"}, Value: []int{9, 2}},
		},
		Action: func(ctx *Context) error {
			if !reflect.DeepEqual(ctx.IntSlice("serve"), []int{9, 2}) {
				t.Errorf("main name not set")
			}
			if !reflect.DeepEqual(ctx.IntSlice("s"), []int{9, 2}) {
				t.Errorf("short name not set")
			}
			return nil
		},
	}).Run([]string{"run"})
	if !reflect.DeepEqual(err, nil) {
		t.Errorf("test failure: %v", err)
	}
}

func TestParseMultiIntSliceFromEnv(t *testing.T) {
	defer resetEnv(os.Environ())
	os.Clearenv()
	os.Setenv("APP_INTERVALS", "20,30,40")

	err := (&App{
		Flags: []Flag{
			&IntSliceFlag{Name: "intervals", Aliases: []string{"i"}, Value: []int{}, EnvVars: []string{"APP_INTERVALS"}},
		},
		Action: func(ctx *Context) error {
			if !reflect.DeepEqual(ctx.IntSlice("intervals"), []int{20, 30, 40}) {
				t.Errorf("main name not set from env")
			}
			if !reflect.DeepEqual(ctx.IntSlice("i"), []int{20, 30, 40}) {
				t.Errorf("short name not set from env")
			}
			return nil
		},
	}).Run([]string{"run"})
	if !reflect.DeepEqual(err, nil) {
		t.Errorf("test failure: %v", err)
	}
}

func TestParseMultiIntSliceFromEnvWithDefaults(t *testing.T) {
	defer resetEnv(os.Environ())
	os.Clearenv()
	os.Setenv("APP_INTERVALS", "20,30,40")

	err := (&App{
		Flags: []Flag{
			&IntSliceFlag{Name: "intervals", Aliases: []string{"i"}, Value: []int{1, 2, 5}, EnvVars: []string{"APP_INTERVALS"}},
		},
		Action: func(ctx *Context) error {
			if !reflect.DeepEqual(ctx.IntSlice("intervals"), []int{20, 30, 40}) {
				t.Errorf("main name not set from env")
			}
			if !reflect.DeepEqual(ctx.IntSlice("i"), []int{20, 30, 40}) {
				t.Errorf("short name not set from env")
			}
			return nil
		},
	}).Run([]string{"run"})
	if !reflect.DeepEqual(err, nil) {
		t.Errorf("test failure: %v", err)
	}
}

func TestParseMultiIntSliceFromEnvCascade(t *testing.T) {
	defer resetEnv(os.Environ())
	os.Clearenv()
	os.Setenv("APP_INTERVALS", "20,30,40")

	err := (&App{
		Flags: []Flag{
			&IntSliceFlag{Name: "intervals", Aliases: []string{"i"}, Value: []int{}, EnvVars: []string{"COMPAT_INTERVALS", "APP_INTERVALS"}},
		},
		Action: func(ctx *Context) error {
			if !reflect.DeepEqual(ctx.IntSlice("intervals"), []int{20, 30, 40}) {
				t.Errorf("main name not set from env")
			}
			if !reflect.DeepEqual(ctx.IntSlice("i"), []int{20, 30, 40}) {
				t.Errorf("short name not set from env")
			}
			return nil
		},
	}).Run([]string{"run"})
	if !reflect.DeepEqual(err, nil) {
		t.Errorf("test failure: %v", err)
	}
}

func TestParseMultiInt64Slice(t *testing.T) {
	err := (&App{
		Flags: []Flag{
			&Int64SliceFlag{Name: "serve", Aliases: []string{"s"}, Value: []int64{}},
		},
		Action: func(ctx *Context) error {
			if !reflect.DeepEqual(ctx.Int64Slice("serve"), []int64{10, 17179869184}) {
				t.Errorf("main name not set")
			}
			if !reflect.DeepEqual(ctx.Int64Slice("s"), []int64{10, 17179869184}) {
				t.Errorf("short name not set")
			}
			return nil
		},
	}).Run([]string{"run", "-s", "10", "-s", "17179869184"})
	if !reflect.DeepEqual(err, nil) {
		t.Errorf("test failure: %v", err)
	}
}

func TestParseMultiTimeSlice(t *testing.T) {
	err := (&App{
		Flags: []Flag{
			&TimeSliceFlag{Name: "serve", Aliases: []string{"s"}},
		},
		Action: func(ctx *Context) error {
			if !reflect.DeepEqual(len(ctx.TimeSlice("serve")), 2) {
				t.Errorf("main name not set: %v", ctx.TimeSlice("serve"))
			}
			if !reflect.DeepEqual(len(ctx.TimeSlice("s")), 2) {
				t.Errorf("short name not set: %v", ctx.TimeSlice("s"))
			}
			return nil
		},
	}).Run([]string{"run", "-s", "10", "-s", "17179869184"})
	if !reflect.DeepEqual(err, nil) {
		t.Errorf("test failure: %v", err)
	}
}

func TestParseMultiInt64SliceFromEnv(t *testing.T) {
	defer resetEnv(os.Environ())
	os.Clearenv()
	os.Setenv("APP_INTERVALS", "20,30,17179869184")

	err := (&App{
		Flags: []Flag{
			&Int64SliceFlag{Name: "intervals", Aliases: []string{"i"}, Value: []int64{}, EnvVars: []string{"APP_INTERVALS"}},
		},
		Action: func(ctx *Context) error {
			if !reflect.DeepEqual(ctx.Int64Slice("intervals"), []int64{20, 30, 17179869184}) {
				t.Errorf("main name not set from env")
			}
			if !reflect.DeepEqual(ctx.Int64Slice("i"), []int64{20, 30, 17179869184}) {
				t.Errorf("short name not set from env")
			}
			return nil
		},
	}).Run([]string{"run"})
	if !reflect.DeepEqual(err, nil) {
		t.Errorf("test failure: %v", err)
	}
}

func TestParseMultiInt64SliceFromEnvCascade(t *testing.T) {
	defer resetEnv(os.Environ())
	os.Clearenv()
	os.Setenv("APP_INTERVALS", "20,30,17179869184")

	err := (&App{
		Flags: []Flag{
			&Int64SliceFlag{Name: "intervals", Aliases: []string{"i"}, Value: []int64{}, EnvVars: []string{"COMPAT_INTERVALS", "APP_INTERVALS"}},
		},
		Action: func(ctx *Context) error {
			if !reflect.DeepEqual(ctx.Int64Slice("intervals"), []int64{20, 30, 17179869184}) {
				t.Errorf("main name not set from env")
			}
			if !reflect.DeepEqual(ctx.Int64Slice("i"), []int64{20, 30, 17179869184}) {
				t.Errorf("short name not set from env")
			}
			return nil
		},
	}).Run([]string{"run"})
	if !reflect.DeepEqual(err, nil) {
		t.Errorf("test failure: %v", err)
	}
}

func TestParseMultiFloat64(t *testing.T) {
	err := (&App{
		Flags: []Flag{
			&Float64Flag{Name: "serve", Aliases: []string{"s"}},
		},
		Action: func(ctx *Context) error {
			if ctx.Float64("serve") != 10.2 {
				t.Errorf("main name not set")
			}
			if ctx.Float64("s") != 10.2 {
				t.Errorf("short name not set")
			}
			return nil
		},
	}).Run([]string{"run", "-s", "10.2"})
	if !reflect.DeepEqual(err, nil) {
		t.Errorf("test failure: %v", err)
	}
}

func TestParseDestinationFloat64(t *testing.T) {
	var dest float64
	err := (&App{
		Flags: []Flag{
			&Float64Flag{
				Name:        "dest",
				Destination: &dest,
			},
		},
		Action: func(ctx *Context) error {
			if dest != 10.2 {
				t.Errorf("expected destination Float64 10.2")
			}
			return nil
		},
	}).Run([]string{"run", "--dest", "10.2"})
	if !reflect.DeepEqual(err, nil) {
		t.Errorf("test failure: %v", err)
	}
}

func TestParseMultiFloat64FromEnv(t *testing.T) {
	defer resetEnv(os.Environ())
	os.Clearenv()
	os.Setenv("APP_TIMEOUT_SECONDS", "15.5")
	err := (&App{
		Flags: []Flag{
			&Float64Flag{Name: "timeout", Aliases: []string{"t"}, EnvVars: []string{"APP_TIMEOUT_SECONDS"}},
		},
		Action: func(ctx *Context) error {
			if ctx.Float64("timeout") != 15.5 {
				t.Errorf("main name not set")
			}
			if ctx.Float64("t") != 15.5 {
				t.Errorf("short name not set")
			}
			return nil
		},
	}).Run([]string{"run"})
	if !reflect.DeepEqual(err, nil) {
		t.Errorf("test failure: %v", err)
	}
}

func TestParseMultiFloat64FromEnvCascade(t *testing.T) {
	defer resetEnv(os.Environ())
	os.Clearenv()
	os.Setenv("APP_TIMEOUT_SECONDS", "15.5")
	err := (&App{
		Flags: []Flag{
			&Float64Flag{Name: "timeout", Aliases: []string{"t"}, EnvVars: []string{"COMPAT_TIMEOUT_SECONDS", "APP_TIMEOUT_SECONDS"}},
		},
		Action: func(ctx *Context) error {
			if ctx.Float64("timeout") != 15.5 {
				t.Errorf("main name not set")
			}
			if ctx.Float64("t") != 15.5 {
				t.Errorf("short name not set")
			}
			return nil
		},
	}).Run([]string{"run"})
	if !reflect.DeepEqual(err, nil) {
		t.Errorf("test failure: %v", err)
	}
}

func TestParseMultiFloat64SliceFromEnv(t *testing.T) {
	defer resetEnv(os.Environ())
	os.Clearenv()
	os.Setenv("APP_INTERVALS", "0.1,-10.5")

	err := (&App{
		Flags: []Flag{
			&Float64SliceFlag{Name: "intervals", Aliases: []string{"i"}, Value: []float64{}, EnvVars: []string{"APP_INTERVALS"}},
		},
		Action: func(ctx *Context) error {
			if !reflect.DeepEqual(ctx.Float64Slice("intervals"), []float64{0.1, -10.5}) {
				t.Errorf("main name not set from env")
			}
			if !reflect.DeepEqual(ctx.Float64Slice("i"), []float64{0.1, -10.5}) {
				t.Errorf("short name not set from env")
			}
			return nil
		},
	}).Run([]string{"run"})
	if !reflect.DeepEqual(err, nil) {
		t.Errorf("test failure: %v", err)
	}
}

func TestParseMultiFloat64SliceFromEnvCascade(t *testing.T) {
	defer resetEnv(os.Environ())
	os.Clearenv()
	os.Setenv("APP_INTERVALS", "0.1234,-10.5")

	err := (&App{
		Flags: []Flag{
			&Float64SliceFlag{Name: "intervals", Aliases: []string{"i"}, Value: []float64{}, EnvVars: []string{"COMPAT_INTERVALS", "APP_INTERVALS"}},
		},
		Action: func(ctx *Context) error {
			if !reflect.DeepEqual(ctx.Float64Slice("intervals"), []float64{0.1234, -10.5}) {
				t.Errorf("main name not set from env")
			}
			if !reflect.DeepEqual(ctx.Float64Slice("i"), []float64{0.1234, -10.5}) {
				t.Errorf("short name not set from env")
			}
			return nil
		},
	}).Run([]string{"run"})
	if !reflect.DeepEqual(err, nil) {
		t.Errorf("test failure: %v", err)
	}
}

func TestParseMultiBool(t *testing.T) {
	err := (&App{
		Flags: []Flag{
			&BoolFlag{Name: "serve", Aliases: []string{"s"}},
		},
		Action: func(ctx *Context) error {
			if ctx.Bool("serve") != true {
				t.Errorf("main name not set")
			}
			if ctx.Bool("s") != true {
				t.Errorf("short name not set")
			}
			return nil
		},
	}).Run([]string{"run", "--serve"})
	if !reflect.DeepEqual(err, nil) {
		t.Errorf("test failure: %v", err)
	}
}

func TestParseBoolShortOptionHandle(t *testing.T) {
	err := (&App{
		Commands: []*Command{
			{
				Name:                   "foobar",
				UseShortOptionHandling: true,
				Action: func(ctx *Context) error {
					if ctx.Bool("serve") != true {
						t.Errorf("main name not set")
					}
					if ctx.Bool("option") != true {
						t.Errorf("short name not set")
					}
					return nil
				},
				Flags: []Flag{
					&BoolFlag{Name: "serve", Aliases: []string{"s"}},
					&BoolFlag{Name: "option", Aliases: []string{"o"}},
				},
			},
		},
	}).Run([]string{"run", "foobar", "-so"})
	if !reflect.DeepEqual(err, nil) {
		t.Errorf("test failure: %v", err)
	}
}

func TestParseDestinationBool(t *testing.T) {
	var dest bool
	err := (&App{
		Flags: []Flag{
			&BoolFlag{
				Name:        "dest",
				Destination: &dest,
			},
		},
		Action: func(ctx *Context) error {
			if dest != true {
				t.Errorf("expected destination Bool true")
			}
			return nil
		},
	}).Run([]string{"run", "--dest"})
	if !reflect.DeepEqual(err, nil) {
		t.Errorf("test failure: %v", err)
	}
}

func TestParseMultiBoolFromEnv(t *testing.T) {
	defer resetEnv(os.Environ())
	os.Clearenv()
	os.Setenv("APP_DEBUG", "1")
	err := (&App{
		Flags: []Flag{
			&BoolFlag{Name: "debug", Aliases: []string{"d"}, EnvVars: []string{"APP_DEBUG"}},
		},
		Action: func(ctx *Context) error {
			if ctx.Bool("debug") != true {
				t.Errorf("main name not set from env")
			}
			if ctx.Bool("d") != true {
				t.Errorf("short name not set from env")
			}
			return nil
		},
	}).Run([]string{"run"})
	if !reflect.DeepEqual(err, nil) {
		t.Errorf("test failure: %v", err)
	}
}

func TestParseMultiBoolFromEnvCascade(t *testing.T) {
	defer resetEnv(os.Environ())
	os.Clearenv()
	os.Setenv("APP_DEBUG", "1")
	err := (&App{
		Flags: []Flag{
			&BoolFlag{Name: "debug", Aliases: []string{"d"}, EnvVars: []string{"COMPAT_DEBUG", "APP_DEBUG"}},
		},
		Action: func(ctx *Context) error {
			if ctx.Bool("debug") != true {
				t.Errorf("main name not set from env")
			}
			if ctx.Bool("d") != true {
				t.Errorf("short name not set from env")
			}
			return nil
		},
	}).Run([]string{"run"})
	if !reflect.DeepEqual(err, nil) {
		t.Errorf("test failure: %v", err)
	}
}

func TestParseBoolFromEnv(t *testing.T) {
	var boolFlagTests = []struct {
		input  string
		output bool
	}{
		{"", false},
		{"1", true},
		{"false", false},
		{"true", true},
	}

	for _, test := range boolFlagTests {
		defer resetEnv(os.Environ())
		os.Clearenv()
		os.Setenv("DEBUG", test.input)
		err := (&App{
			Flags: []Flag{
				&BoolFlag{Name: "debug", Aliases: []string{"d"}, EnvVars: []string{"DEBUG"}},
			},
			Action: func(ctx *Context) error {
				if ctx.Bool("debug") != test.output {
					t.Errorf("expected %+v to be parsed as %+v, instead was %+v", test.input, test.output, ctx.Bool("debug"))
				}
				if ctx.Bool("d") != test.output {
					t.Errorf("expected %+v to be parsed as %+v, instead was %+v", test.input, test.output, ctx.Bool("d"))
				}
				return nil
			},
		}).Run([]string{"run"})
		if !reflect.DeepEqual(err, nil) {
			t.Errorf("test failure: %v", err)
		}
	}
}

func TestParseMultiBoolT(t *testing.T) {
	err := (&App{
		Flags: []Flag{
			&BoolFlag{Name: "implode", Aliases: []string{"i"}, Value: true},
		},
		Action: func(ctx *Context) error {
			if ctx.Bool("implode") {
				t.Errorf("main name not set")
			}
			if ctx.Bool("i") {
				t.Errorf("short name not set")
			}
			return nil
		},
	}).Run([]string{"run", "--implode=false"})
	if !reflect.DeepEqual(err, nil) {
		t.Errorf("test failure: %v", err)
	}
}

type Parser [2]string

func (p *Parser) Set(value string) error {
	parts := strings.Split(value, ",")
	if len(parts) != 2 {
		return fmt.Errorf("invalid format")
	}

	(*p)[0] = parts[0]
	(*p)[1] = parts[1]

	return nil
}

func (p *Parser) String() string {
	return fmt.Sprintf("%s,%s", p[0], p[1])
}

func (p *Parser) Get() interface{} {
	return p
}

func TestParseGeneric(t *testing.T) {
	err := (&App{
		Flags: []Flag{
			&GenericFlag{Name: "serve", Aliases: []string{"s"}, Value: &Parser{}},
		},
		Action: func(ctx *Context) error {
			if !reflect.DeepEqual(ctx.Generic("serve"), &Parser{"10", "20"}) {
				t.Errorf("main name not set")
			}
			if !reflect.DeepEqual(ctx.Generic("s"), &Parser{"10", "20"}) {
				t.Errorf("short name not set")
			}
			return nil
		},
	}).Run([]string{"run", "-s", "10,20"})
	if !reflect.DeepEqual(err, nil) {
		t.Errorf("test failure: %v", err)
	}
}

func TestParseGenericNoFlags(t *testing.T) {
	err := (&App{
		Flags: []Flag{
			&GenericFlag{Name: "serve", Aliases: []string{"s"}, Value: &Parser{"hello", "world"}},
		},
		Action: func(ctx *Context) error {
			if !reflect.DeepEqual(ctx.Generic("serve"), &Parser{"hello", "world"}) {
				t.Errorf("main name not set")
			}
			if !reflect.DeepEqual(ctx.Generic("s"), &Parser{"hello", "world"}) {
				t.Errorf("short name not set")
			}
			return nil
		},
	}).Run([]string{"run"})
	if !reflect.DeepEqual(err, nil) {
		t.Errorf("test failure: %v", err)
	}
}

func TestParseGenericDestination(t *testing.T) {
	val := &Parser{"hello", "world"}
	dest := &Parser{}
	err := (&App{
		Flags: []Flag{
			&GenericFlag{Name: "serve", Aliases: []string{"s"}, Value: val, Destination: dest},
		},
		Action: func(ctx *Context) error {
			if !reflect.DeepEqual(dest, &Parser{"10", "20"}) {
				t.Errorf("destination not set")
			}
			if !reflect.DeepEqual(val, &Parser{"hello", "world"}) {
				t.Errorf("value was set")
			}
			return nil
		},
	}).Run([]string{"run", "-s", "10,20"})
	if !reflect.DeepEqual(err, nil) {
		t.Errorf("test failure: %v", err)
	}
}

func TestParseGenericDestinationNoValue(t *testing.T) {
	dest := &Parser{"a", "ok"}
	err := (&App{
		Flags: []Flag{
			&GenericFlag{Name: "serve", Aliases: []string{"s"}, Destination: dest},
		},
		Action: func(ctx *Context) error {
			if !reflect.DeepEqual(dest, &Parser{"10", "20"}) {
				t.Errorf("destination not set")
			}
			return nil
		},
	}).Run([]string{"run", "-s", "10,20"})
	if !reflect.DeepEqual(err, nil) {
		t.Errorf("test failure: %v", err)
	}
}

func TestParseGenericDestinationNoValueNoFlags(t *testing.T) {
	dest := &Parser{"a", "ok"}
	err := (&App{
		Flags: []Flag{
			&GenericFlag{Name: "serve", Aliases: []string{"s"}, Destination: dest},
		},
		Action: func(ctx *Context) error {
			if !reflect.DeepEqual(dest, &Parser{}) {
				t.Errorf("destination not set")
			}
			return nil
		},
	}).Run([]string{"run"})
	if !reflect.DeepEqual(err, nil) {
		t.Errorf("test failure: %v", err)
	}
}

func TestParseStringSliceDestinationNoValueNoFlags(t *testing.T) {
	dest := []string{"a", "ok"}
	err := (&App{
		Flags: []Flag{
			&StringSliceFlag{Name: "serve", Aliases: []string{"s"}, Destination: &dest},
		},
		Action: func(ctx *Context) error {
			if len(dest) != 0 {
				t.Errorf("destination not set")
			}
			return nil
		},
	}).Run([]string{"run"})
	if !reflect.DeepEqual(err, nil) {
		t.Errorf("test failure: %v", err)
	}
}

func TestParseGenericFromEnv(t *testing.T) {
	defer resetEnv(os.Environ())
	os.Clearenv()
	os.Setenv("APP_SERVE", "20,30")
	err := (&App{
		Flags: []Flag{
			&GenericFlag{
				Name:    "serve",
				Aliases: []string{"s"},
				Value:   &Parser{},
				EnvVars: []string{"APP_SERVE"},
			},
		},
		Action: func(ctx *Context) error {
			if !reflect.DeepEqual(ctx.Generic("serve"), &Parser{"20", "30"}) {
				t.Errorf("main name not set from env")
			}
			if !reflect.DeepEqual(ctx.Generic("s"), &Parser{"20", "30"}) {
				t.Errorf("short name not set from env")
			}
			return nil
		},
	}).Run([]string{"run"})
	if !reflect.DeepEqual(err, nil) {
		t.Errorf("test failure: %v", err)
	}
}

func TestParseGenericFromEnvCascade(t *testing.T) {
	defer resetEnv(os.Environ())
	os.Clearenv()
	os.Setenv("APP_FOO", "99,2000")
	err := (&App{
		Flags: []Flag{
			&GenericFlag{
				Name:    "foos",
				Value:   &Parser{},
				EnvVars: []string{"COMPAT_FOO", "APP_FOO"},
			},
		},
		Action: func(ctx *Context) error {
			if !reflect.DeepEqual(ctx.Generic("foos"), &Parser{"99", "2000"}) {
				t.Errorf("value not set from env")
			}
			return nil
		},
	}).Run([]string{"run"})
	if !reflect.DeepEqual(err, nil) {
		t.Errorf("test failure: %v", err)
	}
}

func TestFromEnvWithAlias(t *testing.T) {
	defer resetEnv(os.Environ())
	os.Clearenv()
	os.Setenv("APP_IMPLODE", "true")
	err := (&App{
		Flags: []Flag{
			&BoolFlag{
				Name:    "implode",
				Aliases: []string{"i"},
				Value:   true,
				EnvVars: []string{"APP_IMPLODE"},
			},
		},
		Action: func(ctx *Context) error {
			if !reflect.DeepEqual(ctx.Bool("implode"), false) {
				t.Errorf("alias with env failure")
			}
			return nil
		},
	}).Run([]string{"run", "-i=false"})
	if !reflect.DeepEqual(err, nil) {
		t.Errorf("test failure: %v", err)
	}
}

func TestFlagFromFile(t *testing.T) {
	temp, err := ioutil.TempFile("", "urfave_cli_test")
	if err != nil {
		t.Error(err)
		return
	}

	defer resetEnv(os.Environ())
	os.Clearenv()
	os.Setenv("APP_FOO", "123")

	io.WriteString(temp, "abc")
	temp.Close()
	defer func() {
		os.Remove(temp.Name())
	}()

	var filePathTests = []struct {
		path     string
		name     []string
		expected string
	}{
		{"file-does-not-exist", []string{"APP_BAR"}, ""},
		{"file-does-not-exist", []string{"APP_FOO"}, "123"},
		{"file-does-not-exist", []string{"APP_FOO", "APP_BAR"}, "123"},
		{temp.Name(), []string{"APP_FOO"}, "123"},
		{temp.Name(), []string{"APP_BAR"}, "abc"},
	}

	for _, filePathTest := range filePathTests {
		got, _ := flagFromEnvOrFile(filePathTest.name, filePathTest.path)
		if want := filePathTest.expected; got != want {
			t.Errorf("Did not expect %v - Want %v", got, want)
		}
	}
}
