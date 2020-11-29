package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"net"
	"os"

	"git.sr.ht/~taiite/senpai"
	"git.sr.ht/~taiite/senpai/irc"
	"golang.org/x/term"
)

var (
	configPath string
	address    string
	nick       string
	password   string
	useTLS     bool
)

func main() {
	parseFlags()

	oldState, err := term.MakeRaw(0)
	if err != nil {
		panic(err)
	}
	defer term.Restore(0, oldState)

	screen := struct {
		io.Reader
		io.Writer
	}{os.Stdin, os.Stdout}
	t := term.NewTerminal(screen, "> ")

	fmt.Fprintf(t, "Connecting to %s...\n", address)

	var conn net.Conn
	if useTLS {
		conn, err = tls.Dial("tcp", address, nil)
	} else {
		conn, err = net.Dial("tcp", address)
	}
	if err != nil {
		panic(fmt.Sprintf("Failed to connect to %s: %v\n", address, err))
	}
	defer conn.Close()

	fmt.Fprintf(t, "Connected. Registration in progress...\n")

	var auth irc.SASLClient
	if password != "" {
		auth = &irc.SASLPlain{Username: nick, Password: password}
	}
	cli, err := irc.NewSession(conn, irc.SessionParams{
		Nickname: nick,
		Username: nick,
		RealName: nick,
		Auth:     auth,
		Debug:    true,
	})
	if err != nil {
		panic(fmt.Sprintf("Failed to connect to %s: %v", address, err))
	}
	defer cli.Stop()

	go func() {
		for {
			line, err := t.ReadLine()
			if err != nil {
				break
			}
			cli.SendRaw(line)
		}
		cli.Stop()
	}()

	for ev := range cli.Poll() {
		switch ev := ev.(type) {
		case irc.RawMessageEvent:
			if ev.Outgoing {
				fmt.Fprintf(t, "C  > S: %s\n", ev.Message)
			} else {
				fmt.Fprintf(t, "C <  S: %s\n", ev.Message)
			}
		case error:
			panic(ev)
		default:
			fmt.Fprintf(t, "=EVENT: %T%+v\n", ev, ev)
		}
	}
	t.SetPrompt("")
	fmt.Fprintln(t, "Disconnected")
}

func parseFlags() {
	flag.StringVar(&configPath, "config", "", "path to the configuration file")
	flag.StringVar(&address, "address", "", "server address")
	flag.StringVar(&nick, "nick", "senpai", "IRC nick/user to use")
	flag.StringVar(&password, "password", "", "SASL password to use")
	flag.BoolVar(&useTLS, "tls", false, "use tls")
	flag.Parse()

	if address == "" {
		if configPath == "" {
			configDir, err := os.UserConfigDir()
			if err != nil {
				panic(err)
			}
			configPath = configDir + "/senpai/senpai.yaml"
		}

		cfg, err := senpai.LoadConfigFile(configPath)
		if err != nil {
			panic(err)
		}

		address = cfg.Addr
		nick = cfg.Nick
		if cfg.Password != nil {
			password = *cfg.Password
		}
		useTLS = true
	}
}
