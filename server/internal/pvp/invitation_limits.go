package pvp

// Called only for a new application, after idempotent retries are resolved.
func (m *Manager) allowApplication(owner, ip string) bool {
	m.applicationLimitsMu.Lock()
	defer m.applicationLimitsMu.Unlock()
	if m.applicationLimits == nil {
		m.applicationLimits = map[string][]int64{}
	}
	now := m.now()
	since := now - 60000
	for key, times := range m.applicationLimits {
		i := 0
		for i < len(times) && times[i] <= since {
			i++
		}
		if i == len(times) {
			delete(m.applicationLimits, key)
		} else {
			m.applicationLimits[key] = times[i:]
		}
	}
	keys := []string{"owner:" + owner}
	limits := []int{3}
	if ip != "" {
		keys = append(keys, "ip:"+ip)
		limits = append(limits, 10)
	}
	for i, key := range keys {
		if len(m.applicationLimits[key]) >= limits[i] {
			return false
		}
	}
	for _, key := range keys {
		m.applicationLimits[key] = append(m.applicationLimits[key], now)
	}
	return true
}
