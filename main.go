package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"os/exec"
)

func main() {
	DELIM := ">"
	PORT := "6060"

	sh, err := shellStart()
	errCheck(err, "shell running")
	fmt.Println("Shell running as process:", sh.cmd.Process.Pid)

	conn, err := netConn(PORT)
	errCheck(err, "netconn")

	output := conn
	input := conn

	go startReaders(sh, output, DELIM)

	scanner := bufio.NewScanner(input)

	for {
		output.Write([]byte(DELIM))

		scanner.Scan()
		io.WriteString(sh.stdin, scanner.Text()+"\n")
	}
}

func netConn(PORT string) (net.Conn, error) {
	address := ":" + PORT
	listener, err := net.Listen("tcp", address)
	errCheck(err, "listener")

	conn, err := listener.Accept()
	errCheck(err, "accept")

	return conn, nil
}

func errCheck(err error, msg string) {
	if err != nil {
		log.Fatal(msg, ":", err)
	}
}

func startReaders(sh *Shell, out net.Conn, DELIM string) {
	go readLoop(sh.stdout, out, DELIM)
	go readLoop(sh.stderr, out, DELIM)
}

func readLoop(reader io.ReadCloser, out net.Conn, DELIM string) {
	for {
		bString := make([]byte, 4096) // read 1024 bytes at a time into a string

		_, err := reader.Read(bString)
		errCheck(err, "reading stdout")
		output := "\r" + string(bString[:])

		out.Write([]byte(output))
		out.Write([]byte(DELIM))
	}
}

func shellStart() (*Shell, error) {
	shell := exec.Command("sh")

	stdin, err := shell.StdinPipe()
	errCheck(err, "stdin set")

	stdout, err := shell.StdoutPipe()
	errCheck(err, "stdout set")

	stderr, err := shell.StderrPipe()
	errCheck(err, "stderr set")

	e := shell.Start()
	errCheck(e, "shell start")

	sh := &Shell{
		cmd:    shell,
		stdin:  stdin,
		stdout: stdout,
		stderr: stderr,
	}
	return sh, nil
}

type Shell struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr io.ReadCloser
}
