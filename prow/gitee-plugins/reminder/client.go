package reminder

type giteeClient interface {
	AddIssueLabel(owner, repo, number, label string) error
	CreateGiteeIssueComment(owner, repo, number, comment string) error
	AddMultiIssueLabel(org, repo, number string, label []string) error
	BotName() (string, error)
}

type ghclient struct {
	giteeClient
}

func (c *ghclient) AddLabel(org, repo, number, label string) error {
	return c.AddIssueLabel(org, repo, number, label)
}

func (c *ghclient) AddLabels(org, repo, number string, labels []string) error {
	return c.AddMultiIssueLabel(org, repo, number, labels)
}

func (c *ghclient) CreateComment(owner, repo, number, comment string) error {
	return c.CreateGiteeIssueComment(owner, repo, number, comment)
}

func (c *ghclient) getBotName() (string, error) {
	return c.BotName()
}
