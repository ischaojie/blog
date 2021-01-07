package blog

import (
	"context"
	"errors"
	"github.com/shurcooL/githubv4"
	"golang.org/x/oauth2"
	"io/fs"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
)

type Client interface{}

type Backup struct {
	client Client
}

func NewBackup(target string) Backup {
	backup := Backup{}
	if target == "github" {
		client := githubClient()
		backup.client = client
	}

	return backup
}

func githubClient() *githubv4.Client {
	token := os.Getenv("GITHUB_AUTH_TOKEN")
	if token == "" {
		log.Fatal("Can't import GITHUB_AUTH_TOKEN")
	}

	tc := oauth2.NewClient(context.Background(), oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	))
	client := githubv4.NewClient(tc)
	return client
}

type Article struct {
	Title   string
	Date    string
	Tags    []string
	Content string
}

// ParseArticle parse blog article struct, extract title & tags & content
func ParseArticle(articlePath string) (article Article, err error) {

	// title
	articleName := filepath.Base(articlePath)
	article.Title = strings.TrimSuffix(articleName, ".md")

	// read from article file
	content, err := ioutil.ReadFile(articlePath)
	article.Content = string(content)
	if err != nil {
		return article, err
	}
	return article, err

}

func (b Backup) QueryOwner() (githubv4.String, error) {
	client := b.client.(*githubv4.Client)

	var q struct {
		Viewer struct {
			Login githubv4.String
		}
	}

	err := client.Query(context.Background(), &q, nil)
	if err != nil {
		return "", err
	}

	return q.Viewer.Login, nil
}

func (b Backup) QueryBlogRepoID(repo string) (githubv4.ID, error) {
	client := b.client.(*githubv4.Client)

	owner, err := b.QueryOwner()
	if err != nil {
		return nil, err
	}

	var q struct {
		Repository struct {
			ID githubv4.ID
		} `graphql:"repository(owner:$repositoryOwner,name:$repositoryName)"`
	}

	variables := map[string]interface{}{
		"repositoryOwner": owner,
		"repositoryName":  githubv4.String(repo),
	}

	err = client.Query(context.Background(), &q, variables)
	if err != nil {
		return nil, err
	}

	return q.Repository.ID, nil

}

// CreateIssue create an github issue from article
func (b Backup) CreateIssue(article Article, repo string) (githubv4.ID, error) {
	client := b.client.(*githubv4.Client)

	blogRepoID, err := b.QueryBlogRepoID(repo)
	if err != nil {
		return nil, err
	}

	var m struct {
		CreateIssue struct {
			Issue struct {
				ID githubv4.ID
			}
		} `graphql:"createIssue(input:$input)"`
	}

	content := githubv4.String(article.Content)
	input := githubv4.CreateIssueInput{
		Title:        githubv4.String(article.Title),
		Body:         &content,
		RepositoryID: blogRepoID,
	}

	err = client.Mutate(context.Background(), &m, input, nil)
	if err != nil {
		return nil, err
	}
	return m.CreateIssue.Issue.ID, nil
}

// DeleteIssue delete an issue from github
func (b Backup) DeleteIssue(issueID githubv4.ID) (githubv4.Int, error) {

	var m struct {
		DeleteIssue struct {
			Repository struct {
				Issues struct {
					TotalCount githubv4.Int
				}
			}
		} `graphql:"deleteIssue(input:$input)"`
	}
	input := githubv4.DeleteIssueInput{
		IssueID: issueID,
	}
	client := b.client.(*githubv4.Client)
	err := client.Mutate(context.Background(), &m, input, nil)
	if err != nil {
		return -1, err
	}

	return m.DeleteIssue.Repository.Issues.TotalCount, nil

}

