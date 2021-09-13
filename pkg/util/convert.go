package util

import (
	"fmt"
	"strconv"
)

func Vtoa(version float64) string {
	return fmt.Sprintf("%.1f", version)
}

func Atov(str string) float64 {
	version, err := strconv.ParseFloat(str, 32)
	if err != nil {
		return 1.0
	}

	return version
}
