package util

import (
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	reservedCidrs []*net.IPNet

	Hostname, _  = os.Hostname()
	HostIPs      []string
	InterfaceIPs []string
)

func UUID() (string, error) {
	val, err := uuid.NewRandom()
	if err != nil {
		return "", err
	}
	return val.String(), err
}

func GetMacOrLinuxLocalIP() string {
	var itf *net.Interface
	var err error
	switch runtime.GOOS {
	case "linux":
		itf, err = net.InterfaceByName("eth0")

	case "darwin":
		itf, err = net.InterfaceByName("en0")
	}
	if err != nil {
		panic(err)
	}
	addrs, err := itf.Addrs()
	if err != nil {
		panic(err)
	}

	for _, addr := range addrs {
		ip := addr.String()
		cip, _, err := net.ParseCIDR(ip)
		if err != nil {
			panic(err)
		}
		ip = cip.String()
		if IsLocalAddress(ip) {
			return ip
		}
	}

	panic("error")
}

func init() {
	cidrBlocks := []string{
		"127.0.0.1/8",
		"10.0.0.0/8",
		"100.64.0.0/10",
		"169.254.0.0/16",
		"172.16.0.0/12",
		"192.0.0.0/24",
		"192.168.0.0/16",
		"198.18.0.0/15",
	}
	reservedCidrs = []*net.IPNet{}
	for _, v := range cidrBlocks {
		_, cidrnet, err := net.ParseCIDR(v)
		if err != nil {
			continue
		}
		reservedCidrs = append(reservedCidrs, cidrnet)
	}

	if ipAddrs := LookupIPTimeout(Hostname, 30*time.Millisecond); len(ipAddrs) != 0 {
		for _, ip := range ipAddrs {
			if ip.IsGlobalUnicast() {
				HostIPs = append(HostIPs, ip.String())
			}
		}
	}
	if ifAddrs, _ := net.InterfaceAddrs(); len(ifAddrs) != 0 {
		for i := range ifAddrs {
			var ip net.IP
			switch in := ifAddrs[i].(type) {
			case *net.IPNet:
				ip = in.IP
			case *net.IPAddr:
				ip = in.IP
			}
			if ip.IsGlobalUnicast() {
				InterfaceIPs = append(InterfaceIPs, ip.String())
			}
		}
	}
}

func IsLocalAddress(addr string) bool {
	for idx := range reservedCidrs {
		if reservedCidrs[idx].Contains(net.ParseIP(addr)) {
			return true
		}
	}
	return false
}

func LookupIP(host string) []net.IP {
	ipAddrs, _ := net.LookupIP(host)
	return ipAddrs
}

func LookupIPTimeout(host string, timeout time.Duration) []net.IP {
	cntx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	var ch = make(chan []net.IP, 1)
	go func() {
		ch <- LookupIP(host)
	}()
	select {
	case ipAddrs := <-ch:
		return ipAddrs
	case <-cntx.Done():
		return nil
	}
}

func ResolveTCPAddr(addr string) *net.TCPAddr {
	tcpAddr, _ := net.ResolveTCPAddr("tcp", addr)
	return tcpAddr
}

func ResolveTCPAddrTimeout(addr string, timeout time.Duration) *net.TCPAddr {
	cntx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	var ch = make(chan *net.TCPAddr, 1)
	go func() {
		ch <- ResolveTCPAddr(addr)
	}()
	select {
	case tcpAddr := <-ch:
		return tcpAddr
	case <-cntx.Done():
		return nil
	}
}

func ReplaceUnspecifiedIP(network string, listenAddr, globalAddr string) (string, error) {
	if globalAddr == "" {
		return replaceUnspecifiedIP(network, listenAddr, true)
	}
	return replaceUnspecifiedIP(network, globalAddr, false)
}

func replaceUnspecifiedIP(network string, address string, replace bool) (string, error) {
	switch network {
	default:
		return "", net.UnknownNetworkError(network)
	case "unix", "unixpacket":
		return address, nil
	case "tcp", "tcp4", "tcp6":
		tcpAddr, err := net.ResolveTCPAddr(network, address)
		if err != nil {
			return "", err
		}
		if tcpAddr.Port != 0 {
			if !tcpAddr.IP.IsUnspecified() {
				return address, nil
			}
			if replace {
				if len(HostIPs) != 0 {
					return net.JoinHostPort(Hostname, strconv.Itoa(tcpAddr.Port)), nil
				}
				if len(InterfaceIPs) != 0 {
					return net.JoinHostPort(InterfaceIPs[0], strconv.Itoa(tcpAddr.Port)), nil
				}
			}
		}
		return "", fmt.Errorf("resolve address '%s' to '%s'", address, tcpAddr.String())
	}
}

// GetOutboundIP gets preferred outbound ip of this machine.
func GetOutboundIP() (string, error) {
	resp, err := http.Get("http://myip.ipip.net")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	bdata, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(bdata), nil
}

// ExternalIP get external ip.
func ExternalIP() (res []string) {
	inters, err := net.Interfaces()
	if err != nil {
		return
	}
	for _, inter := range inters {
		if !strings.HasPrefix(inter.Name, "lo") {
			addrs, err := inter.Addrs()
			if err != nil {
				continue
			}
			for _, addr := range addrs {
				if ipnet, ok := addr.(*net.IPNet); ok {
					if ipnet.IP.IsLoopback() || ipnet.IP.IsLinkLocalMulticast() || ipnet.IP.IsLinkLocalUnicast() {
						continue
					}
					if ip4 := ipnet.IP.To4(); ip4 != nil {
						switch true {
						case ip4[0] == 10:
							continue
						case ip4[0] == 172 && ip4[1] >= 16 && ip4[1] <= 31:
							continue
						case ip4[0] == 192 && ip4[1] == 168:
							continue
						default:
							res = append(res, ipnet.IP.String())
						}
					}
				}
			}
		}
	}
	return
}
