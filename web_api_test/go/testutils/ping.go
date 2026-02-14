package testutils

import (
	"flag"
	"fmt"
	"net"
	"os"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

func main() {
	// Parse command line
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s <hostname>\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() < 1 {
		flag.Usage()
		os.Exit(1)
	}
	host := flag.Arg(0)

	// Resolve IP address
	ips, err := net.LookupIP(host)
	if err != nil {
		fmt.Printf("Could not resolve %s: %v\n", host, err)
		os.Exit(1)
	}
	var dst net.IP
	for _, ip := range ips {
		if ip.To4() != nil {
			dst = ip
			break
		}
	}
	if dst == nil {
		fmt.Printf("No IPv4 address found for %s\n", host)
		os.Exit(1)
	}
	fmt.Printf("PING %s (%s):\n", host, dst)

	// Open raw ICMP connection
	conn, err := icmp.ListenPacket("ip4:icmp", "0.0.0.0")
	if err != nil {
		fmt.Printf("ListenPacket error (may need root): %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	// Prepare ICMP echo request
	id := os.Getpid() & 0xffff
	seq := 1
	msg := icmp.Message{
		Type: ipv4.ICMPTypeEcho,
		Code: 0,
		Body: &icmp.Echo{
			ID:   id,
			Seq:  seq,
			Data: []byte("ping.go test"),
		},
	}
	msgBytes, err := msg.Marshal(nil)
	if err != nil {
		fmt.Printf("Marshal error: %v\n", err)
		os.Exit(1)
	}

	// Send request and measure time
	start := time.Now()
	if _, err := conn.WriteTo(msgBytes, &net.IPAddr{IP: dst}); err != nil {
		fmt.Printf("Write error: %v\n", err)
		os.Exit(1)
	}

	// Wait for reply
	reply := make([]byte, 1500)
	err = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	if err != nil {
		fmt.Printf("SetReadDeadline error: %v\n", err)
		os.Exit(1)
	}
	n, peer, err := conn.ReadFrom(reply)
	if err != nil {
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			fmt.Println("Request timeout")
		} else {
			fmt.Printf("Read error: %v\n", err)
		}
		os.Exit(1)
	}
	rtt := time.Since(start)

	// Parse the reply
	recvMsg, err := icmp.ParseMessage(ipv4.ICMPTypeEchoReply.Protocol(), reply[:n])
	if err != nil {
		fmt.Printf("ParseMessage error: %v\n", err)
		os.Exit(1)
	}
	switch recvMsg.Type {
	case ipv4.ICMPTypeEchoReply:
		echo, ok := recvMsg.Body.(*icmp.Echo)
		if !ok {
			fmt.Println("Invalid echo reply body")
			os.Exit(1)
		}
		if echo.ID != id || echo.Seq != seq {
			fmt.Printf("Reply mismatch: ID=%d Seq=%d (expected ID=%d Seq=%d)\n",
				echo.ID, echo.Seq, id, seq)
			os.Exit(1)
		}
		fmt.Printf("%d bytes from %s: icmp_seq=%d time=%v\n",
			n, peer, seq, rtt)
	default:
		fmt.Printf("Unexpected ICMP type: %v\n", recvMsg.Type)
		os.Exit(1)
	}
}