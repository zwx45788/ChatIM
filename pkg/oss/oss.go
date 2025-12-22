package oss

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type OSSClient struct {
	AccessKeyID     string
	AccessKeySecret string
	Endpoint        string
	BucketName      string
	BucketURL       string
}

type PolicyConfig struct {
	Expiration string        `json:"expiration"`
	Conditions []interface{} `json:"conditions"`
}

type UploadSignature struct {
	AccessKeyID string `json:"accessKeyId"`
	Policy      string `json:"policy"`
	Signature   string `json:"signature"`
	Host        string `json:"host"`
	Key         string `json:"key"`
	Expire      int64  `json:"expire"`
}

func NewOSSClient(accessKeyID, accessKeySecret, endpoint, bucketName string) *OSSClient {
	bucketURL := fmt.Sprintf("https://%s.%s", bucketName, endpoint)
	return &OSSClient{
		AccessKeyID:     accessKeyID,
		AccessKeySecret: accessKeySecret,
		Endpoint:        endpoint,
		BucketName:      bucketName,
		BucketURL:       bucketURL,
	}
}

// GenerateUploadSignature 生成上传签名
func (c *OSSClient) GenerateUploadSignature(fileType string, maxSize int64) (*UploadSignature, error) {
	now := time.Now()
	expireTime := now.Add(30 * time.Minute)
	expireUnix := expireTime.Unix()

	// 生成上传路径
	var dir string
	if fileType == "image" {
		dir = fmt.Sprintf("chatim/images/%s/%s/", now.Format("2006"), now.Format("01"))
	} else {
		dir = fmt.Sprintf("chatim/files/%s/%s/", now.Format("2006"), now.Format("01"))
	}

	// 生成文件名（使用UUID）
	fileName := uuid.New().String()
	key := dir + fileName

	// 构建Policy
	policy := PolicyConfig{
		Expiration: expireTime.UTC().Format("2006-01-02T15:04:05Z"),
		Conditions: []interface{}{
			map[string]string{"bucket": c.BucketName},
			[]interface{}{"starts-with", "$key", dir},
			[]interface{}{"content-length-range", 0, maxSize},
		},
	}

	// Policy转JSON
	policyJSON, err := json.Marshal(policy)
	if err != nil {
		return nil, err
	}

	// Base64编码Policy
	policyBase64 := base64.StdEncoding.EncodeToString(policyJSON)

	// 使用AccessKeySecret对Policy签名
	h := hmac.New(sha1.New, []byte(c.AccessKeySecret))
	h.Write([]byte(policyBase64))
	signature := base64.StdEncoding.EncodeToString(h.Sum(nil))

	return &UploadSignature{
		AccessKeyID: c.AccessKeyID,
		Policy:      policyBase64,
		Signature:   signature,
		Host:        c.BucketURL,
		Key:         key,
		Expire:      expireUnix,
	}, nil
}

// GetFileURL 获取文件访问URL
func (c *OSSClient) GetFileURL(key string) string {
	return fmt.Sprintf("%s/%s", c.BucketURL, key)
}
