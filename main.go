package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/grandcat/zeroconf"
)

var (
	lanIP      string
	mdnsServer *zeroconf.Server
)

func main() {

	setupFileLog()

	appConfig = loadConfig()

	initPairing()
	deviceInfo["pk"] = []byte(accessoryPub)

	lanIP = detectLanIP()
	if lanIP == "" {
		log.Fatal("mDNS: could not determine local LAN IP — check your network connection")
	}

	if err := startMDNS(appConfig.AirplayName); err != nil {
		log.Printf("mDNS: failed to announce service: %v", err)
	}

	addr := net.JoinHostPort("0.0.0.0", strconv.Itoa(appConfig.Port))
	if err := serveRTSP(addr); err != nil {
		log.Fatalf("RTSP: %v", err)
	}

	go func() {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
		<-ch
		if mdnsServer != nil {
			mdnsServer.Shutdown()
		}
		os.Exit(0)
	}()

}

func startMDNS(name string) error {
	if mdnsServer != nil {
		mdnsServer.Shutdown()
		mdnsServer = nil
	}

	pkHex := fmt.Sprintf("%x", accessoryPub)
	var ifaces []net.Interface
	if iface := ifaceForIP(lanIP); iface != nil {
		ifaces = []net.Interface{*iface}
	}

	server, err := zeroconf.RegisterProxy(
		name,
		"_airplay._tcp",
		"local.",
		appConfig.Port,
		"GoPlay",
		[]string{lanIP},
		[]string{
			"deviceid=AA:BB:CC:DD:EE:FF",
			"features=0x5A7FFFF7,0x1E",
			"model=AppleTV3,2",
			"srcvers=220.68",
			"flags=0x4",
			"vv=2",
			"pk=" + pkHex,
		},
		ifaces,
	)
	if err != nil {
		return err
	}
	mdnsServer = server
	log.Printf("mDNS: announcing %q at %s:%d", name, lanIP, appConfig.Port)
	return nil
}

func detectLanIP() string {
	if conn, err := net.Dial("udp", "8.8.8.8:80"); err == nil {
		defer conn.Close()
		if ua, ok := conn.LocalAddr().(*net.UDPAddr); ok {
			if ip4 := ua.IP.To4(); ip4 != nil && ip4.IsPrivate() {
				return ip4.String()
			}
		}
	}
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			ipnet, ok := a.(*net.IPNet)
			if !ok {
				continue
			}
			ip4 := ipnet.IP.To4()
			if ip4 == nil {
				continue
			}
			if ip4.IsPrivate() {
				return ip4.String()
			}
		}
	}
	return ""
}

func ifaceForIP(ip string) *net.Interface {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	for i := range ifaces {
		addrs, _ := ifaces[i].Addrs()
		for _, a := range addrs {
			if ipnet, ok := a.(*net.IPNet); ok && ipnet.IP.String() == ip {
				return &ifaces[i]
			}
		}
	}
	return nil
}
