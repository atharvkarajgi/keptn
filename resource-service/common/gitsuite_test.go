package common

import (
	"errors"
	"fmt"
	"github.com/go-git/go-billy/v5/memfs"
	fixtures "github.com/go-git/go-git-fixtures/v4"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/storage/memory"
	common_mock "github.com/keptn/keptn/resource-service/common/fake"
	"github.com/keptn/keptn/resource-service/common_models"
	kerrors "github.com/keptn/keptn/resource-service/errors"
	. "gopkg.in/check.v1"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func Test(t *testing.T) { TestingT(t) }

type BaseSuite struct {
	Repository *git.Repository
	url        string
}

var _ = Suite(&BaseSuite{})

func (s *BaseSuite) SetUpSuite(c *C) {
	_ = os.Setenv("CONFIG_DIR", "../test/tmp")
	cleanupSuite(c)
	s.buildBasicRepository(c)
}

func (s *BaseSuite) TearDownSuite(c *C) {
	_ = os.Unsetenv("CONFIG_DIR")
	//cleanupSuite(c)
}

func (s *BaseSuite) SetUpTest(c *C) {
	s.SetUpSuite(c)
}

func (s *BaseSuite) buildBasicRepository(c *C) {

	s.url = "../test/tmp" + "/remote"

	//initBare(c,s.url)

	// make a local remote
	_, err := git.PlainClone(s.url, true, &git.CloneOptions{URL: "https://github.com/git-fixtures/basic.git"})
	c.Assert(err, IsNil)

	// make local git repo
	s.Repository, err = git.PlainClone("../test/tmp"+"/sockshop", false, &git.CloneOptions{URL: s.url})
	c.Assert(err, IsNil)
}

func cleanupSuite(c *C) {
	err := os.RemoveAll("./debug")
	c.Assert(err, IsNil)

	err = os.RemoveAll("../test/tmp/remote")
	c.Assert(err, IsNil)

	err = os.RemoveAll("../test/tmp/sockshop")
	c.Assert(err, IsNil)

	err = os.RemoveAll("../test/tmp/shared")
	c.Assert(err, IsNil)
	err = os.RemoveAll("../test/tmp/repo1")
	c.Assert(err, IsNil)
	err = os.RemoveAll("../test/tmp/repo2")
	c.Assert(err, IsNil)
}
func (s *BaseSuite) TestGit_ComponentTest(c *C) {

	g := Git{GogitReal{}}

	url := "../test/tmp" + "/shared"
	url, err := filepath.Abs(url)
	c.Assert(err, IsNil)

	//setup remote as bare

	remote, err := g.git.PlainInit(url, true)
	c.Assert(err, IsNil)
	configUser(remote)
	// make two local repo pointing at our remote

	repo1, err := git.PlainInit("../test/tmp/repo1", false)
	c.Assert(err, IsNil)
	configUser(repo1)
	repo2, err := git.PlainInit("../test/tmp/repo2", false)
	c.Assert(err, IsNil)
	configUser(repo2)

	_, err = repo1.CreateRemote(&config.RemoteConfig{
		Name: "origin",
		URLs: []string{url},
	})
	c.Assert(err, IsNil)

	_, err = repo2.CreateRemote(&config.RemoteConfig{
		Name: "origin",
		URLs: []string{url},
	})

	// push some first change to remote

	c.Assert(err, IsNil)
	w2, err := repo2.Worktree()
	c.Assert(err, IsNil)
	write("f.txt", "init repo", c, w2)
	commit("f.txt", c, w2)
	push(repo2, c)

	//add changes to repo1
	w1, err := repo1.Worktree()
	c.Assert(err, IsNil)
	//make sure repo1 is uptodate
	err = w1.Pull(&git.PullOptions{Force: true})
	c.Assert(err, IsNil)

	//add changes to repo1
	content1 := "my very important stuff"
	write("try.txt", content1, c, w1)
	h := commit("try.txt", c, w1)
	id := h.String()
	push(repo1, c)

	// check changes to repo
	repo1context := common_models.GitContext{Project: "repo1",
		Credentials: &common_models.GitCredentials{
			User:      "u2",
			Token:     "mytoken",
			RemoteURI: url,
		}}
	b, err := g.GetFileRevision(repo1context, id, "try.txt")
	c.Assert(err, IsNil)
	c.Assert(content1, Equals, string(b))

	// check remote already up to date
	err = w1.Pull(&git.PullOptions{})
	c.Assert(kerrors.NoErrAlreadyUpToDate.Is(err), Equals, true)

	//add conflicting changes to repo2
	content2 := "my stuff is complete"
	write("try.txt", content2, c, w2)
	// also adding  file from filesystem works
	//f, err := os.Create("../test/tmp/repo2/try.txt")
	//f.Write([]byte(fmt.Sprintf("my stuff is more important")))
	//f.Close()
	repo2context := common_models.GitContext{Project: "repo2",
		Credentials: &common_models.GitCredentials{
			User:      "u2",
			Token:     "mytoken",
			RemoteURI: url,
		}}

	// check new changes are forced
	id, err = g.StageAndCommitAll(repo2context, "my conflicting change")
	c.Assert(err, IsNil)
	c.Logf("my commit id %s", id)

	// check remote already up to date
	err = w2.Pull(&git.PullOptions{})
	c.Assert(kerrors.NoErrAlreadyUpToDate.Is(err), Equals, true)

	// because we keep their changes ours are not the final ones
	b, err = g.GetFileRevision(repo2context, id, "try.txt")
	c.Assert(err, IsNil)
	c.Assert(content1, Equals, string(b))

	//verify current revision
	curr, err := g.GetCurrentRevision(repo2context)
	c.Assert(curr, Equals, id)
}

func configUser(remote *git.Repository) {
	co, _ := remote.Config()
	co.User.Name = "keptn"
	co.User.Email = "keptn@keptn.sh"
	remote.SetConfig(co)
}

func (s *BaseSuite) TestGit_GetCurrentRevision(c *C) {

	tests := []struct {
		name       string
		git        Gogit
		gitContext common_models.GitContext
		doCommit   bool
		branch     string
		want       string
		err        *kerrors.ResourceServiceError
	}{
		{
			name:       "return master commit",
			git:        GogitReal{},
			gitContext: s.NewGitContext(),
			branch:     "master",
			want:       "6ecf0ef2c2dffb796033e5a02219af86ec6584e5",
			doCommit:   false,
		},
		{
			name:       "return branch commit",
			git:        GogitReal{},
			gitContext: s.NewGitContext(),
			branch:     "dev",
			want:       "",
			doCommit:   true,
		},
		{
			name: "return error",
			git:  GogitReal{},
			gitContext: common_models.GitContext{
				Project: "nope",
				Credentials: &common_models.GitCredentials{
					User:      "ssss",
					Token:     "bjh",
					RemoteURI: "an url that doesnot exists"},
			},
			branch:   "master",
			want:     "",
			err:      kerrors.ErrRepositoryNotExists,
			doCommit: false,
		},
	}

	for _, tt := range tests {
		c.Log("Test : " + tt.name)
		g := &Git{
			git: tt.git,
		}
		var id plumbing.Hash
		var err error

		if tt.err == nil {
			err = checkout(c, g, tt.gitContext, tt.branch)

			if tt.doCommit {
				w, err := s.Repository.Worktree()
				c.Assert(err, IsNil)
				err = write("something.txt", "something", c, w)
				c.Assert(err, IsNil)
				id = commit("something.txt", c, w)
			}
		}
		currId, err := g.GetCurrentRevision(tt.gitContext)

		if err != nil && !tt.err.Is(errors.Unwrap(err)) {
			c.Fatalf("Wanted %v but gotten %v", tt.err, errors.Unwrap(err))
		}
		if tt.doCommit {
			c.Assert(currId, Equals, id.String())
		} else {
			if currId != tt.want {
				c.Error(currId, tt.want)
			}
		}
	}
}

func (s *BaseSuite) TestGit_StageAndCommitAll(c *C) {

	tests := []struct {
		name       string
		gitContext common_models.GitContext
		message    string
		wantErr    bool
		doCommit   bool
	}{
		{
			name:       "commit  new file",
			gitContext: s.NewGitContext(),
			message:    "my commit",
			wantErr:    false,
			doCommit:   true,
		},
		{
			name:       " commit no new content",
			gitContext: s.NewGitContext(),
			message:    "my commit",
			wantErr:    false,
			doCommit:   false,
		},
	}
	for _, tt := range tests {
		c.Log("Test " + tt.name)
		g := Git{GogitReal{}}
		r := s.Repository

		//get current commit
		h, err := r.Head()
		c.Assert(err, IsNil)
		originalId := h.Hash().String()

		if tt.doCommit {
			w, err := r.Worktree()
			c.Assert(err, IsNil)
			write("foo/file.txt", "anycontent", c, w)
		}
		id, err := g.StageAndCommitAll(tt.gitContext, tt.message)
		if (err != nil) != tt.wantErr {
			c.Errorf("StageAndCommitAll() error = %v, wantErr %v", err, tt.wantErr)
		}
		if tt.doCommit {
			c.Assert(id, Not(Equals), "")
			s.checkCommit(c, r, id)
			// make sure there is a new commit
			c.Assert(originalId, Not(Equals), id)
			b, err := g.GetFileRevision(tt.gitContext, id, "foo/file.txt")
			c.Assert(err, IsNil)
			c.Assert("anycontent", Equals, string(b))
		}

	}
}

func (s *BaseSuite) checkCommit(c *C, r *git.Repository, id string) {
	head, err := r.Head()
	c.Assert(err, IsNil)
	// check local changes
	c.Assert(head.Hash().String(), Equals, id)
	//check remote changes
	newRepo, err := git.Clone(memory.NewStorage(), memfs.New(), &git.CloneOptions{URL: s.url})
	c.Assert(err, IsNil)
	nrh, err := newRepo.Head()
	c.Assert(err, IsNil)
	c.Assert(nrh.Hash().String(), Equals, id)
}

func (s *BaseSuite) TestGit_Push(c *C) {

	tests := []struct {
		name       string
		gitContext common_models.GitContext
		err        *kerrors.ResourceServiceError
		push       bool
	}{
		{
			name:       "push, no new changes",
			gitContext: s.NewGitContext(),
			push:       false,
		},
		{
			name:       "push, new changes",
			gitContext: s.NewGitContext(),
			push:       true,
		},
		{
			name: "push, invalid credentials",
			gitContext: common_models.GitContext{
				Project: "sockshop",
				Credentials: &common_models.GitCredentials{
					User:      "ssss",
					Token:     "bjh",
					RemoteURI: "https://github.com/git-fixtures/basic.git"},
			},
			err:  kerrors.ErrAuthenticationRequired,
			push: false,
		},
	}
	for _, tt := range tests {
		r := s.Repository
		var h plumbing.Hash
		if tt.push {
			w, err := r.Worktree()
			c.Assert(err, IsNil)
			err = write("fo/file.txt", "a content", c, w)
			c.Assert(err, IsNil)
			h = commit("fo/file.txt", c, w)
		}
		g := Git{GogitReal{}}
		err := g.Push(tt.gitContext)
		if err != nil && !errors.Is(tt.err, errors.Unwrap(err)) {
			c.Fatalf("Wanted %v but gotten %v", tt.err, errors.Unwrap(err))
		}
		if tt.push {
			s.checkCommit(c, r, h.String())
		}

	}
}

func (s *BaseSuite) TestGit_GetDefaultBranch(c *C) {

	tests := []struct {
		name       string
		gitContext common_models.GitContext
		want       string
		wantErr    bool
	}{
		{
			name:       "simple master",
			gitContext: s.NewGitContext(),
			want:       "master",
			wantErr:    false,
		},
	}
	for _, tt := range tests {
		g := Git{GogitReal{}}
		conf, err := s.Repository.Config()
		c.Assert(err, IsNil)
		conf.Init.DefaultBranch = tt.want
		s.Repository.SetConfig(conf)
		got, err := g.GetDefaultBranch(tt.gitContext)
		if (err != nil) != tt.wantErr {
			c.Errorf("GetDefaultBranch() error = %v, wantErr %v", err, tt.wantErr)
			return
		}
		if got != tt.want {
			c.Errorf("GetDefaultBranch() got = %v, exists %v", got, tt.want)
		}

	}
}

func (s *BaseSuite) TestGit_Pull(c *C) {

	tests := []struct {
		name       string
		gitContext common_models.GitContext
		expected   []string
		err        error
	}{
		{
			name:       "retrieve already uptodate sockshop",
			gitContext: s.NewGitContext(),
			expected: []string{"[core]\n" + "\tbare = false\n" +
				"[remote \"origin\"]\n" +
				"\turl = ../test/tmp/remote\n" +
				"\tfetch = +refs/heads/*:refs/remotes/origin/*\n" +
				"[branch \"master\"]\n" +
				"\tremote = origin\n" +
				"\tmerge = refs/heads/master\n"},
		},
		{
			name: "retrieve from unexisting project",
			gitContext: common_models.GitContext{
				Project: "mine",
				Credentials: &common_models.GitCredentials{
					User:      "ssss",
					Token:     "bjh",
					RemoteURI: s.url},
			},
			expected: []string{"[core]\n" +
				"\tbare = false\n",
				"[branch \"master\"]\n" +
					"\tremote = origin\n" +
					"\tmerge = refs/heads/master\n",
				"[user]\n" +
					"\tname = keptn\n" +
					"\temail = keptn@keptn.sh\n", "[remote \"origin\"]\n" +
					"\turl = ../test/tmp/remote\n" +
					"\tfetch = +refs/heads/*:refs/remotes/origin/*\n"},
		},
		{
			name: "retrieve from unexisting url",
			gitContext: common_models.GitContext{
				Project: "mine",
				Credentials: &common_models.GitCredentials{
					User:      "ssss",
					Token:     "bjh",
					RemoteURI: "jibberish"},
			},
			err: kerrors.ErrRepositoryNotFound,
		},
	}

	for _, tt := range tests {
		c.Logf("Test %s", tt.name)
		g := Git{GogitReal{}}
		err := g.Pull(tt.gitContext)
		if err != nil && !errors.Is(tt.err, errors.Unwrap(err)) {
			c.Fatalf("Wanted %v but gotten %v", tt.err, errors.Unwrap(err))
		}
		if err == nil {
			b, err := os.ReadFile(GetProjectConfigPath(tt.gitContext.Project + "/.git/config"))
			c.Assert(err, IsNil)
			for _, s := range tt.expected {
				c.Assert(strings.Contains(string(b), s), Equals, true)
			}
		}

	}
}

func (s *BaseSuite) TestGit_CloneRepo(c *C) {

	tests := []struct {
		name       string
		gitContext common_models.GitContext
		git        Gogit
		want       bool
		wantErr    bool
	}{
		{
			name: "clone sockshop from remote",
			git: &common_mock.GogitMock{
				PlainOpenFunc: func(path string) (*git.Repository, error) {
					return s.Repository, nil
				},
			},
			gitContext: s.NewGitContext(),
			wantErr:    false,
			want:       true,
		},
		{
			name: "clone existing sockshop",
			git: &common_mock.GogitMock{
				PlainOpenFunc: func(path string) (*git.Repository, error) {
					return s.Repository, nil
				},
			},
			gitContext: s.NewGitContext(),
			wantErr:    false,
			want:       true,
		},
		{
			name:       "empty context",
			gitContext: common_models.GitContext{},
			git:        GogitReal{},
			wantErr:    true,
			want:       false,
		},
		{ // TODO: do we worry here if url is not valid or while saving it?
			// go git seems to try to parse this wrong url
			name: "wrong url context",
			gitContext: common_models.GitContext{
				Project: "sockshop",
				Credentials: &common_models.GitCredentials{
					User:      "Me",
					Token:     "blabla",
					RemoteURI: "http//wrongurl"},
			},
			git: &common_mock.GogitMock{
				PlainCloneFunc: func(path string, isBare bool, o *git.CloneOptions) (*git.Repository, error) {
					return nil, errors.New("auth error")
				},
				PlainInitFunc: func(path string, isBare bool) (*git.Repository, error) {
					return nil, errors.New("not exists")
				},
				PlainOpenFunc: func(path string) (*git.Repository, error) {
					return nil, errors.New("not exists")
				},
			},
			wantErr: true,
			want:    false,
		},
		{
			name: "Wrong credential",
			gitContext: common_models.GitContext{
				Project: "so",
				Credentials: &common_models.GitCredentials{
					User:      "ssss",
					Token:     "bjh",
					RemoteURI: "https://github.com/git-fixtures/basic.git"},
			},
			git: &common_mock.GogitMock{
				PlainCloneFunc: func(path string, isBare bool, o *git.CloneOptions) (*git.Repository, error) {
					return nil, errors.New("auth error")
				},
				PlainInitFunc: func(path string, isBare bool) (*git.Repository, error) {
					return nil, errors.New("not exists")
				},
				PlainOpenFunc: func(path string) (*git.Repository, error) {
					return nil, errors.New("not exists")
				},
			},
			wantErr: true,
			want:    false,
		},
	}
	for _, tt := range tests {
		c.Log("Test ", tt.name)
		g := Git{tt.git}
		got, err := g.CloneRepo(tt.gitContext)
		if (err != nil) != tt.wantErr {
			c.Errorf("CloneRepo() error = %v, wantErr %v", err, tt.wantErr)

		}
		if got != tt.want {
			c.Errorf("CloneRepo() got = %v, exists %v", got, tt.want)
		}

	}
}

func (s *BaseSuite) TestGit_CreateBranch(c *C) {

	tests := []struct {
		name         string
		gitContext   common_models.GitContext
		branch       string
		sourceBranch string
		error        error
	}{
		{
			name:         "simple branch from master",
			gitContext:   s.NewGitContext(),
			branch:       "dev",
			sourceBranch: "master",
			error:        nil,
		},
		{
			name:         "add existing",
			gitContext:   s.NewGitContext(),
			branch:       "dev",
			sourceBranch: "master",
			error:        git.ErrBranchExists,
		},
		{
			name:         "illegal add to non existing branch",
			gitContext:   s.NewGitContext(),
			branch:       "dev",
			sourceBranch: "refs/heads/branch",
			error:        kerrors.ErrReferenceNotFound,
		},
	}
	r := s.Repository
	g := Git{
		s.NewTestGit(),
	}

	expected := []byte("[core]\n\tbare = false\n[remote \"origin\"]\n\turl = " +
		"../test/tmp/remote\n\tfetch = +refs/heads/*:refs/remotes/origin/*\n[branch \"master\"]\n" +
		"\tremote = origin\n\tmerge = refs/heads/master\n[branch \"dev\"]\n" +
		"\tremote = origin\n\tmerge = refs/heads/dev\n")

	for _, tt := range tests {
		c.Logf("Test: %s", tt.name)

		err := g.CreateBranch(tt.gitContext, tt.branch, tt.sourceBranch)

		if err != nil && !errors.Is(tt.error, errors.Unwrap(err)) {
			c.Fatalf("Wanted %v but gotten %v", tt.error, errors.Unwrap(err))
		}

		if err == nil {
			// check git config files
			cfg, err := r.Config()
			c.Assert(err, IsNil)
			marshaled, err := cfg.Marshal()
			c.Assert(err, IsNil)
			c.Assert(string(expected), Equals, string(marshaled))
		}
	}
}

func (s *BaseSuite) TestGit_CheckoutBranch(c *C) {

	tests := []struct {
		name       string
		gitContext common_models.GitContext
		branch     string
		wantErr    bool
	}{
		{
			name:       "checkout master branch full ref",
			gitContext: s.NewGitContext(),
			branch:     "refs/heads/master",
		},
		{
			name:       "checkout master branch",
			gitContext: s.NewGitContext(),
			branch:     "master",
		},
		{
			name:       "checkout not existing branch",
			gitContext: s.NewGitContext(),
			branch:     "refs/heads/dev",
			wantErr:    true,
		},
	}
	g := Git{s.NewTestGit()}
	for _, tt := range tests {
		c.Log("Test: ", tt.name)
		if err := g.CheckoutBranch(tt.gitContext, tt.branch); (err != nil) != tt.wantErr {
			c.Errorf("CheckoutBranch() error = %v, wantErr %v", err, tt.wantErr)
		}

	}
}

func (s *BaseSuite) TestGit_GetFileRevision(c *C) {

	tests := []struct {
		name       string
		gitContext common_models.GitContext
		file       string
		content    string
		wantErr    bool
		id         string
	}{
		{
			name:       "get from commitID",
			gitContext: s.NewGitContext(),
			file:       "foo/example.go",
			content:    "ciao",
			wantErr:    false,
			id:         "",
		},
		{
			name:       "not existing commitID",
			gitContext: s.NewGitContext(),
			file:       "foo/example.go",
			content:    "ciao",
			wantErr:    true,
			id:         "ciaoWrongId",
		},
		{
			name:       "good id but not existing file",
			gitContext: s.NewGitContext(),
			file:       "exam.go",
			content:    "ciao",
			wantErr:    true,
			id:         "6ecf0ef2c2dffb796033e5a02219af86ec6584e5",
		},
		{
			name:       "invalid revision",
			gitContext: s.NewGitContext(),
			file:       "exam.go",
			content:    "ciao",
			wantErr:    true,
			id:         "6ecf0@ef2c2dffb796033e5a0@2219af86ec6584e5",
		},
	}
	for _, tt := range tests {
		c.Log("Test : " + tt.name)
		var id string
		g := Git{s.NewTestGit()}
		if tt.id == "" {
			h := s.commitAndPush(tt.file, tt.content, c)
			id = h.String()
		} else {
			id = tt.id
		}
		got, err := g.GetFileRevision(tt.gitContext, id, tt.file)
		if (err != nil) != tt.wantErr {
			c.Errorf("GetFileRevision() error = %v, wantErr %v", err, tt.wantErr)
			return
		}
		if !tt.wantErr {
			b := []byte(fmt.Sprintf("%s", tt.content))
			if !reflect.DeepEqual(got, b) {
				c.Errorf("GetFileRevision() got = %v, exists %v", got, b)
			}
		}
	}
}

func (s *BaseSuite) TestGit_ProjectRepoExists(c *C) {

	tests := []struct {
		name    string
		project string
		want    bool
	}{
		{
			name:    "project exists",
			project: "sockshop",
			want:    true,
		},
		{
			name:    "project does not exists",
			project: "whatever",
			want:    false,
		},
	}
	for _, tt := range tests {
		if tt.want {
			os.Mkdir(GetProjectConfigPath(tt.project), os.ModePerm)
			git.PlainInit(GetProjectConfigPath(tt.project), false)
		}
		g := Git{GogitReal{}}
		if got := g.ProjectRepoExists(tt.project); got != tt.want {
			c.Errorf("ProjectRepoExists() = %v, exists %v", got, tt.want)
		}

	}
}

func (s *BaseSuite) TestGit_ProjectExists(c *C) {

	tests := []struct {
		name       string
		gitContext common_models.GitContext
		exists     bool
		git        Gogit
	}{
		{
			name:       "project exists",
			gitContext: s.NewGitContext(),
			exists:     true,
			git:        GogitReal{},
		},
		{
			name: "project does not exists",
			gitContext: common_models.GitContext{
				Project: "nonexisting",
				Credentials: &common_models.GitCredentials{
					User:      "ssss",
					Token:     "bjh",
					RemoteURI: "an url that doesnot exists"},
			},
			exists: false,
			git:    GogitReal{},
		},
		{
			name: "project exists, but remote is empty",
			gitContext: common_models.GitContext{
				Project: "podtato",
				Credentials: &common_models.GitCredentials{
					User:      "ssss",
					Token:     "bjh",
					RemoteURI: buildEmptyRemote()},
			},
			exists: true,
			git:    GogitReal{},
		},
	}
	for _, tt := range tests {
		c.Log(tt.name)
		g := &Git{tt.git}
		if got := g.ProjectExists(tt.gitContext); got != tt.exists {
			c.Errorf("ProjectExists() = %v, exists %v", got, tt.exists)
		}
	}
}

func (s *BaseSuite) Test_getGitKeptnUser(c *C) {
	tests := []struct {
		name        string
		envVarValue string
		want        string
	}{
		{
			name:        "default value",
			envVarValue: "",
			want:        gitKeptnUserDefault,
		},
		{
			name:        "env var value",
			envVarValue: "my-user",
			want:        "my-user",
		},
	}
	for _, tt := range tests {

		_ = os.Setenv(gitKeptnUserEnvVar, tt.envVarValue)
		if got := getGitKeptnUser(); got != tt.want {
			c.Errorf("getGitKeptnUser() = %v, exists %v", got, tt.want)
		}
	}
}

func (s *BaseSuite) Test_getGitKeptnEmail(c *C) {
	tests := []struct {
		name        string
		envVarValue string
		want        string
	}{
		{
			name:        "default value",
			envVarValue: "",
			want:        gitKeptnEmailDefault,
		},
		{
			name:        "env var value",
			envVarValue: "my-user@keptn.sh",
			want:        "my-user@keptn.sh",
		},
	}
	for _, tt := range tests {
		_ = os.Setenv(gitKeptnEmailEnvVar, tt.envVarValue)
		if got := getGitKeptnEmail(); got != tt.want {
			c.Errorf("getGitKeptnEmail() = %v, exists %v", got, tt.want)
		}
	}

}

func (s *BaseSuite) NewGitContext() common_models.GitContext {
	return common_models.GitContext{
		Project: "sockshop",
		Credentials: &common_models.GitCredentials{
			User:      "Me",
			Token:     "blabla",
			RemoteURI: s.url,
		},
	}
}

func (s *BaseSuite) NewTestGit() *common_mock.GogitMock {

	return &common_mock.GogitMock{
		PlainCloneFunc: func(path string, isBare bool, o *git.CloneOptions) (*git.Repository, error) {
			return s.Repository, nil
		},
		PlainInitFunc: nil,
		PlainOpenFunc: func(path string) (*git.Repository, error) {
			return s.Repository, nil //git.PlainOpen(path)
		},
	}
}

func (s *BaseSuite) commitAndPush(file string, content string, c *C) plumbing.Hash {
	r := s.Repository
	w, err := r.Worktree()
	c.Assert(err, IsNil)
	err = write(file, content, c, w)
	c.Assert(err, IsNil)
	id := commit(file, c, w)
	push(r, c)
	return id
}

func commit(file string, c *C, w *git.Worktree) plumbing.Hash {
	_, err := w.Add(file)
	c.Assert(err, IsNil)

	id, err := w.Commit("added a file",
		&git.CommitOptions{
			All: true,
			Author: &object.Signature{
				Name:  "Test Create Branch",
				Email: "createBranch@gogit-test.com",
				When:  time.Now(),
			},
		})

	c.Assert(err, IsNil)
	return id
}

func write(file string, content string, c *C, w *git.Worktree) error {
	f, err := w.Filesystem.Create(file)
	c.Assert(err, IsNil)
	f.Write([]byte(fmt.Sprintf("%s", content)))
	f.Close()
	_, err = w.Add(file)
	return err
}

func push(r *git.Repository, c *C) {
	//push to repo
	err := r.Push(&git.PushOptions{
		//Force: true,
		Auth: &http.BasicAuth{
			Username: "whatever",
			Password: "whatever",
		}})
	c.Assert(err, IsNil)
}

func buildEmptyRemote() string {
	url := fixtures.ByURL("https://github.com/git-fixtures/empty.git").One().DotGit().Root()
	return url
}

func checkout(c *C, g *Git, gitContext common_models.GitContext, branch string) error {
	err := g.CheckoutBranch(gitContext, branch)
	if err != nil {
		err = g.CreateBranch(gitContext, branch, "master")
		c.Assert(err, IsNil)
	}
	return err
}