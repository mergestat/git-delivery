package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"strings"
	"syscall"
	"time"
)

var (
	port            = os.Getenv("PORT")
	timeoutDuration = 30 * time.Second
)

func init() {
	if port == "" {
		port = ":8080"
	} else {
		port = fmt.Sprintf(":%s", port)
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

func parseRepo(req *http.Request) (*url.URL, string, error) {
	urlPath := req.URL.EscapedPath()
	split := strings.Split(urlPath[1:], "/")

	if len(split) == 0 {
		return nil, "", fmt.Errorf("invalid git repository")
	}

	switch split[0] {
	case "github":
		split[0] = "github.com"
		break
	case "gitlab":
		split[0] = "gitlab.com"
	}

	if len(split) < 4 {
		return nil, "", fmt.Errorf("could not parse path")
	}

	o, err := url.Parse(fmt.Sprintf("https://%s/%s/%s", split[0], split[1], split[2]))
	if err != nil {
		return nil, "", err
	}

	if user, pass, hasAuth := req.BasicAuth(); hasAuth {
		o.User = url.UserPassword(user, pass)
	}

	return o, strings.Join(split[3:], "/"), nil
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

	repo, filePath, err := parseRepo(req)
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
	err = exec.CommandContext(ctx, gitPath, "clone", "--filter=blob:none", "--depth=1", "--sparse", repo.String(), dir).Run()
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

	srv := &http.Server{Addr: port}

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()
	log.Printf("starting git-delivery HTTP server on %s\n", port)

	<-done

	log.Println("shutting down HTTP server")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("failed to shutdown HTTP server:%+v", err)
	}
}
