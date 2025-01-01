package github

import (
	"context"
	"fmt"
	"github.com/google/go-github/v64/github"
	"log"
	"net/http"
	"os/exec"
)

// PullRequestDiffService is a DiffService which uses GitHub Diff API.
type PullRequestDiffService struct {
	Cli              *github.Client
	Owner            string
	Repo             string
	PR               int
	SHA              string
	FallBackToGitCLI bool
}

// Diff returns a diff of PullRequest.
func (p *PullRequest) Diff(ctx context.Context) ([]byte, error) {
	opt := github.RawOptions{Type: github.Diff}
	d, resp, err := p.cli.PullRequests.GetRaw(ctx, p.owner, p.repo, p.pr, opt)
	if err != nil {
		if resp != nil && p.FallBackToGitCLI && resp.StatusCode == http.StatusNotAcceptable {
			log.Print("fallback to use git command")
			return p.diffUsingGitCommand(ctx)
		}

		return nil, err
	}
	return []byte(d), nil
}

// diffUsingGitCommand returns a diff of PullRequest using git command.
func (p *PullRequest) diffUsingGitCommand(ctx context.Context) ([]byte, error) {
	pr, _, err := p.cli.PullRequests.Get(ctx, p.owner, p.repo, p.pr)
	if err != nil {
		return nil, err
	}

	head := pr.GetHead()
	headSha := head.GetSHA()

	commitsComparison, _, err := p.cli.Repositories.CompareCommits(ctx, p.owner, p.repo, headSha, pr.GetBase().GetSHA(), nil)
	if err != nil {
		return nil, err
	}

	mergeBaseSha := commitsComparison.GetMergeBaseCommit().GetSHA()

	bytes, err := exec.Command("git", "diff", "--find-renames", mergeBaseSha, headSha).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to run git diff: %s\n%w", bytes, err)
	}

	return bytes, nil
}
