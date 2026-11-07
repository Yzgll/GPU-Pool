package gpu

import (
	"fmt"
	"runtime"
)

func NewCollector() (interface {
	Collect() ([]GPUInfo, error)
}, error) {
	switch runtime.GOOS {
	case "windows", "linux":
		return NewNVIDIACollector(), nil

	case "darwin":
		return NewAppleCollector(), nil

	default:
		return nil, fmt.Errorf(
			"unsupported GPU platform: %s",
			runtime.GOOS,
		)
	}
}
