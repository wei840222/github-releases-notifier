package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"
	"strings"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	githubql "github.com/shurcooL/githubql"
)

// Release of a repository tagged via GitHub.
type Release struct {
	ID          string
	Name        string
	Description string
	Tag         string
	URL         url.URL
	PublishedAt time.Time
}

// IsReleaseCandidate returns true if the release name hints at an RC release.
func (r Release) IsReleaseCandidate() bool {
	return strings.Contains(strings.ToLower(r.Name), "-rc")
}

// IsBeta returns true if the release name hints at a beta version release.
func (r Release) IsBeta() bool {
	return strings.Contains(strings.ToLower(r.Name), "beta")
}

// IsNonstable returns true if one of the non-stable release-checking functions return true.
func (r Release) IsNonstable() bool {
	return r.IsReleaseCandidate() || r.IsBeta()
}

// Repository on GitHub.
type Repository struct {
	ID          string
	Name        string
	Owner       string
	Description string
	URL         url.URL
	Release     Release
}

// Checker has a githubql client to run queries and also knows about
// the current repositories releases to compare against.
type Checker struct {
	logger          log.Logger
	client          *githubql.Client
	persistenceFile string
	releases        map[string]Repository
}

// Run the queries and comparisons for the given repositories in a given interval.
func (c *Checker) Run(interval time.Duration, repositories []string, releases chan<- Repository) {
	if c.releases == nil {
		c.releases = make(map[string]Repository)
	}
	if c.persistenceFile != "" {
		c.loadPersistenceFile()
	}

	for {
		for _, repoName := range repositories {
			s := strings.Split(repoName, "/")
			owner, name := s[0], s[1]

			nextRepo, err := c.query(owner, name)
			if err != nil {
				level.Warn(c.logger).Log(
					"msg", "failed to query the repository's releases",
					"owner", owner,
					"name", name,
					"err", err,
				)
				continue
			}

			// For debugging uncomment this next line
			//releases <- nextRepo

			currRepo, ok := c.releases[repoName]

			// We've queried the repository for the first time.
			// Saving the current state to compare with the next iteration.
			if !ok {
				c.releases[repoName] = nextRepo
				continue
			}

			if nextRepo.Release.PublishedAt.After(currRepo.Release.PublishedAt) {
				releases <- nextRepo
				c.releases[repoName] = nextRepo
			} else {
				level.Debug(c.logger).Log(
					"msg", "no new release for repository",
					"owner", owner,
					"name", name,
				)
			}
		}

		if c.persistenceFile != "" {
			b, _ := json.Marshal(c.releases)
			if err := ioutil.WriteFile(c.persistenceFile, b, 0644); err != nil {
				level.Warn(c.logger).Log("msg", "write persistence file error", "error", err)
			}
		}

		time.Sleep(interval)
	}
}

// This should be improved in the future to make batch requests for all watched repositories at once
// TODO: https://github.com/shurcooL/githubql/issues/17

func (c *Checker) query(owner, name string) (Repository, error) {
	var query struct {
		Repository struct {
			ID          githubql.ID
			Name        githubql.String
			Description githubql.String
			URL         githubql.URI

			Releases struct {
				Edges []struct {
					Node struct {
						ID          githubql.ID
						Name        githubql.String
						Description githubql.String
						URL         githubql.URI
						PublishedAt githubql.DateTime
					}
				}
			} `graphql:"releases(last: 1)"`
		} `graphql:"repository(owner: $owner, name: $name)"`
	}

	variables := map[string]interface{}{
		"owner": githubql.String(owner),
		"name":  githubql.String(name),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := c.client.Query(ctx, &query, variables); err != nil {
		return Repository{}, err
	}

	repositoryID, ok := query.Repository.ID.(string)
	if !ok {
		return Repository{}, fmt.Errorf("can't convert repository id to string: %v", query.Repository.ID)
	}

	if len(query.Repository.Releases.Edges) == 0 {
		return Repository{}, fmt.Errorf("can't find any releases for %s/%s", owner, name)
	}
	latestRelease := query.Repository.Releases.Edges[0].Node

	releaseID, ok := latestRelease.ID.(string)
	if !ok {
		return Repository{}, fmt.Errorf("can't convert release id to string: %v", query.Repository.ID)
	}

	return Repository{
		ID:          repositoryID,
		Name:        string(query.Repository.Name),
		Owner:       owner,
		Description: string(query.Repository.Description),
		URL:         *query.Repository.URL.URL,

		Release: Release{
			ID:          releaseID,
			Name:        string(latestRelease.Name),
			Description: string(latestRelease.Description),
			Tag:         strings.Split(latestRelease.URL.URL.Path, "/")[len(strings.Split(latestRelease.URL.URL.Path, "/"))-1],
			URL:         *latestRelease.URL.URL,
			PublishedAt: latestRelease.PublishedAt.Time,
		},
	}, nil
}

func (c *Checker) loadPersistenceFile() {
	b, err := ioutil.ReadFile(c.persistenceFile)
	if err != nil {
		level.Warn(c.logger).Log("msg", "load persistence file error", "error", err)
		return
	}
	if err := json.Unmarshal(b, &c.releases); err != nil {
		level.Warn(c.logger).Log("msg", "unmarshal persistence file error", "error", err)
	}
}
