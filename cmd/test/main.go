package main

import (
	"bufio"
	"crypto/tls"
	"flag"
	"log"
	"net"
	"os"

	"git.sr.ht/~taiite/senpai"
	"git.sr.ht/~taiite/senpai/irc"
)

func main() {
	var configPath string
	var address string
	var nick string
	var password string
	var useTLS bool
	flag.StringVar(&configPath, "config", "", "path to the configuration file")
	flag.StringVar(&address, "address", "", "server address")
	flag.StringVar(&nick, "nick", "senpai", "IRC nick/user to use")
	flag.StringVar(&password, "password", "", "SASL password to use")
	flag.BoolVar(&useTLS, "tls", false, "whether to use tls")
	flag.Parse()

	if address == "" {
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

		address = cfg.Addr
		nick = cfg.Nick
		if cfg.Password != nil {
			password = *cfg.Password
		}
	}

	log.Printf("Connecting to %s...\n", address)

	var conn net.Conn
	var err error
	if useTLS {
		conn, err = tls.Dial("tcp", address, nil)
	} else {
		conn, err = net.Dial("tcp", address)
	}
	if err != nil {
		log.Panicf("Failed to connect to %s: %v", address, err)
	}
	defer conn.Close()

	log.Printf("Connected. Registration in progress...\n")

	var auth irc.SASLPlain
	if password != "" {
		auth = irc.SASLPlain{Username: nick, Password: password}
	}
	cli, err := irc.NewSession(conn, irc.SessionParams{
		Nickname: nick,
		Username: nick,
		RealName: nick,
		Auth:     &auth,
		Debug:    true,
	})
	if err != nil {
		log.Panicf("Failed to connect to %s: %v", address, err)
	}
	defer cli.Stop()

	go func() {
		r := bufio.NewScanner(os.Stdin)
		for r.Scan() {
			line := r.Text()
			cli.SendRaw(line)
		}
		cli.Stop()
	}()

	for ev := range cli.Poll() {
		switch ev := ev.(type) {
		case irc.RawMessageEvent:
			if ev.Outgoing {
				log.Printf("C  > S: %s\n", ev.Message)
			} else {
				log.Printf("C <  S: %s\n", ev.Message)
			}
		case error:
			log.Panicln(ev)
		default:
			log.Printf("Event: %T%+v", ev, ev)
		}
	}
	log.Println("Disconnected")
}
