package main

import (
	"crypto/tls"
	"git.sr.ht/~taiite/senpai/irc"
	"log"
	"os"
)

func main() {
	addr := os.Args[1]

	log.Printf("Connecting to %s...\n", addr)

	conn, err := tls.Dial("tcp", addr, nil)
	if err != nil {
		log.Panicf("Failed to connect to %s: %v", addr, err)
	}
	defer conn.Close()

	log.Printf("Connected. Registration in progress...\n")

	cli, err := irc.NewSession(conn, irc.SessionParams{
		Nickname: "senpai",
		Username: "senpai",
		RealName: "senpai Ier",
		Auth:     &irc.SASLPlain{Username: os.Args[2], Password: os.Args[3]},
	})
	if err != nil {
		log.Panicf("Failed to register to %s: %v", addr, err)
	}

	for {
		ev := <-cli.Poll()
		switch ev := ev.(type) {
		case error:
			log.Panicln(ev)
		}
	}
}
