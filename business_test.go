package testutils

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"go_server_enterprise/internal/business"
)

// ----------------------
// Mock Helpers
// ----------------------

type row struct {
	scanFunc func(dest ...interface{}) error
}

func (r row) Scan(dest ...interface{}) error { return r.scanFunc(dest...) }

type result struct {
	rowsAffected int64
}

func (r result) RowsAffected() (int64, error) { return r.rowsAffected, nil }

var ErrNoRows = errors.New("sql: no rows in result set")

type mockDB struct {
	queryRowFunc func(ctx context.Context, query string, args ...interface{}) business.Row
	execFunc     func(ctx context.Context, query string, args ...interface{}) (business.Result, error)
}

func (m *mockDB) QueryRowContext(ctx context.Context, query string, args ...interface{}) business.Row {
	return m.queryRowFunc(ctx, query, args...)
}

func (m *mockDB) ExecContext(ctx context.Context, query string, args ...interface{}) (business.Result, error) {
	return m.execFunc(ctx, query, args...)
}

type mockTxMgr struct {
	executeFunc func(ctx context.Context, fn func(tx business.Tx) error) error
}

func (m *mockTxMgr) Execute(ctx context.Context, fn func(tx business.Tx) error) error {
	return m.executeFunc(ctx, fn)
}

type mockTx struct {
	queryRowFunc func(ctx context.Context, query string, args ...interface{}) business.Row
	execFunc     func(ctx context.Context, query string, args ...interface{}) (business.Result, error)
}

func (m mockTx) QueryRowContext(ctx context.Context, query string, args ...interface{}) business.Row {
	return m.queryRowFunc(ctx, query, args...)
}

func (m mockTx) ExecContext(ctx context.Context, query string, args ...interface{}) (business.Result, error) {
	return m.execFunc(ctx, query, args...)
}

// ----------------------
// Table-driven repo tests
// ----------------------

type repoTest[T any] struct {
	name      string
	input     interface{}
	setupMock func() any // *mockDB or *mockTxMgr
	want      T
	wantErr   bool
	extra     interface{}
}

func runRepoTest[T any](t *testing.T, tests []repoTest[T], testFunc func(input interface{}, extra interface{}, repo *business.SQLUserRepository) (T, error)) {
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := tt.setupMock()
			repo := business.NewSQLUserRepository(nil, nil)
			switch m := mock.(type) {
			case *mockDB:
				repo = business.NewSQLUserRepository(m, nil)
			case *mockTxMgr:
				repo = business.NewSQLUserRepository(nil, m)
			}
			got, err := testFunc(tt.input, tt.extra, repo)
			if (err != nil) != tt.wantErr {
				t.Errorf("Error = %v, wantErr=%v", err, tt.wantErr)
			}
			if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Got = %v, want %v", got, tt.want)
			}
		})
	}
}

// ----------------------
// CRUD Tests
// ----------------------

// Create
func TestSQLUserRepository_Create(t *testing.T) {
	tests := []repoTest[int64]{
		{
			name:  "Success",
			input: &business.ExampleUser{Username: "alice", Email: "a@x.com", IsActive: true, CreatedAt: 123},
			setupMock: func() any {
				return &mockDB{
					queryRowFunc: func(ctx context.Context, query string, args ...interface{}) business.Row {
						return row{scanFunc: func(dest ...interface{}) error {
							*(dest[0].(*int64)) = 42
							return nil
						}}
					},
				}
			},
			want:    42,
			wantErr: false,
		},
		{
			name:  "Failure",
			input: &business.ExampleUser{Username: "bob"},
			setupMock: func() any {
				return &mockDB{
					queryRowFunc: func(ctx context.Context, query string, args ...interface{}) business.Row {
						return row{scanFunc: func(dest ...interface{}) error { return errors.New("insert fail") }}
					},
				}
			},
			want:    0,
			wantErr: true,
		},
	}

	runRepoTest(t, tests, func(input interface{}, extra interface{}, repo *business.SQLUserRepository) (int64, error) {
		return repo.Create(context.Background(), input.(*business.ExampleUser))
	})
}

