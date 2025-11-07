package sniffer

import (
	"fmt"
	"strings"

	"github.com/google/gopacket/pcap"
)

// ListDeviceNames returns the list of adapters available for sniffing on
// this computer. If the withDescription parameter is set to true, a human
// readable version of the adapter name is added. If the withIP parameter
// is set to true, IP address of the adapter is added.
func ListDeviceNames(withDescription bool, withIP bool) ([]string, error) {
	devices, err := pcap.FindAllDevs()
	if err != nil {
		return []string{}, err
	}

	ret := []string{}
	for _, dev := range devices {
		r := dev.Name

		if withDescription {
			desc := "No description available"
			if len(dev.Description) > 0 {
				desc = dev.Description
			}
			r += fmt.Sprintf(" (%s)", desc)
		}

		if withIP {
			ips := "Not assigned ip address"
			if len(dev.Addresses) > 0 {
				ips = ""

				for i, address := range []pcap.InterfaceAddress(dev.Addresses) {
					// Add a space between the IP address.
					if i > 0 {
						ips += " "
					}

					ips += address.IP.String()
				}
			}
			r += fmt.Sprintf(" (%s)", ips)

		}
		ret = append(ret, r)
	}
	filterDeviceName(ret)
	return ret, nil
}

func filterDeviceName(name []string) {
	for _, d := range name {
		if strings.HasPrefix(d, "bluetooth") || strings.HasPrefix(d, "dbus") || strings.HasPrefix(d, "nf") || strings.HasPrefix(d, "usb") {
			continue
		}
		fmt.Printf("-i %s\n", d)
	}
}
