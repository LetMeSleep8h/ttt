package app

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/eugenioenko/ttt/internal/git"
	"github.com/eugenioenko/ttt/internal/github"
	"github.com/eugenioenko/ttt/internal/workspace"
)

var githubShorthand = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*/[A-Za-z0-9_.-]+$`)

// owner/repo goes through gh when it is installed, so private repos use the
// credentials gh already has.
func cloneSource(input string, haveGH bool) (repo, name string, viaGH bool) {
	repo = strings.TrimSpace(input)
	shorthand := githubShorthand.MatchString(repo)
	if shorthand && !haveGH {
		repo = "https://github.com/" + repo
	}
	name = strings.TrimSuffix(strings.TrimRight(repo, "/"), ".git")
	if i := strings.LastIndexAny(name, "/:"); i >= 0 {
		name = name[i+1:]
	}
	return repo, name, shorthand && haveGH
}

func (a *App) CloneRepository() {
	a.ShowInputDialogEx("Clone Repository", "URL or owner/repo", "", "Next", func(input string) {
		if strings.TrimSpace(input) == "" {
			return
		}
		_, ghErr := exec.LookPath("gh")
		repo, name, viaGH := cloneSource(input, ghErr == nil)
		if name == "" {
			a.StatusError("Cannot tell the folder name from " + input)
			return
		}
		a.ShowFolderPicker("Clone Into", "Clone", "", func(parent string) {
			parent, err := filepath.Abs(workspace.ExpandPath(parent))
			if err != nil {
				a.StatusError("Error: " + err.Error())
				return
			}
			dest := filepath.Join(parent, name)
			if _, err := os.Stat(dest); !errors.Is(err, os.ErrNotExist) {
				a.StatusError("Already exists: " + dest)
				return
			}
			clone := func(string) error { return git.Clone(repo, dest) }
			if viaGH {
				clone = func(string) error { return github.CloneRepo(repo, dest) }
			}
			a.RunRepoTask(RepoTask{
				Progress: "Cloning " + name,
				Done:     "Cloned " + name,
				Dirs:     []string{parent},
				Ops:      []RepoOp{{Name: "clone", Run: clone}},
				OnDone:   func() { a.openFolderPath(dest) },
			})
		})
	})
}
