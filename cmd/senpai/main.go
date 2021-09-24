package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"path"
	"strings"
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
		fmt.Fprintf(os.Stderr, "failed to load the required configuration file at %q: %s\n", configPath, err)
		os.Exit(1)
	}

	cfg.Debug = cfg.Debug || debug

	lastBuffer := getLastBuffer()

	app, err := senpai.NewApp(cfg, lastBuffer)
	if err != nil {
		panic(err)
	}

	app.Run()
	app.Close()

	// Write last buffer on close
	lastBufferPath := getLastBufferPath()
	err = os.WriteFile(lastBufferPath, []byte(app.CurrentBuffer()), 0666)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to write last buffer at %q: %s\n", lastBufferPath, err)
	}
}

func getLastBufferPath() string {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		panic(err)
	}
	cachePath := path.Join(cacheDir, "senpai")
	err = os.MkdirAll(cachePath, 0755)
	if err != nil {
		panic(err)
	}

	lastBufferPath := path.Join(cachePath, "lastbuffer.txt")
	return lastBufferPath
}

func getLastBuffer() string {
	buf, err := ioutil.ReadFile(getLastBufferPath())
	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(buf))
}
