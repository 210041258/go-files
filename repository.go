package testutils


import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math/rand"
	"time"
)

// ExampleUser represents a simple domain entity.
type ExampleUser struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	Email     string `json:"email"`
	IsActive  bool   `json:"is_active"`
	CreatedAt int64  `json:"created_at"` // Unix timestamp
}

// UserRepository defines the interface for user data operations.
type UserRepository interface {
	Create(ctx context.Context, user *ExampleUser) (int64, error)
	GetByID(ctx context.Context, id int64) (*ExampleUser, error)
	UpdateEmail(ctx context.Context, id int64, newEmail string) error
	UpdateStatus(ctx context.Context, id int64, isActive bool) error
	Delete(ctx context.Context, id int64) error
	List(ctx context.Context, limit, offset int) ([]*ExampleUser, error)
}

// RetryConfig defines retry behavior for transient errors.
type RetryConfig struct {
	MaxAttempts int
	BaseBackoff time.Duration
	MaxBackoff  time.Duration
}

// DefaultRetryConfig returns a safe default retry policy.
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxAttempts: 5,
		BaseBackoff: 200 * time.Millisecond,
		MaxBackoff:  3 * time.Second,
	}
}

// SQLUserRepository implements UserRepository using DB abstraction.
type SQLUserRepository struct {
	db     DB
	txMgr  *TransactionManager
	logger Logger
	retry  *RetryConfig
}

// NewSQLUserRepository creates a new repository instance.
func NewSQLUserRepository(db DB, txMgr *TransactionManager, logger Logger) *SQLUserRepository {
	return &SQLUserRepository{
		db:     db,
		txMgr:  txMgr,
		logger: logger,
		retry:  DefaultRetryConfig(),
	}
}

// SetRetryConfig allows overriding the default retry policy.
func (r *SQLUserRepository) SetRetryConfig(cfg *RetryConfig) {
	r.retry = cfg
}

// log safely logs if logger is set.
func (r *SQLUserRepository) log(format string, v ...interface{}) {
	if r.logger != nil {
		r.logger.Printf(format, v...)
	}
}

// retryable executes a DB operation with exponential backoff and jitter.
func (r *SQLUserRepository) retryable(ctx context.Context, fn func() error) error {
	var lastErr error
	for attempt := 1; attempt <= r.retry.MaxAttempts; attempt++ {
		err := fn()
		if err == nil {
			return nil
		}

		// Only retry on transient errors
		if errors.Is(err, sql.ErrConnDone) || errors.Is(err, sql.ErrTxDone) {
			lastErr = err

			// Exponential backoff with jitter
			backoff := r.retry.BaseBackoff * time.Duration(1<<uint(attempt-1))
			if backoff > r.retry.MaxBackoff {
				backoff = r.retry.MaxBackoff
			}
			jitter := time.Duration(rand.Int63n(int64(backoff))) - backoff/2
			sleep := backoff + jitter

			r.log("Attempt %d failed: %v, retrying in %s...", attempt, err, sleep)

			select {
			case <-ctx.Done():
				return fmt.Errorf("context cancelled during retry: %w", ctx.Err())
			case <-time.After(sleep):
			}
			continue
		}

		return err // non-transient errors are returned immediately
	}
	return fmt.Errorf("all %d attempts failed: %w", r.retry.MaxAttempts, lastErr)
}

// ---------------- CRUD Operations ---------------- //

// Create inserts a new user into the database with retries.
func (r *SQLUserRepository) Create(ctx context.Context, user *ExampleUser) (int64, error) {
	if user.CreatedAt == 0 {
		user.CreatedAt = time.Now().Unix()
	}
	const query = `
		INSERT INTO users (username, email, is_active, created_at)
		VALUES ($1, $2, $3, $4)
		RETURNING id
	`

	var id int64
	err := r.retryable(ctx, func() error {
		return r.db.QueryRowContext(ctx, query, user.Username, user.Email, user.IsActive, user.CreatedAt).Scan(&id)
	})
	if err != nil {
		return 0, fmt.Errorf("failed to create user: %w", err)
	}

	r.log("Created user %d", id)
	return id, nil
}

// GetByID retrieves a user by ID with retries.
func (r *SQLUserRepository) GetByID(ctx context.Context, id int64) (*ExampleUser, error) {
	const query = `
		SELECT id, username, email, is_active, created_at
		FROM users
		WHERE id = $1
	`

	user := &ExampleUser{}
	err := r.retryable(ctx, func() error {
		return r.db.QueryRowContext(ctx, query, id).Scan(
			&user.ID, &user.Username, &user.Email, &user.IsActive, &user.CreatedAt,
		)
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || IsNotFoundError(err) {
			return nil, fmt.Errorf("user with id %d not found", id)
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	r.log("Retrieved user %d", id)
	return user, nil
}

// UpdateEmail updates a user's email inside a transaction with retries.
func (r *SQLUserRepository) UpdateEmail(ctx context.Context, id int64, newEmail string) error {
	return r.retryable(ctx, func() error {
		return r.txMgr.Execute(ctx, func(tx Tx) error {
			const query = `UPDATE users SET email = $1 WHERE id = $2`
			res, err := tx.ExecContext(ctx, query, newEmail, id)
			if err != nil {
				return fmt.Errorf("failed to update email: %w", err)
			}
			if rows, _ := res.RowsAffected(); rows == 0 {
				return fmt.Errorf("no user updated with id %d", id)
			}
			r.log("Updated email for user %d", id)
			return nil
		})
	})
}

// UpdateStatus updates a user's active status with retries.
func (r *SQLUserRepository) UpdateStatus(ctx context.Context, id int64, isActive bool) error {
	return r.retryable(ctx, func() error {
		return r.txMgr.Execute(ctx, func(tx Tx) error {
			const query = `UPDATE users SET is_active = $1 WHERE id = $2`
			res, err := tx.ExecContext(ctx, query, isActive, id)
			if err != nil {
				return fmt.Errorf("failed to update status: %w", err)
			}
			if rows, _ := res.RowsAffected(); rows == 0 {
				return fmt.Errorf("no user updated with id %d", id)
			}
			r.log("Updated status for user %d", id)
			return nil
		})
	})
}

// Delete removes a user by ID with retries.
func (r *SQLUserRepository) Delete(ctx context.Context, id int64) error {
	const query = `DELETE FROM users WHERE id = $1`
	return r.retryable(ctx, func() error {
		res, err := r.db.ExecContext(ctx, query, id)
		if err != nil {
			return fmt.Errorf("failed to delete user: %w", err)
		}
		if rows, _ := res.RowsAffected(); rows == 0 {
			return fmt.Errorf("no user deleted with id %d", id)
		}
		r.log("Deleted user %d", id)
		return nil
	})
}

// List returns a paginated list of users with retries.
func (r *SQLUserRepository) List(ctx context.Context, limit, offset int) ([]*ExampleUser, error) {
	const query = `
		SELECT id, username, email, is_active, created_at
		FROM users
		ORDER BY id
		LIMIT $1 OFFSET $2
	`
	var users []*ExampleUser
	err := r.retryable(ctx, func() error {
		rows, err := r.db.QueryContext(ctx, query, limit, offset)
		if err != nil {
			return fmt.Errorf("failed to query users: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			var u ExampleUser
			if err := rows.Scan(&u.ID, &u.Username, &u.Email, &u.IsActive, &u.CreatedAt); err != nil {
				return fmt.Errorf("failed to scan user: %w", err)
			}
			users = append(users, &u)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, err
	}

	r.log("Listed %d users (limit=%d, offset=%d)", len(users), limit, offset)
	return users, nil
}