func (b Backup) QueryRepoIssues(name string) ([]githubv4.ID, error) {
	client := b.client.(*githubv4.Client)
	owner, err := b.QueryOwner()
	if err != nil {
		return nil, err
	}

	var q1 struct {
		Repository struct {
			Issues struct {
				TotalCount githubv4.Int
			}
		} `graphql:"repository(owner:$repositoryOwner,name:$repositoryName)"`
	}
	variables := map[string]interface{}{
		"repositoryOwner": owner,
		"repositoryName":  githubv4.String(name),
	}
	err = client.Query(context.Background(), &q1, variables)
	if err != nil {
		return nil, err
	}
	issuesCount := q1.Repository.Issues.TotalCount

	var q2 struct {
		Repository struct {
			Issues struct {
				Nodes []struct {
					ID githubv4.ID
				}
			} `graphql:"issues(first:$totalCount)"`
		} `graphql:"repository(owner:$repositoryOwner,name:$repositoryName)"`
	}
	variables2 := map[string]interface{}{
		"repositoryOwner": githubv4.String("shiniao"),
		"repositoryName":  githubv4.String("blog"),
		"totalCount":      issuesCount,
	}
	err = client.Query(context.Background(), &q2, variables2)
	if err != nil {
		return nil, err
	}
	var ids []githubv4.ID
	for _, node := range q2.Repository.Issues.Nodes {
		ids = append(ids, node.ID)
	}
	return ids, nil

}

// BackupToGithub backup blog articles to github issue
// first delete all issues, then backup(maybe have a better solution)
func (b Backup) BackupToGithub(repo, articlesDir string) (string, error) {

	// get all issues
	issuesID, err := b.QueryRepoIssues(repo)
	if err != nil {
		return "", err
	}
	// delete all issues
	for _, id := range issuesID {
		b.DeleteIssue(id)
	}
	// read articles
	articles, err := ioutil.ReadDir(articlesDir)

	if err != nil {
		return "", err
	}
	// upload all articles to issue
	for _, articlePath := range articles {
		// abs article path
		path := filepath.Join(articlesDir, articlePath.Name())
		article, err := ParseArticle(path)
		if err != nil {
			return "", err
		}
		// create Issue concurrent
		b.CreateIssue(article, repo)

	}
	return "ok", nil

}

func (b Backup) BackupToGithubCon(repo, articlesDir string) {

	// get all issues
	issuesID, err := b.QueryRepoIssues(repo)
	if err != nil {
		log.Fatal(err)
	}

	deleteErrorChan := make(chan error, len(issuesID))

	// delete all issues
	for _, id := range issuesID {

		go func(id githubv4.ID) {
			_, err := b.DeleteIssue(id)
			if err != nil {
				deleteErrorChan <- err
			}
			deleteErrorChan <- nil

		}(id)
	}

	var deleteError string
	for i := 0; i < len(issuesID); i++ {
		if err := <-deleteErrorChan; err != nil {
			deleteError = deleteError + "" + err.Error()
		}
	}

	if deleteError != "" {
		log.Fatalf("delete issue err: %s", errors.New(deleteError))
	}

	log.Print("delete all issues ok")

	// read articles
	articles, err := ioutil.ReadDir(articlesDir)

	if err != nil {
		log.Fatal(err)
	}
	// upload all articles to issue
	createIssueChan := make(chan githubv4.ID, len(articles))
	createIssueErrorChan := make(chan error, len(articles))
	for _, articlePath := range articles {

		go func(articlePath fs.FileInfo) {
			// abs article path
			path := filepath.Join(articlesDir, articlePath.Name())
			article, err := ParseArticle(path)
			if err != nil {
				createIssueChan <- -1
				createIssueErrorChan <- err
			}
			// create Issue
			id, err := b.CreateIssue(article, repo)
			if err != nil {
				createIssueChan <- -1
				createIssueErrorChan <- err
			}
			createIssueChan <- id
			createIssueErrorChan <- nil
		}(articlePath)

	}
	var createErrorStr string
	var createIssuesID []githubv4.ID
	for i := 0; i < len(articles); i++ {
		createIssuesID = append(createIssuesID, <-createIssueChan)
		if err := <-createIssueErrorChan; err != nil {
			createErrorStr = createErrorStr + "" + err.Error()
		}
	}

	if createErrorStr != "" {
		log.Fatal(errors.New(createErrorStr))
	}

	log.Print("backup ok")

}
