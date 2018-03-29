package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"runtime"
	"strings"

	"github.com/akamensky/argparse"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/iam"
	ps "github.com/mitchellh/go-ps"
)

func newSessionFromProfile(profile string) *session.Session {
	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState:       session.SharedConfigEnable,
		AssumeRoleTokenProvider: stscreds.StdinTokenProvider,
		Profile:                 profile,
	}))

	return sess
}

func loadSessionFromCache(profile string, cacheFilePath string) (*session.Session, error) {
	b, err := ioutil.ReadFile(cacheFilePath)
	if err != nil {
		return nil, err
	}

	var credMap map[string]string
	err = json.Unmarshal(b, &credMap)

	creds := credentials.NewStaticCredentials(credMap["AccessKeyID"], credMap["SecretAccessKey"], credMap["SessionToken"])
	sess := session.Must(session.NewSession(&aws.Config{
		Credentials: creds,
	}))

	return sess, nil
}

func writeSessionToCache(sess *session.Session, cacheFilePath string) error {
	credMap := make(map[string]string)
	creds, _ := sess.Config.Credentials.Get()

	credMap["AccessKeyID"] = creds.AccessKeyID
	credMap["SecretAccessKey"] = creds.SecretAccessKey
	credMap["SessionToken"] = creds.SessionToken
	credsJSONString, err := json.Marshal(credMap)
	if err != nil {
		fmt.Println("Failed to serialize credentials to JSON. This is a bug")
		return err
	}

	err = ioutil.WriteFile(cacheFilePath, []byte(credsJSONString), 0600)
	if err != nil {
		return err
	}

	return nil
}

func main() {
	parser := argparse.NewParser("aws-switchrole", "Set AWS credentials from CLI profile")
	profile := parser.String("p", "profile", &argparse.Options{Required: true, Help: "CLI profile name"})

	err := parser.Parse(os.Args)
	if err != nil {
		fmt.Print(parser.Usage(err))
	}

	var homeDir string
	if runtime.GOOS == "windows" {
		homeDir = os.Getenv("UserProfile")
	} else {
		homeDir = os.Getenv("HOME")
	}
	cachePathParts := []string{homeDir, ".aws", "cli", "cache"}
	cachePath := strings.Join(cachePathParts[:], string(os.PathSeparator))

	cachedSessionPathParts := []string{homeDir, ".aws", "cli", "cache", "aws-switchrole-" + *profile}
	cachedSessionPath := strings.Join(cachedSessionPathParts[:], string(os.PathSeparator))

	error := os.MkdirAll(cachePath, 0700)
	if error != nil {
		fmt.Println("Error : ", error)
		os.Exit(-1)
	}

	sess, err := loadSessionFromCache(*profile, cachedSessionPath)
	if err != nil || sess.Config.Credentials.IsExpired() {
		sess = newSessionFromProfile(*profile)
		writeSessionToCache(sess, cachedSessionPath)
	}

	svc := iam.New(sess)
	x, err := svc.ListRoles(&iam.ListRolesInput{})
	if err != nil {
		fmt.Println("Error : ", err)
		os.Exit(-1)
	}

	creds, _ := sess.Config.Credentials.Get()
	fmt.Println("AccessKeyID : ", creds.AccessKeyID)
	fmt.Println("SecretAccessKey : ", creds.SecretAccessKey)
	fmt.Println("sessionToken : ", creds.SessionToken)

	fmt.Println("X : ", x.IsTruncated)

	ppid := os.Getppid()
	thisProc, err := ps.FindProcess(ppid)

	if err != nil {
		fmt.Println("Error : ", err)
		os.Exit(-1)
	}
	fmt.Println("Parent Process ID binary name : ", thisProc.Executable())
}
