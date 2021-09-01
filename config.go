package senpai

import (
	"errors"
	"fmt"
	"io/ioutil"
	"strconv"
	"strings"

	"github.com/gdamore/tcell/v2"

	"gopkg.in/yaml.v2"
)

type Color tcell.Color

func (c *Color) UnmarshalText(data []byte) error {
	s := string(data)

	if strings.HasPrefix(s, "#") {
		hex, err := strconv.ParseInt(s[1:], 16, 32)
		if err != nil {
			return err
		}

		*c = Color(tcell.NewHexColor(int32(hex)))
		return nil
	}

	code, err := strconv.Atoi(s)
	if err != nil {
		return err
	}

	if code == -1 {
		*c = Color(tcell.ColorDefault)
		return nil
	}

	if code < 0 || code > 255 {
		return fmt.Errorf("color code must be between 0-255. If you meant to use true colors, use #aabbcc notation")
	}

	*c = Color(tcell.PaletteColor(code))

	return nil
}

type Config struct {
	Addr     string
	Nick     string
	Real     string
	User     string
	Password *string
	NoTLS    bool `yaml:"no-tls"`
	Channels []string

	NoTypings bool `yaml:"no-typings"`
	Mouse     *bool

	Highlights     []string
	OnHighlight    string `yaml:"on-highlight"`
	NickColWidth   int    `yaml:"nick-column-width"`
	ChanColWidth   int    `yaml:"chan-column-width"`
	MemberColWidth int    `yaml:"member-column-width"`

	Colors struct {
		Prompt Color
	}

	Debug bool
}

func ParseConfig(buf []byte) (cfg Config, err error) {
	err = yaml.Unmarshal(buf, &cfg)
	if err != nil {
		return cfg, err
	}
	if cfg.Addr == "" {
		return cfg, errors.New("addr is required")
	}
	if cfg.Nick == "" {
		return cfg, errors.New("nick is required")
	}
	if cfg.User == "" {
		cfg.User = cfg.Nick
	}
	if cfg.Real == "" {
		cfg.Real = cfg.Nick
	}
	if cfg.NickColWidth <= 0 {
		cfg.NickColWidth = 16
	}
	if cfg.ChanColWidth < 0 {
		cfg.ChanColWidth = 0
	}
	if cfg.MemberColWidth <= 0 {
		cfg.MemberColWidth = 16
	}
	return
}

func LoadConfigFile(filename string) (cfg Config, err error) {
	var buf []byte

	buf, err = ioutil.ReadFile(filename)
	if err != nil {
		return cfg, fmt.Errorf("failed to read the file: %s", err)
	}

	cfg, err = ParseConfig(buf)
	if err != nil {
		return cfg, fmt.Errorf("invalid content found in the file: %s", err)
	}
	return
}
