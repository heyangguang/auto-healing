package repository

import integrationrepo "github.com/company/auto-healing/internal/modules/integrations/repository"

type GitRepositoryRepository = integrationrepo.GitRepositoryRepository
type GitRepoListOptions = integrationrepo.GitRepoListOptions

type PlaybookRepository = integrationrepo.PlaybookRepository
type PlaybookListOptions = integrationrepo.PlaybookListOptions

var (
	ErrGitRepositoryNotFound = integrationrepo.ErrGitRepositoryNotFound
	ErrPlaybookNotFound      = integrationrepo.ErrPlaybookNotFound
)

func NewGitRepositoryRepository() *GitRepositoryRepository {
	return integrationrepo.NewGitRepositoryRepository()
}

func NewPlaybookRepository() *PlaybookRepository {
	return integrationrepo.NewPlaybookRepository()
}
