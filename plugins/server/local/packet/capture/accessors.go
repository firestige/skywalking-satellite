package capture

// GetPoolStats 获取池统计信息
func (nc *networkCapture) GetPoolStats() map[string]int64 {
	if nc.poolManager != nil {
		return nc.poolManager.GetStats()
	}
	return map[string]int64{}
}

// GetPoolHitRate 获取池命中率
func (nc *networkCapture) GetPoolHitRate() float64 {
	if nc.poolManager != nil {
		return nc.poolManager.GetHitRate()
	}
	return 0
}
