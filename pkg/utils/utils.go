package utils

import (
	"strconv"
	"time"
)

func ParseInt(s string) (int, error) {
	return strconv.Atoi(s)
}

func GetCurrentTimestamp() string {
	return time.Now().Format(time.RFC3339)
}