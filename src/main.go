package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/google/go-github/github"
	log "github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
	"gopkg.in/src-d/go-git.v4"
	"os"
	"os/exec"
	"time"
)

type GithubReleaseInfo struct {
	ProjectOwner string                    `json:"owner"`
	Repository   string                    `json:"repository"`
	AccessToken  string                    `json:"access_token"`
	ReleaseInfo  *github.RepositoryRelease `json:"repository_release"`
}

func GetEnv(name string) string {
	val, found := os.LookupEnv(name)
	if found {
		return val
	}

	return ""
}

func MustGetEnv(name string) string {
	val := GetEnv(name)
	if val == "" {
		log.Fatal("Couldn't get env ", name)
	}

	return val
}

func retry(attempts int, sleep time.Duration, f func() error) (err error) {
	for i := 0; ; i++ {
		err = f()
		if err == nil {
			return
		}

		if i >= (attempts - 1) {
			break
		}

		time.Sleep(sleep)

		log.Info("retrying after error:", err)
	}
	return fmt.Errorf("after %d attempts, last error: %s", attempts, err)
}

func buildDockerImage(service string, name string, useCache bool) error {
	useCacheStr := ""
	if !useCache {
		useCacheStr = "--no-cache"
	}
	cmd := exec.Command("docker", "build", "-t", name, useCacheStr, "-f", "env/docker/Dockerfile."+service, ".")
	cmd.Dir = "/tmp/source/"
	cmd.Env = os.Environ()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Start()
	if err != nil {
		return err
	}

	// Wait for the process to finish
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()
	select {
	case err := <-done:
		return err
	}
}

func tagDockerImage(old string, new string) error {
	// Start a process:
	cmd := exec.Command("docker", "tag", old, new)
	cmd.Env = os.Environ()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Start()
	if err != nil {
		return err
	}

	// Wait for the process to finish
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()
	select {
	case err := <-done:
		return err
	}
}

func pushDockerImage(name string) error {
	// Start a process:
	cmd := exec.Command("docker", "push", name)
	cmd.Env = os.Environ()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Start()
	if err != nil {
		return err
	}

	// Wait for the process to finish
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()
	select {
	case err := <-done:
		return err
	}
}

func loginToDockerHub(username string, password string) error {
	c := `echo $DOCKER_PASSWORD | docker login --username $DOCKER_USER --password-stdin`
	cmd := exec.Command("bash", "-c", c)
	cmd.Env = os.Environ()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Start()
	if err != nil {
		return err
	}

	// Wait for the process to finish
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()
	select {
	case err := <-done:
		return err
	}
}

func createGithubRelease(githubReleaseInfo GithubReleaseInfo) error {
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: githubReleaseInfo.AccessToken},
	)
	tc := oauth2.NewClient(ctx, ts)

	client := github.NewClient(tc)

	_, _, err := client.Repositories.CreateRelease(ctx, githubReleaseInfo.ProjectOwner,
		githubReleaseInfo.Repository, githubReleaseInfo.ReleaseInfo)
	return err
}

func main() {
	version := flag.String("version", "", "Version")

	flag.Parse()

	ver := *version
	if *version == "" {
		ver = GetEnv("IMAGEMONKEY_VERSION")
		if ver == "" {
			log.Fatal("Please provide a valid version")
		}
	}

	log.Info("ImageMonkey-Releaser started")

	log.Info("Connecting to DockerHub...")
	dockerUser := MustGetEnv("DOCKER_USER")
	dockerPassword := MustGetEnv("DOCKER_PASSWORD")
	err := loginToDockerHub(dockerUser, dockerPassword)
	if err != nil {
		log.Fatal("Couldn't connect to DockerHub: ", err.Error())
	}

	sourceCodeDir := "/tmp/source"
	if _, err := os.Stat(sourceCodeDir); os.IsNotExist(err) {
		log.Info("Checking out sourcecode to ", sourceCodeDir, "...")
		_, err = git.PlainClone(sourceCodeDir, false, &git.CloneOptions{
			URL:      MustGetEnv("GITHUB_REPOSITORY_URL"),
			Progress: os.Stdout,
		})

		if err != nil {
			log.Fatal("Couldn't clone repository ", MustGetEnv("GITHUB_REPOSITORY_URL"), ": ", err.Error())
			err = os.RemoveAll(sourceCodeDir)
			if err != nil {
				log.Fatal("Couldn't remove ", sourceCodeDir, ": ", err.Error())
			}
		}
	} else {
		log.Info("Repository already exists..use this one")
	}

	services := map[string]string{
		"api":                    "api",
		"web":                    "web",
		"statworker":             "statworker",
		"blogsubscriptionworker": "blogsubscriptionworker",
		"trendinglabelsworker":   "trendinglabelsworker",
		"dataprocessor":          "dataprocessor",
		"postgres":               "db",
		"testing":                "testing",
		"pgbouncer":              "pgbouncer",
	}

	cnt := 1
	for service, name := range services {
		log.Info("[", cnt, "/", len(services), "] Building ", service, " docker image")
		imageTagLatest := dockerUser + "/imagemonkey-" + name + ":latest"
		imageTagVersion := dockerUser + "/imagemonkey-" + name + ":" + ver

		err = retry(5, 10*time.Second, func() (err error) {
			err = buildDockerImage(service, imageTagLatest, false)
			return
		})
		if err != nil {
			log.Fatal("Couldn't build ", service, ": ", err.Error())
		}

		err = retry(2, 2*time.Second, func() (err error) {
			err = tagDockerImage(imageTagLatest, imageTagVersion)
			return
		})
		if err != nil {
			log.Fatal("Couldn't tag ", imageTagVersion, " docker image: ", err.Error())
		}

		err = retry(10, 2*time.Second, func() (err error) {
			err = pushDockerImage(imageTagLatest)
			return
		})
		if err != nil {
			log.Fatal("Couldn't push ", imageTagLatest, " docker image: ", err.Error())
		}

		err = retry(2, 2*time.Second, func() (err error) {
			err = pushDockerImage(imageTagVersion)
			return
		})
		if err != nil {
			log.Fatal("Couldn't push ", imageTagVersion, " docker image: ", err.Error())
		}

		cnt++
	}

	log.Info("Connecting with Github...")

	var githubReleaseInfo GithubReleaseInfo
	githubReleaseInfo.AccessToken = MustGetEnv("GITHUB_ACCESS_TOKEN")
	githubReleaseInfo.ProjectOwner = MustGetEnv("GITHUB_PROJECT_OWNER")
	githubReleaseInfo.Repository = MustGetEnv("GITHUB_REPOSITORY")

	detailedReleaseInfo := "# Docker Images\n\n"
	for _, name := range services {
		detailedReleaseInfo += "[" + "imagemonkey-" + name + "]" + "(https://hub.docker.com/r/" + dockerUser + "/imagemonkey-" + name + ")"
		detailedReleaseInfo += "\n"
	}

	repoRelease := &github.RepositoryRelease{
		Name:    github.String("v" + ver),
		Body:    github.String(detailedReleaseInfo),
		TagName: github.String("v" + ver),
	}

	githubReleaseInfo.ReleaseInfo = repoRelease

	err = createGithubRelease(githubReleaseInfo)
	if err != nil {
		log.Fatal("Couldn't create github release: ", err.Error())
	}
}
