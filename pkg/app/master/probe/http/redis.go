package http

import (
	"bufio"
	"fmt"
	"net"
)

const redisPingReq = "*1\r\n$4\r\nPING\r\n"

func redisPing(targetHost string, port string) (string, error) {
	addr := fmt.Sprintf("%s:%s", targetHost, port)
	c, err := net.Dial("tcp", addr)
	if err != nil {
		return "", err
	}
	defer c.Close()

	if _, err := fmt.Fprintf(c, redisPingReq); err != nil {
		return "", err
	}

	r := bufio.NewReader(c)
	output, err := r.ReadString('\n')
	if err != nil {
		return "", err
	}

	return output, nil
}
