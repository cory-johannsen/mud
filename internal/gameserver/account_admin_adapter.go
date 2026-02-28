package gameserver

import (
	"context"

	"github.com/cory-johannsen/mud/internal/storage/postgres"
)

// AccountRepoAdapter wraps a postgres.AccountRepository to satisfy the AccountAdmin interface.
type AccountRepoAdapter struct {
	repo *postgres.AccountRepository
}

// NewAccountRepoAdapter creates an adapter around the given repository.
func NewAccountRepoAdapter(repo *postgres.AccountRepository) *AccountRepoAdapter {
	return &AccountRepoAdapter{repo: repo}
}

// GetAccountByUsername looks up an account by username.
func (a *AccountRepoAdapter) GetAccountByUsername(ctx context.Context, username string) (AccountInfo, error) {
	acct, err := a.repo.GetByUsername(ctx, username)
	if err != nil {
		return AccountInfo{}, err
	}
	return AccountInfo{
		ID:       acct.ID,
		Username: acct.Username,
		Role:     acct.Role,
	}, nil
}

// SetAccountRole updates an account's role.
func (a *AccountRepoAdapter) SetAccountRole(ctx context.Context, accountID int64, role string) error {
	return a.repo.SetRole(ctx, accountID, role)
}
