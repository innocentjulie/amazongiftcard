package amazon

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	ActivateGiftCard        = "ActivateGiftCard"
	DeactivateGiftCard      = "DeactivateGiftCard"
	ActivationStatusCheck   = "ActivationStatusCheck"
	CreateGiftCard          = "CreateGiftCard"
	CancelGiftCard          = "CancelGiftCard"
	GetGiftCardActivityPage = "GetGiftCardActivityPage"
	JSON                    = "JSON"
	XML                     = "XML"
)

var (
	partnerId    = "A1VDBH5NED3H0L"
	awsKeyId     = "AKID"
	awsSecretKey = "SECRET"
	serviceName  = "AGCODService"
)

type Value struct {
	Amount       float64 `json:"amount"`
	CurrencyCode string  `json:"currencyCode"`
}

type GiftCardReq struct {
	CreationRequestId string `json:"creationRequestId"`
	PartnerId         string `json:"partnerId"`
	Value             Value  `json:"value"`
}
type EndPoint struct {
	host   string
	region string
}

type Callback func(...any)

// var nextWeek = time.Now().AddDate(0, 0, 7)
var amazonCredentials = aws.Credentials{AccessKeyID: awsKeyId, CanExpire: false, SecretAccessKey: awsSecretKey, SessionToken: "SESSION"}

func CreateGiftCardRequest(partnerId string, sequentialId string, amount float64, currencyCode string) *GiftCardReq {
	return &GiftCardReq{
		CreationRequestId: partnerId + sequentialId,
		PartnerId:         partnerId,
		Value: Value{
			Amount:       amount,
			CurrencyCode: currencyCode,
		},
	}
}

type GiftCardResponse struct {
	CardInfo          CardInfo `json:"cardInfo"`
	CreationRequestId string   `json:"creationRequestId"`
	GcClaimCode       string   `json:"gcClaimCode"`
	GcExpirationDate  string   `json:"gcExpirationDate"`
	GcId              string   `json:"gcId"`
	Status            string   `json:"status"`
}

type CardInfo struct {
	CardNumber     string `json:"cardNumber"`
	CardStatus     string `json:"cardStatus"`
	ExpirationDate string `json:"expirationDate"`
	Value          Value  `json:"value"`
}

func createGiftCardResponse(partnerId string, sequentialId string, amount float64, currencyCode string) *GiftCardResponse {
	return &GiftCardResponse{
		CardInfo: CardInfo{
			CardNumber:     "",
			CardStatus:     "Fulfilled",
			ExpirationDate: "",
			Value: Value{
				Amount:       amount,
				CurrencyCode: currencyCode,
			},
		},
		CreationRequestId: partnerId + sequentialId,
		GcClaimCode:       "3T42-DGTTRJ-GATB",
		GcExpirationDate:  "",
		GcId:              "A1VDBH5NED3H0L",
		Status:            "SUCCESS",
	}
}

type CancelGiftCardReq struct {
	CreationRequestId string `json:"creationRequestId"`
	PartnerId         string `json:"partnerId"`
	GcId              string `json:"gcId"`
}

// CancelGiftCardRequest 取消礼品卡时，用到的CreationRequestId是创建时生成的唯一标识符，所以之前创建的要记录下来用于礼品卡取消
func CancelGiftCardRequest(partnerId string, sequentialId string, gcId string) *CancelGiftCardReq {
	return &CancelGiftCardReq{
		CreationRequestId: partnerId + sequentialId,
		PartnerId:         partnerId,
		GcId:              gcId,
	}
}

type CancelGiftCardResp struct {
	CreationRequestId string `json:"creationRequestId"`
	GcId              string `json:"gcId"`
	Status            string `json:"status"`
}

func CancelGiftCardResponse(partnerId string, sequentialId string, gcId string) *CancelGiftCardResp {
	return &CancelGiftCardResp{
		CreationRequestId: partnerId + sequentialId,
		GcId:              gcId,
		Status:            "SUCCESS",
	}
}

// 检测亚马逊地区节点
func checkRegion(region string) bool {
	//如果region地区中包括以下字段就可以"NA","EU","FE"
	if strings.Contains(region, "NA") || strings.Contains(region, "EU") || strings.Contains(region, "FE") {
		return true
	}
	//if region == "us-east-1" || region == "us-west-2" || region == "eu-west-1" || region == "ap-northeast-1" || region == "ap-southeast-2" {
	//	return true
	//}
	return false
}

// Generates a unique sequential base-36 string based on processor time
// @returns string with length of 10
func getSequentialId() string {
	// Get the current time in nanoseconds
	// This is used to generate a unique sequential id
	// This is not a random number, but it is unique enough for our purposes
	// The sequential id is used to prevent duplicate gift cards from being created
	// If you are using this code in production, you should replace this with a random number
	// or some other unique identifier
	currentTime := time.Now().UnixNano()
	sequentialId := strconv.FormatInt(currentTime, 36)
	return "JJHY" + sequentialId[len(sequentialId)-10:]
	//return "A1VDBH5NED3H0L"
}

