package testutils

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"

	"golang.org/x/crypto/ssh"
)

func main() {
	// Command line flags
	user := flag.String("u", "root", "SSH username")
	host := flag.String("h", "localhost:22", "Remote host address (host:port)")
	password := flag.String("p", "", "Password (or use SSH agent)")
	command := flag.String("c", "echo hello", "Command to execute")
	flag.Parse()

	if *password == "" {
		log.Fatal("Password is required (or implement key authentication)")
	}

	// SSH client configuration
	config := &ssh.ClientConfig{
		User: *user,
		Auth: []ssh.AuthMethod{
			ssh.Password(*password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // NOT for production!
	}

	// Connect to the remote host
	client, err := ssh.Dial("tcp", *host, config)
	if err != nil {
		log.Fatalf("Failed to dial: %v", err)
	}
	defer client.Close()

	// Create a session
	session, err := client.NewSession()
	if err != nil {
		log.Fatalf("Failed to create session: %v", err)
	}
	defer session.Close()

	// Connect stdin/stdout/stderr
	session.Stdin = os.Stdin
	session.Stdout = os.Stdout
	session.Stderr = os.Stderr

	// Run the command
	fmt.Printf("Running command: %s\n", *command)
	if err := session.Run(*command); err != nil {
		if exitErr, ok := err.(*ssh.ExitError); ok {
			log.Fatalf("Command failed with exit code %d", exitErr.ExitStatus())
		}
		log.Fatalf("Failed to run command: %v", err)
	}
}