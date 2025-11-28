package utils

import (
	"bufio"
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"math/rand"
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