// Builds the request body to be POSTed for creating a gift card
func getCreateGiftCardRequestBody(sequentialId string, amount float64, currencyCode string) *GiftCardReq {
	return CreateGiftCardRequest(partnerId, sequentialId, amount, currencyCode)
}

// Builds the request body to be POSTed for cancelling a gift card
func getCancelGiftCardRequestBody(sequentialId string, gcId string) *CancelGiftCardReq {
	return CancelGiftCardRequest(partnerId, sequentialId, gcId)
}

// 根据地区返回最终访问节点
func getEndPointByRegion(region string) EndPoint {
	switch region {
	case "NA":
		return EndPoint{
			host:   "agcod-v2-gamma.amazon.com",
			region: "us-east-1",
		}
	case "EU":
		return EndPoint{
			host:   "agcod-v2-gamma.amazon.co.uk",
			region: "eu-west-1",
		}
	case "FE":
		return EndPoint{
			host:   "agcod-v2-gamma.amazon.co.jp",
			region: "ap-northeast-1",
		}
	default:
		return EndPoint{
			host:   "agcod-v2-gamma.amazon.com",
			region: "us-east-1",
		}
	}
}

// DoCreateGiftCard 初始化请求，生成body,发送请求，处理返回结果
func DoCreateGiftCard(region string, amount float64, currencyCode string, cb Callback) error {
	if checkRegion(region) {
		// Generate a unique sequential id
		sequentialId := getSequentialId()
		// Build the request body
		requestBody := getCreateGiftCardRequestBody(sequentialId, amount, currencyCode)
		// build the request
		rsp, err := getSignedRequest(region, CreateGiftCard, requestBody)
		cb(rsp, err)
		return err
	} else {
		return errors.New("region is not support")
	}
}

// 构建一个签名v4版本的request ,Builds an object with all the specifics for a new https request
// it includes headers with a version 4 signing authentication header
// region - 'NA' for US/CA, 'EU' for IT/ES/DE/FR/UK, 'FE' for JP
// action - 'CreateGiftCard' or 'CancelGiftCard'
// requestBody - generated by _getCreateGiftCardRequestBody or _getCancelGiftCardRequestBody
func getSignedRequest(region string, action string, requestBody *GiftCardReq) (*http.Response, error) {
	//获取授权
	credentials := amazonCredentials
	//根据配置获取最终访问节点
	endpoint := getEndPointByRegion(region)
	//构建请求选项参数结构体
	reqBody, _ := json.Marshal(requestBody)
	req, body := buildRequest(serviceName, endpoint.region, string(reqBody), endpoint, action)

	signer := v4.NewSigner()
	_err := signer.SignHTTP(context.Background(), credentials, req, body, serviceName, endpoint.region, time.Now())
	if _err != nil {
		fmt.Printf("expect no error, got %v", _err)
	}
	return sendRequest(req)
}

func buildRequest(serviceName, region, body string, endpoint EndPoint, action string) (*http.Request, string) {
	reader := strings.NewReader(body)
	return buildRequestWithBodyReader(serviceName, region, reader, endpoint, action)
}
func buildRequestWithBodyReader(serviceName, region string, body io.Reader, point EndPoint, action string) (*http.Request, string) {
	var bodyLen int

	buffer := new(bytes.Buffer)
	_, _ = buffer.ReadFrom(body)
	bodyLen = buffer.Len()

	//type lenner interface {
	//	Len() int
	//}
	//if lr, ok := body.(lenner); ok {
	//	bodyLen = lr.Len()
	//}

	endpoint := "https://" + point.host + "/" + action
	req, _ := http.NewRequest("POST", endpoint, buffer)
	req.URL.Path = "/" + action
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Amz-Date", time.Now().Format("20060102T150405Z"))
	req.Header.Set("x-amz-target", "com.amazonaws.agcod.AGCODService."+action)

	//req.Header.Set("Host", point.host)
	if bodyLen > 0 {
		req.ContentLength = int64(bodyLen)
		//req.Header.Add("Content-Length", fmt.Sprintf("%d", bodyLen))
	}

	//req.Header.Set("X-Amz-Meta-Other-Header", "some-value=!@#$%^&* (+)")
	//req.Header.Add("X-Amz-Meta-Other-Header_With_Underscore", "some-value=!@#$%^&* (+)")
	//req.Header.Add("X-amz-Meta-Other-Header_With_Underscore", "some-value=!@#$%^&* (+)")

	h := sha256.New()
	_, _ = io.Copy(h, body)
	payloadHash := hex.EncodeToString(h.Sum(nil))

	return req, payloadHash
}

func sendRequest(req *http.Request) (*http.Response, error) {
	client := &http.Client{}
	resp, err := client.Do(req)
	//解析返回结果
	fmt.Println(resp, err)
	if err != nil {
		fmt.Println("Error sending request:", err)
		return resp, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error reading response body:", err)
		return resp, nil
	}

	fmt.Println("Response:", string(body))

	return resp, err
}
