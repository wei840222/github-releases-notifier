package main

import (
	"context"
	"fmt"
	"time"

	"github.com/slack-go/slack"
)

// SlackSender has the hook to send slack notifications.
type SlackSender struct {
	Hook string
}

// Send a notification with a formatted message build from the repository.
func (s *SlackSender) Send(repository Repository) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return slack.PostWebhookContext(ctx, s.Hook, &slack.WebhookMessage{
		Username:  "GitHub Releases",
		IconEmoji: ":github:",
		Blocks: &slack.Blocks{
			BlockSet: []slack.Block{
				&slack.SectionBlock{
					Type: slack.MBTSection,
					Text: &slack.TextBlockObject{
						Type: slack.MarkdownType,
						Text: fmt.Sprintf(
							"*Repo:* <%s|%s/%s>\n*Tag:* <%s|%s>",
							repository.URL.String(),
							repository.Owner,
							repository.Name,
							repository.Release.URL.String(),
							repository.Release.Tag,
						),
					},
				},
				&slack.DividerBlock{
					Type: slack.MBTDivider,
				},
				&slack.SectionBlock{
					Type: slack.MBTSection,
					Text: &slack.TextBlockObject{
						Type: slack.MarkdownType,
						Text: fmt.Sprintf("*%s*\n```%s```", repository.Release.Name, repository.Release.Description),
					},
				},
			},
		},
	})
}
