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
)

func envMapFromSession(sess *session.Session) map[string]string {
	creds, _ := sess.Config.Credentials.Get()

	envMap := make(map[string]string)

	envMap["AWS_ACCESS_KEY_ID"] = creds.AccessKeyID
	envMap["AWS_SECRET_ACCESS_KEY"] = creds.SecretAccessKey
	envMap["AWS_SESSION_TOKEN"] = creds.SessionToken
	envMap["AWS_SECURITY_TOKEN"] = creds.SessionToken

	return envMap
}

// Return new session object given a profile name that's correctly
// configured in the ~/.aws config and credential files
func newSessionFromProfile(profile string) *session.Session {
	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState:       session.SharedConfigEnable,
		AssumeRoleTokenProvider: stscreds.StdinTokenProvider,
		Profile:                 profile,
	}))

	return sess
}

// Return new session object from JSON cache file
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

	musteredCreds, _ := sess.Config.Credentials.Get()
	if musteredCreds.ProviderName == "EnvConfigCredentials" {
		return nil, nil
	}

	// fmt.Println(creds)
	return sess, nil
}

// Given a session object write it to a JSON cache file
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

	// Attempt to load the session from the cache file
	sess, err := loadSessionFromCache(*profile, cachedSessionPath)

	// If no cached session found or error encountered
	// get a new session and cache it
	if sess == nil || err != nil {
		sess = newSessionFromProfile(*profile)
		writeSessionToCache(sess, cachedSessionPath)
	}

	envMap := envMapFromSession(sess)
	for key, value := range envMap {
		fmt.Printf("$env:%s=\"%s\"\r\n", key, value)
	}

	// fmt.Println("AccessKeyID : ", creds.AccessKeyID)
	// fmt.Println("SecretAccessKey : ", creds.SecretAccessKey)
	// fmt.Println("sessionToken : ", creds.SessionToken)

	/*
		svc := iam.New(sess)
		x, err := svc.ListRoles(&iam.ListRolesInput{})
		if err != nil {
			fmt.Println("Error : ", err)
			os.Exit(-1)
		}

		fmt.Println("X : ", x.IsTruncated)

		ppid := os.Getppid()
		thisProc, err := ps.FindProcess(ppid)

		if err != nil {
			fmt.Println("Error : ", err)
			os.Exit(-1)
		}
		fmt.Println("Parent Process ID binary name : ", thisProc.Executable())
	*/
}
