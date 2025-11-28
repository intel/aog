package utils

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type SignParams struct {
	SecretId      string           `json:"secret_id"`
	SecretKey     string           `json:"secret_key"`
	RequestBody   string           `json:"request_body"`
	RequestUrl    string           `json:"request_url"`
	RequestMethod string           `json:"request_method"`
	RequestHeader http.Header      `json:"request_header"`
	CommonParams  SignCommonParams `json:"common_params"`
}

type SignCommonParams struct {
	Version string `json:"version"`
	Action  string `json:"action"`
	Region  string `json:"region"`
}

type SignAuthInfo struct {
	SecretId  string `json:"secret_id"`
	SecretKey string `json:"secret_key"`
}

type ServiceConfig struct {
	Endpoint     string `json:"endpoint"`
	ExtraHeaders string `json:"extra_headers"`
}

type TencentSignAuthenticator struct {
	AuthInfo      string         `json:"auth_info"`
	Req           *http.Request  `json:"request"`
	ServiceConfig *ServiceConfig `json:"service_config"`
	ReqBody       string         `json:"req_body"`
}

func Sign(s *TencentSignAuthenticator) error {
	var authInfoData SignAuthInfo
	err := json.Unmarshal([]byte(s.AuthInfo), &authInfoData)
	if err != nil {
		return err
	}

	commonParams := SignParams{
		SecretId:      authInfoData.SecretId,
		SecretKey:     authInfoData.SecretKey,
		RequestUrl:    s.ServiceConfig.Endpoint,
		RequestBody:   s.ReqBody,
		RequestHeader: s.Req.Header,
		RequestMethod: s.Req.Method,
	}
	if s.ServiceConfig.ExtraHeaders != "" {
		var serviceExtraInfo SignCommonParams
		err := json.Unmarshal([]byte(s.ServiceConfig.ExtraHeaders), &serviceExtraInfo)
		if err != nil {
			return err
		}
		commonParams.CommonParams = serviceExtraInfo
	}

	err = TencentSignGenerate(commonParams, s.Req)
	if err != nil {
		return err
	}
	return nil
}

func TencentSignGenerate(p SignParams, req *http.Request) error {
	secretId := p.SecretId
	secretKey := p.SecretKey
	parseUrl, err := url.Parse(p.RequestUrl)
	if err != nil {
		return err
	}
	host := parseUrl.Host
	service := strings.Split(host, ".")[0]
	algorithm := "TC3-HMAC-SHA256"
	tcVersion := p.CommonParams.Version
	action := p.CommonParams.Action
	region := p.CommonParams.Region
	timestamp := time.Now().Unix()

	// step 1: build canonical request string
	httpRequestMethod := p.RequestMethod
	canonicalURI := "/"
	canonicalQueryString := ""
	canonicalHeaders := ""
	signedHeaders := ""
	for k, v := range p.RequestHeader {
		if strings.ToLower(k) == "content-type" {
			signedHeaders += fmt.Sprintf("%s;", strings.ToLower(k))
			canonicalHeaders += fmt.Sprintf("%s:%s\n", strings.ToLower(k), strings.ToLower(v[0]))
		}
	}
	signedHeaders += "host"
	canonicalHeaders += fmt.Sprintf("%s:%s\n", "host", host)
	signedHeaders = strings.TrimRight(signedHeaders, ";")
	payload := p.RequestBody
	hashedRequestPayload := Sha256hex(payload)
	canonicalRequest := fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n%s",
		httpRequestMethod,
		canonicalURI,
		canonicalQueryString,
		canonicalHeaders,
		signedHeaders,
		hashedRequestPayload)

	// step 2: build string to sign
	date := time.Unix(timestamp, 0).UTC().Format("2006-01-02")
	credentialScope := fmt.Sprintf("%s/%s/tc3_request", date, service)
	hashedCanonicalRequest := Sha256hex(canonicalRequest)
	string2sign := fmt.Sprintf("%s\n%d\n%s\n%s",
		algorithm,
		timestamp,
		credentialScope,
		hashedCanonicalRequest)

	// step 3: sign string
	secretDate := HmacSha256(date, "TC3"+secretKey)
	secretService := HmacSha256(service, secretDate)
	secretSigning := HmacSha256("tc3_request", secretService)
	signature := hex.EncodeToString([]byte(HmacSha256(string2sign, secretSigning)))

	// step 4: build authorization
	authorization := fmt.Sprintf("%s Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		algorithm,
		secretId,
		credentialScope,
		signedHeaders,
		signature)

	req.Header.Add("Authorization", authorization)
	req.Header.Add("X-TC-Timestamp", strconv.FormatInt(timestamp, 10))
	req.Header.Add("X-TC-Version", tcVersion)
	req.Header.Add("X-TC-Region", region)
	req.Header.Add("X-TC-Action", action)
	return nil
}
