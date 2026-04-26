package report

import (
	"fmt"
	"os/exec"
	"strings"
)

const (
	RepoOwner    = "rengotaku"
	RepoName     = "claude-usage-tracker"
	DiscussionNum = 34
)

// GetDiscussionID resolves the GraphQL node ID for a GitHub discussion number.
func GetDiscussionID(owner, repo string, num int) (string, error) {
	query := fmt.Sprintf(
		`query { repository(owner: "%s", name: "%s") { discussion(number: %d) { id } } }`,
		owner, repo, num,
	)
	out, err := ghRun("api", "graphql", "-f", "query="+query, "--jq", ".data.repository.discussion.id")
	if err != nil {
		return "", err
	}
	id := strings.TrimSpace(out)
	if id == "" || id == "null" {
		return "", fmt.Errorf("discussion #%d not found in %s/%s", num, owner, repo)
	}
	return id, nil
}

// PostComment adds a comment to a GitHub discussion and returns the comment URL.
func PostComment(discussionID, body string) (string, error) {
	mutation := `mutation($id: ID!, $body: String!) { addDiscussionComment(input: {discussionId: $id, body: $body}) { comment { url } } }`
	out, err := ghRun(
		"api", "graphql",
		"-f", "query="+mutation,
		"-f", "id="+discussionID,
		"-f", "body="+body,
		"--jq", ".data.addDiscussionComment.comment.url",
	)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func ghRun(args ...string) (string, error) {
	cmd := exec.Command("gh", args...)
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("gh %s: %s", args[0], strings.TrimSpace(string(ee.Stderr)))
		}
		return "", fmt.Errorf("gh %s: %w", args[0], err)
	}
	return string(out), nil
}
