package main

import (
	"net"
	"log"
)

func ResolveAddr(network, address string) (net.Addr, error) {
	switch network {
		case "tcp", "tcp4", "tcp6": return net.ResolveTCPAddr(network, address)
		case "udp", "udp4", "udp6": return net.ResolveUDPAddr(network, address)
		case "ip", "ip4", "ip6": return net.ResolveIPAddr(network, address)
		case "unix", "unixgram", "unixpacket": return net.ResolveUnixAddr(network, address)
	}
	return nil, nil
}

func main() {
	addr, err := net.ResolveTCPAddr("tcp", "0.0.0.0:8888");
	if err != nil {
		log.Fatal(err)
	}

	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		log.Fatal(err)
	}
	defer l.Close()

	f, err := l.File()
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	fl, err := net.FileListener(f)
	if err != nil {
		log.Fatal(err)
	}
	defer fl.Close()

	fladdr := fl.Addr()

	raddr, err := ResolveAddr(fladdr.Network(), fladdr.String())
	if err != nil {
		log.Fatal(err)
	}

	log.Println(fladdr)
	log.Println(raddr)
}
