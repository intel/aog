package utils

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

var TokenMap = make(map[string][]string)

func encodeText(text string) string {
	encoded := url.QueryEscape(text)
	encoded = strings.ReplaceAll(encoded, "+", "%20")
	encoded = strings.ReplaceAll(encoded, "*", "%2A")
	encoded = strings.ReplaceAll(encoded, "%7E", "~")
	return encoded
}

func encodeDict(params map[string]string) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	values := url.Values{}
	for _, k := range keys {
		values.Add(k, params[k])
	}

	encoded := values.Encode()
	encoded = strings.ReplaceAll(encoded, "+", "%20")
	encoded = strings.ReplaceAll(encoded, "*", "%2A")
	encoded = strings.ReplaceAll(encoded, "%7E", "~")

	return encoded
}

func createToken(accessKeyId, accessKeySecret string) (string, int64, error) {
	params := map[string]string{
		"AccessKeyId":      accessKeyId,
		"Action":           "CreateToken",
		"Format":           "JSON",
		"RegionId":         "cn-shanghai",
		"SignatureMethod":  "HMAC-SHA1",
		"SignatureNonce":   uuid.NewString(),
		"SignatureVersion": "1.0",
		"Timestamp":        time.Now().UTC().Format("2006-01-02T15:04:05Z"),
		"Version":          "2019-02-28",
	}

	// Normalized request string
	queryString := encodeDict(params)
	fmt.Println("Normalized request string:", queryString)

	// Construct the string to be signed
	stringToSign := "GET" + "&" + encodeText("/") + "&" + encodeText(queryString)
	fmt.Println("The string to be signed:", stringToSign)

	// Compute signature
	h := hmac.New(sha1.New, []byte(accessKeySecret+"&"))
	h.Write([]byte(stringToSign))
	signature := base64.StdEncoding.EncodeToString(h.Sum(nil))
	fmt.Println("signature:", signature)

	// URL coded signature
	signatureEncoded := encodeText(signature)
	fmt.Println("URL-encoded signature:", signatureEncoded)

	// Splice full URL
	fullURL := fmt.Sprintf("http://nls-meta.cn-shanghai.aliyuncs.com/?Signature=%s&%s", signatureEncoded, queryString)
	fmt.Println("url:", fullURL)

	// Make an HTTP GET request
	resp, err := http.Get(fullURL)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()

	body, _ := ioutil.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		fmt.Println("Response:", string(body))
		return "", 0, fmt.Errorf("HTTP error: %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", 0, err
	}

	if tokenObj, ok := result["Token"].(map[string]interface{}); ok {
		token := tokenObj["Id"].(string)
		expire := int64(tokenObj["ExpireTime"].(float64))
		return token, expire, nil
	}

	fmt.Println("Response:", string(body))
	return "", 0, fmt.Errorf("no Token field found in response")
}

func GetToken(accessKeyId, accessKeySecret string) (string, error) {
	isGetToken := false
	var token string
	if TokenMap != nil {
		tokenInfo, ok := TokenMap[accessKeyId]
		if !ok {
			isGetToken = true
		} else {
			if len(tokenInfo) < 2 {
				isGetToken = true
			} else {
				now := time.Now().Unix()
				expire_time, err := time.Parse("2006-01-02 15:04:05", tokenInfo[1])
				if err != nil {
					isGetToken = true
				} else {
					if expire_time.Unix() > now {
						isGetToken = true
					}
				}
				token = tokenInfo[0]
			}
		}
	}
	if isGetToken {
		delete(TokenMap, accessKeyId)
		newToken, expire, err := createToken(accessKeyId, accessKeySecret)
		if err != nil {
			fmt.Println("Failed to get Token:", err)
			return "", err
		}
		TokenMap[accessKeyId] = append(TokenMap[accessKeyId], newToken)
		if expire > 0 {
			beijingTime := time.Unix(expire/1000, 0).Local().Format("2006-01-02 15:04:05")
			TokenMap[accessKeyId] = append(TokenMap[accessKeyId], beijingTime)
		}
		token = newToken
	}

	return token, nil
}
