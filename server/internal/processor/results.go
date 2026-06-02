package processor

import "time"

func ReclassifyStatus(agentStatus string, notAfter time.Time, thresholdDays int) string {
	if agentStatus != "ok" {
		return agentStatus
	}
	if int(time.Until(notAfter).Hours()/24) < thresholdDays {
		return "expiring"
	}
	return "ok"
}
