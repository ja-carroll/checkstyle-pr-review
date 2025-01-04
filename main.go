package main

import (
	"checkstyle-review/checkstylexml"
	"checkstyle-review/env"
	"checkstyle-review/github"
	"checkstyle-review/runner"
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	githubservice "github.com/google/go-github/v64/github"
	"github.com/google/uuid"
	"golang.org/x/oauth2"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
)

type option struct {
	path string
}

var opt = &option{}

func init() {
	flag.StringVar(&opt.path, "xmlPath", "", "checkstyle xml doc path")
}

func main() {
	flag.Parse()
	// Assume fixed relative path and open main.xml
	fmt.Printf("Running the checkstyke pre review tool\n")
	open, err := os.Open("build/reports/checkstyle/main.xml")
	if err != nil {
		fmt.Printf("open file error: %v\n", err)
		os.Exit(1)
	}
	if err := run(open); err != nil {
		fmt.Printf("checkstyle review error: %v\n", err)
		os.Exit(1)
	}
}

func run(inputCheckStyle io.Reader) error {
	ctx := context.Background()
	checkStyleParser := &checkstylexml.CheckStyleXML{}

	parseResult, err := checkStyleParser.Parse(inputCheckStyle)

	var errorMap = make(map[string][]*checkstylexml.CheckStyleErrorFormat)
	for _, fileResult := range parseResult.Files {
		errorMap[fileResult.Name] = make([]*checkstylexml.CheckStyleErrorFormat, 0)
		for _, errorFormat := range fileResult.Errors {
			k, _ := uuid.NewRandom()
			newFormat := checkstylexml.CheckStyleErrorFormat{
				ErrKey:   k,
				File:     fileResult.Name,
				Column:   errorFormat.Column,
				Line:     errorFormat.Line,
				Message:  errorFormat.Message,
				Severity: errorFormat.Severity,
				Source:   errorFormat.Source,
			}

			errorMap[fileResult.Name] = append(errorMap[fileResult.Name], &newFormat)

		}
	}

	if err != nil {
		return err
	}

	var ds *github.PullRequest

	gs, isPR, err := githubService(ctx)
	if err != nil {
		return err
	}
	if !isPR {
		_, err := fmt.Fprintln(os.Stderr, "This is not PullRequest build.")
		if err != nil {
			return err
		}
		return nil
	}
	ds = gs

	fmt.Printf("Running checkstyle: %d\n", len(errorMap))
	return runner.Run(ctx, ds, errorMap)

}

func githubService(ctx context.Context) (gs *github.PullRequest, isPR bool, err error) {
	g, client, err := githubBuildInfoWithClient(ctx)
	if err != nil {
		return nil, false, err
	}
	if g.PullRequest == 0 {

		if g.Branch == "" && g.SHA == "" {
			return nil, false, nil
		}

		prID, err := getPullRequestIDByBranchOrCommit(ctx, client, g)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return nil, false, nil
		}
		g.PullRequest = prID
	}

	gs, err = github.NewGitHubPullRequest(client, g.Owner, g.Repo, g.PullRequest, g.SHA)
	if err != nil {
		return nil, false, err
	}
	return gs, true, nil
}

func githubBuildInfoWithClient(ctx context.Context) (*env.BuildInfo, *githubservice.Client, error) {
	token, err := nonEmptyEnv("CHECKSTYLE_GITHUB_API_TOKEN")
	if err != nil {
		return nil, nil, err
	}
	g, _, err := env.GetBuildInfo()
	if err != nil {
		return nil, nil, err
	}
	client, err := githubClient(ctx, token)
	if err != nil {
		return nil, nil, err
	}
	return g, client, nil
}

func getPullRequestIDByBranchOrCommit(ctx context.Context, client *githubservice.Client, info *env.BuildInfo) (int, error) {
	options := &githubservice.SearchOptions{
		Sort:  "updated",
		Order: "desc",
	}

	query := []string{
		"type:pr",
		"state:open",
		fmt.Sprintf("repo:%s/%s", info.Owner, info.Repo),
	}
	if info.Branch != "" {
		query = append(query, fmt.Sprintf("head:%s", info.Branch))
	}
	if info.SHA != "" {
		query = append(query, info.SHA)
	}

	preparedQuery := strings.Join(query, " ")
	pullRequests, _, err := client.Search.Issues(ctx, preparedQuery, options)
	if err != nil {
		return 0, err
	}

	if *pullRequests.Total == 0 {
		return 0, fmt.Errorf("PullRequest not found, query: %s", preparedQuery)
	}

	return *pullRequests.Issues[0].Number, nil
}

func githubClient(ctx context.Context, token string) (*githubservice.Client, error) {
	ctx = context.WithValue(ctx, oauth2.HTTPClient, newHTTPClient())
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(ctx, ts)
	client := githubservice.NewClient(tc)
	var err error
	client.BaseURL, err = githubBaseURL()
	return client, err
}

const defaultGitHubAPI = "https://api.github.com/"

func githubBaseURL() (*url.URL, error) {
	if baseURL := os.Getenv("GITHUB_API"); baseURL != "" {
		u, err := url.Parse(baseURL)
		if err != nil {
			return nil, fmt.Errorf("GitHub base URL from GITHUB_API is invalid: %v, %w", baseURL, err)
		}
		return u, nil
	}
	// get GitHub base URL from GitHub Actions' default environment variable GITHUB_API_URL
	// ref: https://docs.github.com/en/actions/reference/environment-variables#default-environment-variables
	if baseURL := os.Getenv("GITHUB_API_URL"); baseURL != "" {
		u, err := url.Parse(baseURL + "/")
		if err != nil {
			return nil, fmt.Errorf("GitHub base URL from GITHUB_API_URL is invalid: %v, %w", baseURL, err)
		}
		return u, nil
	}
	u, err := url.Parse(defaultGitHubAPI)
	if err != nil {
		return nil, fmt.Errorf("GitHub base URL from default is invalid: %v, %w", defaultGitHubAPI, err)
	}
	return u, nil
}

func nonEmptyEnv(env string) (string, error) {
	value := os.Getenv(env)
	if value == "" {
		return "", fmt.Errorf("environment variable $%v is not set", env)
	}
	return value, nil
}

func newHTTPClient() *http.Client {
	tr := &http.Transport{
		Proxy:           http.ProxyFromEnvironment,
		TLSClientConfig: &tls.Config{InsecureSkipVerify: false},
	}
	return &http.Client{Transport: tr}
}
