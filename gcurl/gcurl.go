package gcurl

import (
	"errors"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/term"
)

type GoCurl struct {
	cloudantURL *url.URL
	curlArgs    []string
	jqArgs      []string
	curlPath    string
	jqPath      string
	tmpFilename string
	userAgent   string
	isTTY       bool
}

func New() (*GoCurl, error) {

	// read the COUCH_URL environment variable
	couchURLStr := os.Getenv("COUCH_URL")
	if len(couchURLStr) == 0 {
		return nil, errors.New("COUCH_URL environment variable not found")
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
		"user-agent: gocurl/1.0",
		cloudantURL.String(),
	}
	args = append(args, extraParams...)

	// run which to find the path of curl & jq (if available)
	curlPath, err := which("curl")
	if err != nil {
		return nil, err
	}
	jqPath, _ := which("jq")

	// check whether the output is TTY or a file
	isTTY := term.IsTerminal(int(os.Stdout.Fd()))

	// create mew struct
	gc := GoCurl{
		cloudantURL: cloudantURL,
		curlArgs:    args,
		jqArgs:      []string{"."},
		curlPath:    curlPath,
		jqPath:      jqPath,
		tmpFilename: tmpFilename,
		userAgent:   "gocurl/dev",
		isTTY:       isTTY,
	}

	return &gc, nil
}

func (gc *GoCurl) Run() error {

	// Process A - curl
	var curl, jq *exec.Cmd
	curl = exec.Command(gc.curlPath, gc.curlArgs...)

	if gc.isTTY && len(gc.jqPath) > 0 {
		jq = exec.Command(gc.jqPath, ".")
		pipe, err := curl.StdoutPipe()
		if err != nil {
			panic(err)
		}
		jq.Stdin = pipe
		jq.Stdout = os.Stdout
		err = jq.Start()
		if err != nil {
			panic(err)
		}
	} else {
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
	if gc.isTTY && len(gc.jqPath) > 0 {
		err = jq.Wait()
		if err != nil {
			return err
		}
	}
	return nil
}

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
