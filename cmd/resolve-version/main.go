package main

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"runtime"
	"sort"
	"time"

	"github.com/Masterminds/semver"
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

type release struct {
	binary   string
	stage    string
	platform string
	url      string
	version  *semver.Version
}

func main() {
	if len(os.Args) < 3 {
		printUsage()
		os.Exit(0)
	}
	binary := os.Args[1]
	versionRequirement := os.Args[2]

	if binary == "node" {
		rel, err := resolveNode(versionRequirement)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		fmt.Printf("%s %s\n", rel.version.String(), rel.url)
	} else if binary == "yarn" {
		rel, err := resolveYarn(versionRequirement)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		fmt.Printf("%s %s\n", rel.version.String(), rel.url)
	}
}

func printUsage() {
	fmt.Println("resolve-version binary version-requirement")
}

func getPlatform() string {
	if runtime.GOOS == "darwin" {
		return "darwin-x64"
	}
	return "linux-x64"
}

func resolveNode(versionRequirement string) (release, error) {
	objects, err := listObjects("heroku-nodebin", "node")
	if err != nil {
		return release{}, err
	}

	releases := []release{}
	staging := []release{}
	platform := getPlatform()

	for _, obj := range objects {
		release, err := parseObject(obj.Key)
		if err != nil {
			continue
		}

		if release.platform == platform {
			if release.stage == "staging" {
				staging = append(staging, release)
			} else {
				releases = append(releases, release)
			}
		}
	}

	return matchRelease(releases, versionRequirement)
}

func resolveYarn(versionRequirement string) (release, error) {
	objects, err := listObjects("heroku-nodebin", "yarn")
	if err != nil {
		return release{}, err
	}

	releases := []release{}

	for _, obj := range objects {
		release, err := parseObject(obj.Key)
		if err != nil {
			continue
		}

		releases = append(releases, release)
	}

	return matchRelease(releases, versionRequirement)
}

func matchRelease(releases []release, versionRequirement string) (release, error) {
	constraints, err := semver.NewConstraint(versionRequirement)
	if err != nil {
		return release{}, err
	}

	filtered := []release{}
	for _, release := range releases {
		if constraints.Check(release.version) {
			filtered = append(filtered, release)
		}
	}

	versions := make([]*semver.Version, len(filtered))
	for i, rel := range filtered {
		versions[i] = rel.version
	}

	coll := semver.Collection(versions)
	sort.Sort(coll)

	if len(coll) == 0 {
		return release{}, errors.New("No matching version")
	}

	resolvedVersion := coll[len(coll)-1]

	for _, rel := range filtered {
		if rel.version.Equal(resolvedVersion) {
			return rel, nil
		}
	}
	return release{}, errors.New("Unknown error")
}

func parseObject(key string) (release, error) {
	nodeRegex := regexp.MustCompile("node\\/([^\\/]+)\\/([^\\/]+)\\/node-v([0-9]+.[0-9]+.[0-9]+)-([^.]*)(.*).tar.gz")
	yarnRegex := regexp.MustCompile("yarn\\/([^\\/]+)\\/yarn-v([0-9]+.[0-9]+.[0-9]+).tar.gz")

	if nodeRegex.MatchString(key) {
		match := nodeRegex.FindStringSubmatch(key)
		version, err := semver.NewVersion(match[3])
		if err != nil {
			return release{}, errors.New("Failed to parse version as semver")
		}
		return release{
			binary:   "node",
			stage:    match[1],
			platform: match[2],
			version:  version,
			url:      fmt.Sprintf("https://s3.amazonaws.com/%s/node/%s/%s/node-v%s-%s.tar.gz", "heroku-nodebin", match[1], match[2], match[3], match[2]),
		}, nil
	}

	if yarnRegex.MatchString(key) {
		match := yarnRegex.FindStringSubmatch(key)
		version, err := semver.NewVersion(match[2])
		if err != nil {
			return release{}, errors.New("Failed to parse version as semver")
		}
		return release{
			binary:   "yarn",
			stage:    match[1],
			platform: "",
			url:      fmt.Sprintf("https://s3.amazonaws.com/heroku-nodebin/yarn/release/yarn-v%s.tar.gz", version),
			version:  version,
		}, nil
	}

	return release{}, errors.New("Failed to parse key")
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
