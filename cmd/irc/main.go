package main

import (
	"flag"
	"log"
	"math/rand"
	"os"
	"time"

	"git.sr.ht/~taiite/senpai"
	"github.com/gdamore/tcell"
)

func init() {
	rand.Seed(time.Now().Unix())
}

func main() {
	tcell.SetEncodingFallback(tcell.EncodingFallbackASCII)

	var configPath string
	flag.StringVar(&configPath, "config", "", "path to the configuration file")
	flag.Parse()

	if configPath == "" {
		configDir, err := os.UserConfigDir()
		if err != nil {
			log.Panicln(err)
		}
		configPath = configDir + "/senpai/senpai.yaml"
	}

	cfg, err := senpai.LoadConfigFile(configPath)
	if err != nil {
		log.Panicln(err)
	}

	app, err := senpai.NewApp(cfg)
	if err != nil {
		log.Panicln(err)
	}
	defer app.Close()

	app.Run()
}