// GetByID
func TestSQLUserRepository_GetByID(t *testing.T) {
	tests := []repoTest[*business.ExampleUser]{
		{
			name:  "Found",
			input: 1,
			setupMock: func() any {
				return &mockDB{
					queryRowFunc: func(ctx context.Context, query string, args ...interface{}) business.Row {
						return row{scanFunc: func(dest ...interface{}) error {
							*(dest[0].(*int)) = 1
							*(dest[1].(*string)) = "alice"
							*(dest[2].(*string)) = "alice@x.com"
							*(dest[3].(*bool)) = true
							*(dest[4].(*int64)) = 123
							return nil
						}}
					},
				}
			},
			want: &business.ExampleUser{ID: 1, Username: "alice", Email: "alice@x.com", IsActive: true, CreatedAt: 123},
		},
		{
			name:  "NotFound",
			input: 2,
			setupMock: func() any {
				return &mockDB{
					queryRowFunc: func(ctx context.Context, query string, args ...interface{}) business.Row {
						return row{scanFunc: func(dest ...interface{}) error { return ErrNoRows }}
					},
				}
			},
			want:    nil,
			wantErr: true,
		},
	}

	runRepoTest(t, tests, func(input interface{}, extra interface{}, repo *business.SQLUserRepository) (*business.ExampleUser, error) {
		return repo.GetByID(context.Background(), input.(int))
	})
}

// UpdateEmail
func TestSQLUserRepository_UpdateEmail(t *testing.T) {
	tests := []repoTest[struct{}]{
		{
			name:  "Success",
			input: 1,
			extra: "new@x.com",
			setupMock: func() any {
				return &mockTxMgr{
					executeFunc: func(ctx context.Context, fn func(tx business.Tx) error) error {
						tx := mockTx{
							queryRowFunc: func(ctx context.Context, query string, args ...interface{}) business.Row {
								return row{scanFunc: func(dest ...interface{}) error {
									*(dest[0].(*bool)) = true
									return nil
								}}
							},
							execFunc: func(ctx context.Context, query string, args ...interface{}) (business.Result, error) {
								return result{rowsAffected: 1}, nil
							},
						}
						return fn(tx)
					},
				}
			},
			wantErr: false,
		},
		{
			name:  "UserDoesNotExist",
			input: 2,
			extra: "new@x.com",
			setupMock: func() any {
				return &mockTxMgr{
					executeFunc: func(ctx context.Context, fn func(tx business.Tx) error) error {
						tx := mockTx{
							queryRowFunc: func(ctx context.Context, query string, args ...interface{}) business.Row {
								return row{scanFunc: func(dest ...interface{}) error {
									*(dest[0].(*bool)) = false
									return nil
								}}
							},
							execFunc: func(ctx context.Context, query string, args ...interface{}) (business.Result, error) {
								return result{rowsAffected: 0}, nil
							},
						}
						return fn(tx)
					},
				}
			},
			wantErr: true,
		},
	}

	runRepoTest(t, tests, func(input interface{}, extra interface{}, repo *business.SQLUserRepository) (struct{}, error) {
		return struct{}{}, repo.UpdateEmail(context.Background(), input.(int), extra.(string))
	})
}

// Delete
func TestSQLUserRepository_Delete(t *testing.T) {
	tests := []repoTest[struct{}]{
		{
			name:  "Success",
			input: 1,
			setupMock: func() any {
				return &mockDB{
					execFunc: func(ctx context.Context, query string, args ...interface{}) (business.Result, error) {
						return result{rowsAffected: 1}, nil
					},
				}
			},
			wantErr: false,
		},
		{
			name:  "NotFound",
			input: 2,
			setupMock: func() any {
				return &mockDB{
					execFunc: func(ctx context.Context, query string, args ...interface{}) (business.Result, error) {
						return result{rowsAffected: 0}, nil
					},
				}
			},
			wantErr: true,
		},
	}

	runRepoTest(t, tests, func(input interface{}, extra interface{}, repo *business.SQLUserRepository) (struct{}, error) {
		return struct{}{}, repo.Delete(context.Background(), input.(int))
	})
}
