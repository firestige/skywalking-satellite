package utils

import (
	"net"
	"time"
)

func CurrentTimeMillis() int64 {
	// Returns the current time in milliseconds since epoch
	return time.Now().UnixNano() / 1e6
}

func GetNetworkInterfaceIP(nicName string) string {
	// 如果没有指定网卡名称，默认使用 eth0
	if nicName == "" {
		nicName = "eth0"
	}

	// 获取指定网卡的接口
	iface, err := net.InterfaceByName(nicName)
	if err != nil {
		// 如果指定的网卡不存在，尝试使用 eth0
		if nicName != "eth0" {
			return GetNetworkInterfaceIP("eth0")
		}
		// 如果 eth0 也不存在，返回本地回环地址
		return "127.0.0.1"
	}

	// 获取接口的地址列表
	addrs, err := iface.Addrs()
	if err != nil {
		// 如果获取地址失败，返回本地回环地址
		return "127.0.0.1"
	}

	// 遍历地址列表，提取 IPv4 地址
	for _, addr := range addrs {
		if ipNet, ok := addr.(*net.IPNet); ok {
			// 只获取 IPv4 地址，排除回环地址
			if ipNet.IP.To4() != nil && !ipNet.IP.IsLoopback() {
				return ipNet.IP.String()
			}
		}
	}

	// 如果没有找到有效的 IPv4 地址，返回本地回环地址
	return "127.0.0.1"
}
