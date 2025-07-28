package web

import (
	"math"
	"strconv"
	"time"
)

// ConvertThreadTsToTime converts Slack thread timestamp to time.Time
func ConvertThreadTsToTime(threadTs string) (time.Time, error) {
	threadTsFloat, err := strconv.ParseFloat(threadTs, 64)
	if err != nil {
		return time.Time{}, err
	}

	seconds := int64(threadTsFloat)
	nanoseconds := int64((threadTsFloat - math.Floor(threadTsFloat)) * 1e9)

	return time.Unix(seconds, nanoseconds), nil
}

// FormatThreadTime formats thread time for display
func FormatThreadTime(threadTime time.Time) string {
	return threadTime.Format("2006-01-02 15:04:05 MST")
}
