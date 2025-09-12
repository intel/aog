package utils

import (
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	"github.com/intel/aog/internal/types"
	"github.com/jaypipes/ghw"
	"github.com/shirou/gopsutil/disk"
)

func DetectGpuModel() string {
	gpu, err := ghw.GPU()
	if err != nil {
		return types.GPUTypeNone
	}

	hasNvidia := false
	hasAMD := false
	hasIntel := false

	for _, card := range gpu.GraphicsCards {
		// 转为小写
		productName := strings.ToLower(card.DeviceInfo.Product.Name)
		if strings.Contains(productName, "nvidia") {
			hasNvidia = true
		} else if strings.Contains(productName, "amd") {
			hasAMD = true
		} else if strings.Contains(productName, "intel") && (strings.Contains(productName, "arc") || strings.Contains(productName, "core")) {
			hasIntel = true
		}
	}

	if hasNvidia && hasAMD {
		return types.GPUTypeNvidia + "," + types.GPUTypeAmd
	} else if hasNvidia {
		return types.GPUTypeNvidia
	} else if hasAMD {
		return types.GPUTypeAmd
	} else if hasIntel {
		return types.GPUTypeIntelArc
	} else {
		return types.GPUTypeNone
	}
}

func SystemDiskSize(path string) (*types.PathDiskSizeInfo, error) {
	if runtime.GOOS == "windows" {
		path = filepath.VolumeName(path)
	}
	usage, err := disk.Usage(path)
	if err != nil {
		return &types.PathDiskSizeInfo{}, err
	}
	res := &types.PathDiskSizeInfo{}
	res.TotalSize = int(usage.Total / 1024 / 1024 / 1024)
	res.FreeSize = int(usage.Free / 1024 / 1024 / 1024)
	res.UsageSize = int(usage.Used / 1024 / 1024 / 1024)
	return res, nil
}

func ParseSizeToGB(sizeStr string) float64 {
	s := strings.TrimSpace(strings.ToUpper(sizeStr))
	re := regexp.MustCompile(`([\d.]+)\s*(GB|MB)`)
	matches := re.FindStringSubmatch(s)
	if len(matches) != 3 {
		return 1
	}
	num, _ := strconv.ParseFloat(matches[1], 64)
	unit := matches[2]
	if unit == "GB" {
		return num
	} else if unit == "MB" {
		return num / 1024
	}
	return 1
}

func IpexOllamaSupportGPUStatus() bool {
	gpu, err := ghw.GPU()
	if err != nil {
		return false
	}

	for _, card := range gpu.GraphicsCards {
		if strings.Contains(card.DeviceInfo.Product.Name, "Intel") {
			if strings.Contains(card.DeviceInfo.Product.Name, "Arc") || strings.Contains(card.DeviceInfo.Product.Name, "Core") {
				return true
			}
		}
	}
	return false
}
