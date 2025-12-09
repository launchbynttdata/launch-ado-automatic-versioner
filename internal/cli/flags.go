package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/pflag"

	"github.com/launchbynttdata/launch-ado-automatic-versioner/internal/config"
)

type flagBase struct {
	fs      *pflag.FlagSet
	setting string
	name    string
	envKey  string
}

func newFlagBase(fs *pflag.FlagSet, setting, name, envKey string) flagBase {
	return flagBase{fs: fs, setting: setting, name: name, envKey: envKey}
}

func (b flagBase) changed() bool {
	if b.fs == nil || b.name == "" {
		return false
	}
	return b.fs.Changed(b.name)
}

func describeUsage(usage, envKey string) string {
	trimmed := strings.TrimSpace(usage)
	if envKey == "" {
		return trimmed
	}
	if trimmed == "" {
		return fmt.Sprintf("env: %s", envKey)
	}
	return fmt.Sprintf("%s (env: %s)", trimmed, envKey)
}

type stringFlag struct {
	base       flagBase
	defaultVal string
	value      string
	isSecret   bool
}

func bindStringFlag(fs *pflag.FlagSet, setting, name, short, envKey, defaultVal, usage string) *stringFlag {
	f := &stringFlag{
		base:       newFlagBase(fs, setting, name, envKey),
		defaultVal: defaultVal,
		value:      defaultVal,
		isSecret:   false,
	}
	if fs == nil {
		return f
	}
	if short != "" {
		fs.StringVarP(&f.value, name, short, defaultVal, describeUsage(usage, envKey))
	} else {
		fs.StringVar(&f.value, name, defaultVal, describeUsage(usage, envKey))
	}
	return f
}

func bindSecretFlag(fs *pflag.FlagSet, setting, name, short, envKey, defaultVal, usage string) *stringFlag {
	f := &stringFlag{
		base:       newFlagBase(fs, setting, name, envKey),
		defaultVal: defaultVal,
		value:      defaultVal,
		isSecret:   true,
	}
	if fs == nil {
		return f
	}
	if short != "" {
		fs.StringVarP(&f.value, name, short, defaultVal, describeUsage(usage, envKey))
	} else {
		fs.StringVar(&f.value, name, defaultVal, describeUsage(usage, envKey))
	}
	return f
}

func (f *stringFlag) Value(resolver config.Resolver) string {
	cliVal := strings.TrimSpace(f.value)
	if f.isSecret {
		return resolver.Secret(f.base.setting, f.base.envKey, cliVal, f.base.changed(), f.defaultVal)
	}
	return resolver.String(f.base.setting, f.base.envKey, cliVal, f.base.changed(), f.defaultVal)
}

type boolFlag struct {
	base       flagBase
	defaultVal bool
	value      bool
}

func bindBoolFlag(fs *pflag.FlagSet, setting, name, short, envKey string, defaultVal bool, usage string) *boolFlag {
	f := &boolFlag{
		base:       newFlagBase(fs, setting, name, envKey),
		defaultVal: defaultVal,
		value:      defaultVal,
	}
	if fs == nil {
		return f
	}
	if short != "" {
		fs.BoolVarP(&f.value, name, short, defaultVal, describeUsage(usage, envKey))
	} else {
		fs.BoolVar(&f.value, name, defaultVal, describeUsage(usage, envKey))
	}
	return f
}

func (f *boolFlag) Value(resolver config.Resolver) (bool, error) {
	return resolver.Bool(f.base.setting, f.base.envKey, f.value, f.base.changed(), f.defaultVal)
}

type intFlag struct {
	base       flagBase
	defaultVal int
	value      int
}

func bindIntFlag(fs *pflag.FlagSet, setting, name, short, envKey string, defaultVal int, usage string) *intFlag {
	f := &intFlag{
		base:       newFlagBase(fs, setting, name, envKey),
		defaultVal: defaultVal,
		value:      defaultVal,
	}
	if fs == nil {
		return f
	}
	if short != "" {
		fs.IntVarP(&f.value, name, short, defaultVal, describeUsage(usage, envKey))
	} else {
		fs.IntVar(&f.value, name, defaultVal, describeUsage(usage, envKey))
	}
	return f
}

func (f *intFlag) Value(resolver config.Resolver) (int, error) {
	return resolver.Int(f.base.setting, f.base.envKey, f.value, f.base.changed(), f.defaultVal)
}

type stringSliceFlag struct {
	base       flagBase
	defaultVal []string
	value      []string
}

func bindStringSliceFlag(fs *pflag.FlagSet, setting, name, short, envKey string, defaultVal []string, usage string) *stringSliceFlag {
	f := &stringSliceFlag{
		base:       newFlagBase(fs, setting, name, envKey),
		defaultVal: append([]string(nil), defaultVal...),
		value:      append([]string(nil), defaultVal...),
	}
	if fs == nil {
		return f
	}
	if short != "" {
		fs.StringSliceVarP(&f.value, name, short, defaultVal, describeUsage(usage, envKey))
	} else {
		fs.StringSliceVar(&f.value, name, defaultVal, describeUsage(usage, envKey))
	}
	return f
}

func (f *stringSliceFlag) Value(resolver config.Resolver) []string {
	cliVal := sanitizeSliceValues(f.value)
	return resolver.StringSlice(f.base.setting, f.base.envKey, cliVal, f.base.changed(), f.defaultVal)
}

func sanitizeSliceValues(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	cleaned := make([]string, 0, len(values))
	for _, v := range values {
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			continue
		}
		cleaned = append(cleaned, trimmed)
	}
	if len(cleaned) == 0 {
		return nil
	}
	return cleaned
}
