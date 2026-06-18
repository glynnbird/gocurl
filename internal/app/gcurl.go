package gcurl

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/term"
)

// GoCurl is a struct that contains everything we need to run gocurl
type GoCurl struct {
	cloudantURL *url.URL
	curlArgs    []string
	jqArgs      []string
	curlPath    string
	jqPath      string
	tmpFilename string
	userAgent   string
	isTTY       bool
	useIAM      bool
	bearerToken string
}

// New creates a new GoCurl struct, massaging the parameters, assuming the last parameter is the URL.
// It runs "which" to locate curl and jq and formulates the arguments for each
func New() (*GoCurl, error) {

	// Check for IAM authentication
	iamAPIKey := os.Getenv("IAM_API_KEY")
	useIAM := len(iamAPIKey) > 0
	var bearerToken string
	var couchURLStr string

	if useIAM {
		// IAM authentication mode
		// Get bearer token (from cache or fetch new one)
		token, err := GetIAMToken(iamAPIKey)
		if err != nil {
			return nil, fmt.Errorf("failed to get IAM token: %w", err)
		}
		bearerToken = token

		// For IAM, COUCH_URL should not contain credentials
		couchURLStr = os.Getenv("COUCH_URL")
		if len(couchURLStr) == 0 {
			return nil, errors.New("COUCH_URL environment variable not found")
		}
	} else {
		// Basic authentication mode (legacy)
		couchURLStr = os.Getenv("COUCH_URL")
		if len(couchURLStr) == 0 {
			return nil, errors.New("COUCH_URL environment variable not found")
		}
	}

	// parse the incoming parameters
	// - remove the first one (the executable)
	// - assume the last one is the relative path (if present)
	args := os.Args
	// remove first argument (the executable)
	args = args[1:]
	relativePath := "/"

	// if arguments supplied
	if len(args) > 0 {
		// extract the relative path from the end
		relativePath = args[len(args)-1]
		args = args[0 : len(args)-1]
	}

	// convert relative URL into absolute URL
	base, err := url.Parse(couchURLStr)
	if err != nil {
		return nil, err
	}
	rel, err := url.Parse(relativePath)
	if err != nil {
		return nil, err
	}
	cloudantURL := base.ResolveReference(rel)

	// create temp file
	tmpFilename := filepath.Join(os.TempDir(), "gocurl.tmp")

	// additional curl parameters
	userAgent := "gocurl/dev"
	userAgentHeader := fmt.Sprintf("user-agent: %v", userAgent)
	extraParams := []string{
		"-b",
		tmpFilename,
		"-c",
		tmpFilename,
		"-s",
		"-g",
		"-H",
		"content-type: application/json",
		"-H",
		userAgentHeader,
	}

	// Add IAM bearer token authorization header if using IAM
	if useIAM {
		authHeader := fmt.Sprintf("Authorization: Bearer %s", bearerToken)
		extraParams = append(extraParams, "-H", authHeader)
	}

	// Add the URL as the last parameter
	extraParams = append(extraParams, cloudantURL.String())
	args = append(args, extraParams...)

	// run which to find the path of curl & jq (if available)
	curlPath, err := which("curl")
	if err != nil {
		return nil, err
	}
	jqPath, _ := which("jq")

	// check whether the output is TTY or a file
	isTTY := term.IsTerminal(int(os.Stdout.Fd()))

	// create new struct
	gc := GoCurl{
		cloudantURL: cloudantURL,
		curlArgs:    args,
		jqArgs:      []string{"."},
		curlPath:    curlPath,
		jqPath:      jqPath,
		tmpFilename: tmpFilename,
		userAgent:   userAgent,
		isTTY:       isTTY,
		useIAM:      useIAM,
		bearerToken: bearerToken,
	}

	return &gc, nil
}

// Run executes a gocurl command, shelling out to curl and optionally jq, passing in the
// parameters from the GoCurl struct.
func (gc *GoCurl) Run() error {

	// Process A - curl
	var curl, jq *exec.Cmd
	curl = exec.Command(gc.curlPath, gc.curlArgs...)
	pipeMode := (gc.isTTY && len(gc.jqPath) > 0)

	// if we're not piping to a file & jq is present
	if pipeMode {

		// execute jq
		jq = exec.Command(gc.jqPath, gc.jqArgs...)

		// connect curl's stdout to jq's stdin
		pipe, err := curl.StdoutPipe()
		if err != nil {
			return err
		}
		jq.Stdin = pipe
		jq.Stdout = os.Stdout
		err = jq.Start()
		if err != nil {
			return err
		}
	} else {
		// otherwise it's curl --> stdout
		curl.Stdout = os.Stdout
	}

	// Start curl
	err := curl.Start()
	if err != nil {
		return err
	}

	// Wait for both
	err = curl.Wait()
	if err != nil {
		return err
	}
	if pipeMode {
		err = jq.Wait()
		if err != nil {
			return err
		}
	}
	return nil
}

// which executes the "which" command-line tool to calculate the path of
// the passed-in script. This serves as means of detecting whether an
// executable is present or not.
func which(script string) (string, error) {
	// Run which to find the path of the supplied command
	out, err := exec.Command("which", script).Output()
	if err != nil {
		return "", err
	}
	outStr := string(out)
	outStr = strings.TrimSuffix(outStr, "\n")
	outStr = strings.TrimSuffix(outStr, "\r")
	return outStr, nil
}
