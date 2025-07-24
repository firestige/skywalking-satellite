package utils

import "net"

// GetLocalAddresses 获取本机IP地址
func GetLocalAddresses() []string {
	var addresses []string

	interfaces, err := net.Interfaces()
	if err != nil {
		return addresses
	}

	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 {
			continue // 接口未启用
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
				if ipnet.IP.To4() != nil {
					addresses = append(addresses, ipnet.IP.String())
				}
			}
		}
	}

	// 添加回环地址
	addresses = append(addresses, "127.0.0.1", "::1")

	return addresses
}
