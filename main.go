package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"
)

var (
	addr            = os.Getenv("ADDR")
	timeoutDuration = 30 * time.Second
)

func init() {
	if addr == "" {
		addr = ":8080"
	}
}

func handleErr(err error, code int, w http.ResponseWriter) bool {
	if err != nil {
		w.WriteHeader(code)
		w.Write([]byte(err.Error()))
		return false
	}
	return true
}

func pathToRepo(urlPath string) (string, string, error) {
	split := strings.Split(urlPath[1:], "/")

	if len(split) == 0 {
		return "", "", fmt.Errorf("invalid git repository")
	}

	switch split[0] {
	case "github":
		fallthrough
	case "github.com":
		if len(split) < 3 {
			return "", "", fmt.Errorf("could not parse github repo")
		}
		return fmt.Sprintf("https://github.com/%s/%s", split[1], split[2]), strings.Join(split[3:], "/"), nil
	}

	return "", "", fmt.Errorf("unsupported git host")
}

func handler(w http.ResponseWriter, req *http.Request) {
	start := time.Now()
	defer func() {
		log.Printf("GET %s finished in %s", req.URL.String(), time.Now().Sub(start))
	}()

	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET")
	w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")

	if req.Method == "OPTIONS" {
		return
	}

	ctx, close := context.WithTimeout(req.Context(), timeoutDuration)
	defer close()

	repo, filePath, err := pathToRepo(req.URL.EscapedPath())
	if ok := handleErr(err, http.StatusUnprocessableEntity, w); !ok {
		return
	}

	dir, err := ioutil.TempDir("", "repo-*")
	if ok := handleErr(err, http.StatusInternalServerError, w); !ok {
		return
	}
	defer os.RemoveAll(dir)

	gitPath, err := exec.LookPath("git")
	if ok := handleErr(err, http.StatusInternalServerError, w); !ok {
		return
	}

	// TODO log the output
	err = exec.CommandContext(ctx, gitPath, "clone", "--filter=blob:none", "--depth=1", "--sparse", repo, dir).Run()
	if ok := handleErr(err, http.StatusInternalServerError, w); !ok {
		return
	}

	cmd := exec.CommandContext(ctx, gitPath, "sparse-checkout", "init", "--cone")
	cmd.Dir = dir

	err = cmd.Run()
	if ok := handleErr(err, http.StatusInternalServerError, w); !ok {
		return
	}

	cmd = exec.CommandContext(ctx, gitPath, "ls-tree", "HEAD", filePath)
	cmd.Dir = dir

	out, err := cmd.Output()
	if ok := handleErr(err, http.StatusInternalServerError, w); !ok {
		return
	}

	str := string(out)

	count := strings.Count(str, "\n")

	if count != 1 {
		handleErr(fmt.Errorf("not found"), http.StatusNotFound, w)
		return
	}

	if strings.Split(str, " ")[1] != "blob" {
		handleErr(fmt.Errorf("must be a blob"), http.StatusInternalServerError, w)
		return
	}

	cmd = exec.CommandContext(ctx, gitPath, "sparse-checkout", "set", filePath)
	cmd.Dir = dir

	err = cmd.Run()
	if ok := handleErr(err, http.StatusInternalServerError, w); !ok {
		return
	}

	fullFilePath := path.Join(dir, filePath)

	f, err := os.Open(fullFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			handleErr(err, http.StatusNotFound, w)
			return
		}
	}
	if ok := handleErr(err, http.StatusInternalServerError, w); !ok {
		return
	}

	http.ServeContent(w, req, fullFilePath, time.Now(), f)

	err = f.Close()
	if ok := handleErr(err, http.StatusInternalServerError, w); !ok {
		return
	}
}

func main() {
	http.HandleFunc("/", handler)
	err := http.ListenAndServe(addr, nil)
	if err != nil {
		log.Fatal(err)
	}
}
