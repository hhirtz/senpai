package irc

import (
	"bufio"
	"fmt"
	"net"
)

const chanCapacity = 64

func ChanInOut(conn net.Conn) (in <-chan Message, out chan<- Message) {
	in_ := make(chan Message, chanCapacity)
	out_ := make(chan Message, chanCapacity)

	go func() {
		r := bufio.NewScanner(conn)
		for r.Scan() {
			line := r.Text()
			msg, err := ParseMessage(line)
			if err != nil {
				continue
			}
			in_ <- msg
		}
		close(in_)
	}()

	go func() {
		for msg := range out_ {
			// TODO send messages by batches
			_, err := fmt.Fprintf(conn, "%s\r\n", msg.String())
			if err != nil {
				break
			}
		}
		_ = conn.Close()
	}()

	return in_, out_
}
