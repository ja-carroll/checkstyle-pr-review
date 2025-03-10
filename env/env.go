package env

import (
	"encoding/json"
	"errors"
	"os"
)

// BuildInfo represents build information about GitHub or GitLab project.
type BuildInfo struct {
	Owner string
	Repo  string
	SHA   string

	// Optional.
	PullRequest int // MergeRequest for GitLab.

	// Optional.
	Branch string
}

// GetBuildInfo returns BuildInfo from environment variables.
func GetBuildInfo() (prInfo *BuildInfo, isPR bool, err error) {
	return getBuildInfoFromGitHubAction()

}

// https://docs.github.com/en/actions/reference/environment-variables#default-environment-variables
type GitHubEvent struct {
	PullRequest GitHubPullRequest `json:"pull_request"`
	Repository  struct {
		Owner struct {
			Login string `json:"login"`
		} `json:"owner"`
		Name string `json:"name"`
	} `json:"repository"`
	CheckSuite struct {
		After        string              `json:"after"`
		PullRequests []GitHubPullRequest `json:"pull_requests"`
	} `json:"check_suite"`
	HeadCommit struct {
		ID string `json:"id"`
	} `json:"head_commit"`
	ActionName string `json:"-"` // this is defined as env GITHUB_EVENT_NAME
}

type GitHubRepo struct {
	Owner struct {
		ID int64 `json:"id"`
	}
}

type GitHubPullRequest struct {
	Number int `json:"number"`
	Head   struct {
		Sha  string     `json:"sha"`
		Ref  string     `json:"ref"`
		Repo GitHubRepo `json:"repo"`
	} `json:"head"`
	Base struct {
		Repo GitHubRepo `json:"repo"`
	} `json:"base"`
}

func loadGitHubEventFromPath(eventPath string) (*GitHubEvent, error) {
	f, err := os.Open(eventPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var event GitHubEvent
	if err := json.NewDecoder(f).Decode(&event); err != nil {
		return nil, err
	}
	event.ActionName = os.Getenv("GITHUB_EVENT_NAME")
	return &event, nil
}

func getBuildInfoFromGitHubAction() (*BuildInfo, bool, error) {
	eventPath := os.Getenv("GITHUB_EVENT_PATH")
	if eventPath == "" {
		return nil, false, errors.New("GITHUB_EVENT_PATH not found")
	}
	return getBuildInfoFromGitHubActionEventPath(eventPath)
}

func getBuildInfoFromGitHubActionEventPath(eventPath string) (*BuildInfo, bool, error) {
	event, err := loadGitHubEventFromPath(eventPath)
	if err != nil {
		return nil, false, err
	}
	info := &BuildInfo{
		Owner:       event.Repository.Owner.Login,
		Repo:        event.Repository.Name,
		PullRequest: event.PullRequest.Number,
		Branch:      event.PullRequest.Head.Ref,
		SHA:         event.PullRequest.Head.Sha,
	}
	// For re-run check_suite event.
	if info.PullRequest == 0 && len(event.CheckSuite.PullRequests) > 0 {
		pr := event.CheckSuite.PullRequests[0]
		info.PullRequest = pr.Number
		info.Branch = pr.Head.Ref
		info.SHA = pr.Head.Sha
	}
	if info.SHA == "" {
		info.SHA = event.HeadCommit.ID
	}
	if info.SHA == "" {
		info.SHA = os.Getenv("GITHUB_SHA")
	}
	return info, info.PullRequest != 0, nil
}
