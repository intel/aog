package utils

import (
	"bufio"
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"
)

// generate nonce str
const (
	letterBytes   = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	letterIdxBits = 6                    // 6 bits to represent a letter index
	letterIdxMask = 1<<letterIdxBits - 1 // All 1-bits, as many as letterIdxBits
	letterIdxMax  = 63 / letterIdxBits   // # of letter indices fitting in 63 bits
)

func GenerateNonceString(n int) string {
	src := rand.NewSource(time.Now().UnixNano())
	b := make([]byte, n)
	// A src.Int63() generates 63 random bits, enough for letterIdxMax characters!
	for i, cache, remain := n-1, src.Int63(), letterIdxMax; i >= 0; {
		if remain == 0 {
			cache, remain = src.Int63(), letterIdxMax
		}
		if idx := int(cache & letterIdxMask); idx < len(letterBytes) {
			b[i] = letterBytes[idx]
			i--
		}
		cache >>= letterIdxBits
		remain--
	}

	return string(b)
}

func Contains(slice []string, target string) bool {
	for _, str := range slice {
		if str == target {
			return true
		}
	}
	return false
}

func Sha256hex(s string) string {
	b := sha256.Sum256([]byte(s))
	return hex.EncodeToString(b[:])
}

func HmacSha256(s, key string) string {
	hashed := hmac.New(sha256.New, []byte(key))
	hashed.Write([]byte(s))
	return string(hashed.Sum(nil))
}

func HmacSha256String(s, key string) string {
	hashed := hmac.New(sha256.New, []byte(key))
	hashed.Write([]byte(s))
	hmacResult := hashed.Sum(nil)
	signature := hex.EncodeToString(hmacResult)
	return signature
}

func HmacSha1String(s, key string) string {
	hashed := hmac.New(sha1.New, []byte(key))
	hashed.Write([]byte(s))
	hmacResult := hashed.Sum(nil)
	signature := hex.EncodeToString(hmacResult)
	return signature
}

// +-----------------------------+--------------------------------------------------------------------+
// | Device ID                   | 0                                                                  |
// +-----------------------------+--------------------------------------------------------------------+
// | GPU Utilization (%)         | 0                                                                  |
// | EU Array Active (%)         |                                                                    |
// Analyze the output table content of the above terminal command
func ParseTableOutput(output string) map[string]string {
	result := make(map[string]string)
	scanner := bufio.NewScanner(strings.NewReader(string(output)))

	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "|") {
			parts := strings.Split(line, "|")
			if len(parts) >= 3 {
				key := strings.TrimSpace(parts[1])
				value := strings.TrimSpace(parts[2])
				if key != "" && value != "" {
					result[key] = value
				}
			}
		}
	}
	return result
}

// FormatSecondsToSRT 将秒数转换为SRT时间格式 (HH:MM:SS,mmm)
func FormatSecondsToSRT(secondsStr string) string {
	seconds, err := strconv.ParseFloat(secondsStr, 64)
	if err != nil {
		return DefaultSRTTime
	}

	hours := int(seconds) / SecondsPerHour
	minutes := (int(seconds) % SecondsPerHour) / SecondsPerMinute
	secs := int(seconds) % SecondsPerMinute
	milliseconds := int((seconds - float64(int(seconds))) * MillisecondsPerSecond)

	return fmt.Sprintf("%02d:%02d:%02d,%03d", hours, minutes, secs, milliseconds)
}

// ParseTimestamp 将 SRT 格式的时间戳 (00:00:00,000) 转换为毫秒
func ParseTimestamp(timestamp string) int {
	// 去除可能的空白字符
	timestamp = strings.TrimSpace(timestamp)

	// 分离毫秒部分
	parts := strings.Split(timestamp, ",")
	if len(parts) != 2 {
		return -1
	}

	timeStr := parts[0] // 00:00:00
	msStr := parts[1]   // 000

	// 解析时间部分 (小时:分钟:秒)
	timeParts := strings.Split(timeStr, ":")
	if len(timeParts) != 3 {
		return -1
	}

	hours, errH := strconv.Atoi(timeParts[0])
	minutes, errM := strconv.Atoi(timeParts[1])
	seconds, errS := strconv.Atoi(timeParts[2])
	milliseconds, errMs := strconv.Atoi(msStr)

	if errH != nil || errM != nil || errS != nil || errMs != nil {
		return -1
	}

	// 转换为总毫秒数
	totalMs := hours*3600000 + minutes*60000 + seconds*1000 + milliseconds

	return totalMs
}

// GenerateUUID generates a unique UUID string
func GenerateUUID() string {
	// 使用时间和随机字符串组合生成唯一ID
	now := time.Now().UnixNano()
	random := GenerateNonceString(16)
	return fmt.Sprintf("%d-%s", now, random)
}

// DecodeBase64 解码Base64字符串为字节数组
func DecodeBase64(data string) ([]byte, error) {
	// 处理可能的URL安全Base64格式
	data = strings.ReplaceAll(data, "-", "+")
	data = strings.ReplaceAll(data, "_", "/")

	// 处理不完整的Base64字符串
	missing := len(data) % 4
	if missing > 0 {
		data += strings.Repeat("=", 4-missing)
	}

	return base64.StdEncoding.DecodeString(data)
}

func ParseSRTTimestamps(srtContent string) (*int, *int) {
	var beginTime, endTime *int

	// 检查内容是否为空
	if srtContent == "" {
		return nil, nil
	}

	// 按行分割内容
	lines := strings.Split(srtContent, "\n")

	// 查找时间戳行 (格式: 00:00:00,000 --> 00:00:00,000)
	for _, line := range lines {
		if strings.Contains(line, " --> ") {
			parts := strings.Split(line, " --> ")
			if len(parts) == 2 {
				// 解析开始时间
				start := ParseTimestamp(parts[0])
				if start >= 0 {
					startMs := start
					if beginTime == nil || startMs < *beginTime {
						beginTime = &startMs
					}
				}

				// 解析结束时间
				end := ParseTimestamp(parts[1])
				if end >= 0 {
					endMs := end
					if endTime == nil || endMs > *endTime {
						endTime = &endMs
					}
				}

				// 由于我们只需要找到最早的开始时间和最晚的结束时间，可以继续搜索下一个时间戳行
			}
		}
	}

	return beginTime, endTime
}
