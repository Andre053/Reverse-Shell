package main

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	mathrand "math/rand"
	"net"
	"os/exec"
)

func main() {
	DELIM := ">"
	PORT := "6060"
	users := map[string]struct{}{} // likely need a syncmap to avoid race conditions

	// example public key PEM, replace with your own
	PUB_KEY_PEM_1024 := `
-----BEGIN PUBLIC KEY-----
MIGfMA0GCSqGSIb3DQEBAQUAA4GNADCBiQKBgQC86B2VU6LH4Nb8fkbgDtVFJwTZ
09Z58EfoakgwrP1uZTPaqSIdy7wfKYCM5ixFfWAY9gSrNg9kHFEvsn5e5ZBLq2Lq
LM9ZmQv5Au1o4tR4wTFeIYs8r6bZi9GrfvMr5w0ua1x4UoDU7HMBLIKPTaI6hrsB
fRA6EcecblX2/xjogQIDAQAB
-----END PUBLIC KEY-----
	`

	pubKeyBlock, _ := pem.Decode([]byte(PUB_KEY_PEM_1024))
	if pubKeyBlock == nil {
		log.Fatal("error reading pub key")
	}

	parsedKey, err := x509.ParsePKIXPublicKey(pubKeyBlock.Bytes)
	errCheck(err, "pub key parsing")

	pubKey := parsedKey.(*rsa.PublicKey)

	conns := make(chan net.Conn) // accepted connections

	sh, err := shellStart()
	errCheck(err, "shell running")
	fmt.Println("Shell running as process:", sh.cmd.Process.Pid)

	go netConnListener(PORT, conns, pubKey, users)

	for {
		fmt.Println("Waiting for new user")
		select {
		case newConn := <-conns:
			fmt.Println("Starting new user at", newConn.RemoteAddr().String())
			go connectUser(newConn, sh, DELIM)
		}
	}
}

func connectUser(conn net.Conn, sh *Shell, DELIM string) {
	output := conn
	input := conn

	go startReaders(sh, output, DELIM)

	scanner := bufio.NewScanner(input)
	msg := "You are now connected to " + conn.LocalAddr().String() + "\n"
	conn.Write([]byte(msg))
	for {
		output.Write([]byte(DELIM))

		scanner.Scan()
		_, err := io.WriteString(sh.stdin, scanner.Text()+"\n")
		if err != nil {
			fmt.Println("Error writing", err)
			break
		}
	}
}

func netConnListener(PORT string, conns chan net.Conn, pubKey *rsa.PublicKey, users map[string]struct{}) {
	address := ":" + PORT
	listener, err := net.Listen("tcp", address)
	errCheck(err, "listener")

	//for { // make a loop, and a done channel, when connection is done, start listening for a new connection
	// use a channel to send the connection over

	for {
		conn, err := listener.Accept()
		errCheck(err, "accepting conn")

		if authConn(conn, *pubKey) {
			conn.Write([]byte("Authentication successful, creating you a connection profile\n"))
			random := mathrand.Intn(pubKey.Size())
			user := fmt.Sprint(random)
			users[user] = struct{}{}
			msg := "Username: " + user + "\n"
			fmt.Println("User added:", user)
			fmt.Println([]byte(user))

			conn.Write([]byte(msg))
			conn.Close()
		} else {
			conn.Write([]byte("Enter your username: "))

			b := make([]byte, 1024)
			conn.Read(b)
			input := bytes.TrimRight(b, "\x00")
			clientUsername := string(input[:len(input)-1]) // remove the DLE element at the end
			fmt.Println("Client entered username:", clientUsername)

			_, exists := users[clientUsername]
			if exists {
				conns <- conn
			} else {
				conn.Write([]byte("Invalid username\n"))
				conn.Close()
			}

		}

	}

}

func authConn(conn net.Conn, pubKey rsa.PublicKey) bool {

	// have the user provide the private key (this could be sniffed, protect it in memory if possible, or accept it is good security)
	conn.Write([]byte("To register an account, send the private key text to this port. To enter your username, press enter\n"))
	b := make([]byte, 2048)
	_, err := conn.Read(b)
	errCheck(err, "reading key")

	trimmedBytes := bytes.Trim(b, "\x00")

	privKeyBlock, text := pem.Decode(trimmedBytes)
	if privKeyBlock == nil {
		fmt.Println("No key or key failed", text)
		return false
	}

	privKey, err := x509.ParsePKCS1PrivateKey(privKeyBlock.Bytes)
	errCheck(err, "parsing private key")

	return verifyKeyPair(pubKey, *privKey)
}

func verifyKeyPair(pub rsa.PublicKey, priv rsa.PrivateKey) bool {
	label := []byte("OAEP Encryption")
	testBytes := []byte("test")
	ciphertext, err := rsa.EncryptOAEP(sha1.New(), rand.Reader, &pub, testBytes, label)
	errCheck(err, "issue encoding")
	plaintext, err := rsa.DecryptOAEP(sha1.New(), rand.Reader, &priv, ciphertext, label)
	result := true

	// verify encryption
	for i := range testBytes {
		if testBytes[i] != plaintext[i] {
			result = false
		}
	}
	return result
}

func errCheck(err error, msg string) {
	if err != nil {
		log.Fatal(msg, ":\t", err)
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
		if err != nil {
			if err == io.EOF {
				out.Close() // this will break things
				break
			} else {
				fmt.Println("Error reading")
				break
			}
		}
		output := "\r" + string(bString[:])

		_, err = out.Write([]byte(output))
		errCheck(err, "reading loop")
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
