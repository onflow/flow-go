package main

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/storer"
	"github.com/rs/zerolog"
)

type RepoInfo struct {
	CommitTime    time.Time
	CommitHash    string
	CommitTag     string
	CommitSubject string
	Branch        string
}

type Git struct {
	log zerolog.Logger
	dir string

	repo *git.Repository
}

func NewGit(log zerolog.Logger, dir string, url string) (*Git, error) {
	var repo *git.Repository
	if _, err := os.Stat(dir); errors.Is(err, os.ErrNotExist) {
		repo, err = git.PlainClone(dir, false, &git.CloneOptions{URL: url})
		if err != nil {
			return nil, fmt.Errorf("failed to clone %s into %s: %w", url, dir, err)
		}
	} else {
		repo, err = git.PlainOpen(dir)
		if err != nil {
			return nil, fmt.Errorf("failed to open %s: %w", dir, err)
		}
	}

	return &Git{
		log:  log,
		dir:  dir,
		repo: repo,
	}, nil
}

func (g *Git) GetRepoInfo() (*RepoInfo, error) {
	ref, err := g.repo.Head()
	if err != nil {
		return nil, fmt.Errorf("failed to get head: %w", err)
	}

	commit, err := g.repo.CommitObject(ref.Hash())
	if err != nil {
		return nil, fmt.Errorf("failed to get commit object for: %s: %w", ref.Hash().String(), err)
	}

	subject, _, _ := strings.Cut(commit.Message, "\n")
	subject = strings.TrimSpace(subject)

	iter, err := g.repo.Tags()
	if err != nil {
		return nil, fmt.Errorf("failed to get tags: %w", err)
	}

	var tag string
	err = iter.ForEach(func(tagRef *plumbing.Reference) error {
		if ref.Hash() == tagRef.Hash() {
			tag = tagRef.Name().Short()
			return storer.ErrStop
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to iterate over tags: %w", err)
	}

	return &RepoInfo{
		Branch:        ref.Name().Short(),
		CommitTag:     tag,
		CommitHash:    ref.Hash().String(),
		CommitSubject: subject,
		CommitTime:    commit.Author.When,
	}, nil
}

func MustGetRepoInfo(log zerolog.Logger, gitRepoURL string, gitRepoPath string) *RepoInfo {
	git, err := NewGit(log, gitRepoPath, gitRepoURL)
	if err != nil {
		log.Fatal().Err(err).Str("path", gitRepoPath).Msg("failed to clone/open git repo")
	}
	repoInfo, err := git.GetRepoInfo()
	if err != nil {
		log.Fatal().Err(err).Str("path", gitRepoPath).Msg("failed to get repo info")
	}
	log.Info().Interface("repoInfo", repoInfo).Msg("parsed repo info")
	return repoInfo
}
