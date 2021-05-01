package main

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"path"
	"time"

	"git.sr.ht/~taiite/senpai"
	"github.com/gdamore/tcell/v2"
)

func init() {
	rand.Seed(time.Now().Unix())
}

func main() {
	tcell.SetEncodingFallback(tcell.EncodingFallbackASCII)

	var configPath string
	var debug bool
	flag.StringVar(&configPath, "config", "", "path to the configuration file")
	flag.BoolVar(&debug, "debug", false, "show raw protocol data in the home buffer")
	flag.Parse()

	if configPath == "" {
		configDir, err := os.UserConfigDir()
		if err != nil {
			panic(err)
		}
		configPath = path.Join(configDir, "senpai", "senpai.yaml")
	}

	cfg, err := senpai.LoadConfigFile(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load the required configuraiton file at %q: %s\n", configPath, err)
		os.Exit(1)
	}

	cfg.Debug = cfg.Debug || debug

	app, err := senpai.NewApp(cfg)
	if err != nil {
		panic(err)
	}
	defer app.Close()

	app.Run()
}
