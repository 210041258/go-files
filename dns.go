package testutils

import (
	"flag"
	"fmt"
	"net"
	"os"
	"strings"
)

func main() {
	// Define command-line flags
	recordType := flag.String("type", "A", "DNS record type (A, AAAA, MX, NS, TXT, CNAME)")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options] <domain>\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()

	if flag.NArg() < 1 {
		flag.Usage()
		os.Exit(1)
	}
	domain := flag.Arg(0)

	switch strings.ToUpper(*recordType) {
	case "A":
		lookupA(domain)
	case "AAAA":
		lookupAAAA(domain)
	case "MX":
		lookupMX(domain)
	case "NS":
		lookupNS(domain)
	case "TXT":
		lookupTXT(domain)
	case "CNAME":
		lookupCNAME(domain)
	default:
		fmt.Fprintf(os.Stderr, "Unsupported record type: %s\n", *recordType)
		os.Exit(1)
	}
}

func lookupA(domain string) {
	ips, err := net.LookupHost(domain)
	if err != nil {
		fmt.Printf("A lookup failed: %v\n", err)
		return
	}
	fmt.Println("A records:")
	for _, ip := range ips {
		fmt.Println(ip)
	}
}

func lookupAAAA(domain string) {
	// net.LookupHost returns both A and AAAA; we can filter IPv6 addresses.
	ips, err := net.LookupHost(domain)
	if err != nil {
		fmt.Printf("AAAA lookup failed: %v\n", err)
		return
	}
	fmt.Println("AAAA records:")
	for _, ip := range ips {
		if strings.Contains(ip, ":") { // IPv6 addresses contain colon
			fmt.Println(ip)
		}
	}
}

func lookupMX(domain string) {
	mxs, err := net.LookupMX(domain)
	if err != nil {
		fmt.Printf("MX lookup failed: %v\n", err)
		return
	}
	fmt.Println("MX records (priority, host):")
	for _, mx := range mxs {
		fmt.Printf("%d %s\n", mx.Pref, mx.Host)
	}
}

func lookupNS(domain string) {
	nss, err := net.LookupNS(domain)
	if err != nil {
		fmt.Printf("NS lookup failed: %v\n", err)
		return
	}
	fmt.Println("NS records:")
	for _, ns := range nss {
		fmt.Println(ns.Host)
	}
}

func lookupTXT(domain string) {
	txts, err := net.LookupTXT(domain)
	if err != nil {
		fmt.Printf("TXT lookup failed: %v\n", err)
		return
	}
	fmt.Println("TXT records:")
	for _, txt := range txts {
		fmt.Println(txt)
	}
}

func lookupCNAME(domain string) {
	cname, err := net.LookupCNAME(domain)
	if err != nil {
		fmt.Printf("CNAME lookup failed: %v\n", err)
		return
	}
	fmt.Printf("CNAME record: %s\n", cname)
}