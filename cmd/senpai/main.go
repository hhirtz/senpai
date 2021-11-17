package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"os/signal"
	"path"
	"strings"
	"syscall"
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

	app, err := senpai.NewApp(cfg)
	if err != nil {
		panic(err)
	}

	lastNetID, lastBuffer := getLastBuffer()
	app.SwitchToBuffer(lastNetID, lastBuffer)

	// Write last buffer on close
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	go func() {
		for sig := range sigCh {
			switch sig {
			case syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM:
				app.Close()
				writeLastBuffer(app)
			}
		}
	}()

	app.Run()
	app.Close()
	writeLastBuffer(app)
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

func getLastBuffer() (netID, buffer string) {
	buf, err := ioutil.ReadFile(getLastBufferPath())
	if err != nil {
		return "", ""
	}

	fields := strings.SplitN(string(buf), " ", 2)
	if len(fields) < 2 {
		return "", ""
	}

	return fields[0], fields[1]
}

func writeLastBuffer(app *senpai.App) {
	lastBufferPath := getLastBufferPath()
	lastNetID, lastBuffer := app.CurrentBuffer()
	err := os.WriteFile(lastBufferPath, []byte(fmt.Sprintf("%s %s", lastNetID, lastBuffer)), 0666)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to write last buffer at %q: %s\n", lastBufferPath, err)
	}
}
