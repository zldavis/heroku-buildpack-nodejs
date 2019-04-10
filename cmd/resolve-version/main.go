package main

import (
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"time"
)

type result struct {
	Name                  string     `xml:"Name"`
	KeyCount              int        `xml:"KeyCount"`
	MaxKeys               int        `xml:"MaxKeys"`
	IsTruncated           bool       `xml:"IsTruncated"`
	ContinuationToken     string     `xml:"ContinuationToken"`
	NextContinuationToken string     `xml:"NextContinuationToken"`
	Prefix                string     `xml:"Prefix"`
	Contents              []s3Object `xml:"Contents"`
}

type s3Object struct {
	Key          string    `xml:"Key"`
	LastModified time.Time `xml:"LastModified"`
	ETag         string    `xml:"ETag"`
	Size         int       `xml:"Size"`
	StorageClass string    `xml:"StorageClass"`
}

func main() {
	objects, err := listObjects("heroku-nodebin", "node")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	fmt.Println(len(objects))

	for _, obj := range objects {
		fmt.Println(obj.Key)
	}
}

func listObjectsHelper(bucketName string, options map[string]string) (result, error) {
	var result result
	v := url.Values{}
	v.Set("list-type", "2")
	for key, val := range options {
		v.Set(key, val)
	}
	url := fmt.Sprintf("https://%s.s3.amazonaws.com?%s", bucketName, v.Encode())
	resp, err := http.Get(url)
	if err != nil {
		return result, err
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return result, err
	}

	xml.Unmarshal(body, &result)

	return result, nil
}

func listObjects(bucketName string, prefix string) ([]s3Object, error) {
	var out = []s3Object{}
	var options = map[string]string{"prefix": prefix}

	for {
		result, err := listObjectsHelper(bucketName, options)
		if err != nil {
			return nil, err
		}

		out = append(out, result.Contents...)
		if !result.IsTruncated {
			break
		}

		options["continuation-token"] = result.NextContinuationToken
	}

	return out, nil
}
