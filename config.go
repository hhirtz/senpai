package senpai

import (
	"gopkg.in/yaml.v2"
	"io/ioutil"
)

type Config struct {
	Addr string
	User string
	Password string
}

func ParseConfig(buf []byte) (cfg Config, err error) {
	err = yaml.Unmarshal(buf, &cfg)
	return
}

func LoadConfigFile(filename string) (cfg Config, err error) {
	var buf []byte
	
	buf, err = ioutil.ReadFile(filename)
	if err != nil {
		return 
	}
	
	cfg, err = ParseConfig(buf)

	return
}
