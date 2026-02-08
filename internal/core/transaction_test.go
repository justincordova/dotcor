package core

import (
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/justincordova/dotcor/internal/config"
	"github.com/stretchr/testify/assert"
)

// mockOperation is a simple operation for testing
type mockOperation struct {
	doErr     error
	undoErr   error
	doCalls   int
	undoCalls int
}

func (m *mockOperation) Do() error {
	m.doCalls++
	return m.doErr
}

func (m *mockOperation) Undo() error {
	m.undoCalls++
	return m.undoErr
}

func (m *mockOperation) Describe() string {
	return "mock operation"
}

func TestNewTransaction(t *testing.T) {
	// Arrange
	cfg := &config.Config{Logger: slog.Default()}

	// Act
	tx := NewTransaction(cfg)

	// Assert
	assert.NotNil(t, tx, "NewTransaction should return non-nil transaction")
	assert.False(t, tx.IsCommitted(), "NewTransaction should not be committed")
	assert.Equal(t, 0, tx.ExecutedCount(), "NewTransaction should have 0 executed operations")
}

func TestTransactionExecute(t *testing.T) {
	// Arrange
	cfg := &config.Config{Logger: slog.Default()}
	tx := NewTransaction(cfg)
	op := &mockOperation{}

	// Act
	err := tx.Execute(op)

	// Assert
	assert.NoError(t, err, "Execute should succeed")
	assert.Equal(t, 1, op.doCalls, "Execute should call Do() once")
	assert.Equal(t, 1, tx.ExecutedCount(), "ExecutedCount should be 1")
}

func TestTransactionExecuteFails(t *testing.T) {
	// Arrange
	cfg := &config.Config{Logger: slog.Default()}
	tx := NewTransaction(cfg)
	op1 := &mockOperation{}
	op2 := &mockOperation{doErr: errors.New("operation failed")}

	// Act - First operation succeeds
	if err := tx.Execute(op1); err != nil {
		t.Fatalf("First Execute() error = %v", err)
	}
	err := tx.Execute(op2)

	// Assert
	assert.Error(t, err, "Execute should return error when operation fails")
	assert.Equal(t, 1, op1.undoCalls, "Rollback should have called Undo() on op1")
}

func TestTransactionRollback(t *testing.T) {
	// Arrange
	cfg := &config.Config{Logger: slog.Default()}
	tx := NewTransaction(cfg)
	op1 := &mockOperation{}
	op2 := &mockOperation{}
	tx.Execute(op1)
	tx.Execute(op2)

	// Act
	err := tx.Rollback()

	// Assert
	assert.NoError(t, err, "Rollback should succeed")
	assert.Equal(t, 1, op1.undoCalls, "op1.Undo() should be called once")
	assert.Equal(t, 1, op2.undoCalls, "op2.Undo() should be called once")
}

func TestTransactionCommit(t *testing.T) {
	// Arrange
	cfg := &config.Config{Logger: slog.Default()}
	tx := NewTransaction(cfg)
	op := &mockOperation{}
	tx.Execute(op)

	// Act
	tx.Commit()

	// Assert
	assert.True(t, tx.IsCommitted(), "Commit should mark transaction as committed")

	err := tx.Rollback()
	assert.Error(t, err, "Rollback should error after Commit")
}

func TestTransactionExecuteAfterCommit(t *testing.T) {
	// Arrange
	cfg := &config.Config{Logger: slog.Default()}
	tx := NewTransaction(cfg)
	tx.Commit()

	// Act
	op := &mockOperation{}
	err := tx.Execute(op)

	// Assert
	assert.Error(t, err, "Execute should error after Commit")
}

func TestMoveFileOp(t *testing.T) {
	// Arrange
	tempDir := t.TempDir()
	cfg := &config.Config{Logger: slog.Default()}
	src := filepath.Join(tempDir, "source")
	dst := filepath.Join(tempDir, "dest")
	if err := os.WriteFile(src, []byte("content"), 0644); err != nil {
		t.Fatalf("failed to create source file: %v", err)
	}
	op := &CopyFileOp{Src: src, Dst: dst, Config: cfg}

	// Act
	err := op.Do()

	// Assert
	assert.NoError(t, err, "CopyFileOp.Do should succeed")
	assert.FileExists(t, src, "CopyFileOp.Do should keep source")
	assert.FileExists(t, dst, "CopyFileOp.Do should create dest")

	// Act - Undo the operation
	err = op.Undo()

	// Assert
	assert.NoError(t, err, "CopyFileOp.Undo should succeed")
	assert.FileExists(t, src, "CopyFileOp.Undo should keep source")
	assert.NoFileExists(t, dst, "CopyFileOp.Undo should remove dest")
}

func TestCreateDirOp(t *testing.T) {
	// Arrange
	tempDir := t.TempDir()
	newDir := filepath.Join(tempDir, "newdir")
	cfg := &config.Config{Logger: slog.Default()}
	op := &CreateDirOp{Path: newDir, Config: cfg}

	// Act
	err := op.Do()

	// Assert
	assert.NoError(t, err, "CreateDirOp.Do should succeed")
	info, err := os.Stat(newDir)
	assert.NoError(t, err, "CreateDirOp.Do should create directory")
	assert.True(t, info.IsDir(), "CreateDirOp.Do should create a directory, not file")

	// Act - Undo the operation (should remove empty dir)
	err = op.Undo()

	// Assert
	assert.NoError(t, err, "CreateDirOp.Undo should succeed")
	assert.NoFileExists(t, newDir, "CreateDirOp.Undo should remove empty directory")
}

func TestCreateDirOpUndoNonEmpty(t *testing.T) {
	// Arrange
	tempDir := t.TempDir()
	cfg := &config.Config{Logger: slog.Default()}
	newDir := filepath.Join(tempDir, "newdir")
	op := &CreateDirOp{Path: newDir, Config: cfg}

	// Act - Do operation
	err := op.Do()
	assert.NoError(t, err, "CreateDirOp.Do should succeed")

	// Arrange - Add a file to make it non-empty
	if err := os.WriteFile(filepath.Join(newDir, "file"), []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create file in dir: %v", err)
	}

	// Act - Undo should NOT remove non-empty directory
	err = op.Undo()

	// Assert
	assert.NoError(t, err, "CreateDirOp.Undo should succeed")
	assert.DirExists(t, newDir, "CreateDirOp.Undo should not remove non-empty directory")
}

func TestOperationDescribe(t *testing.T) {
	// Arrange
	tests := []struct {
		name string
		op   Operation
	}{
		{"MoveFileOp", &MoveFileOp{Src: "/a", Dst: "/b"}},
		{"CopyFileOp", &CopyFileOp{Src: "/a", Dst: "/b"}},
		{"CreateDirOp", &CreateDirOp{Path: "/a"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			desc := tt.op.Describe()

			// Assert
			assert.NotEmpty(t, desc, "Describe() should not return empty string")
		})
	}
}

func TestTransactionExecuteAll(t *testing.T) {
	// Arrange
	tempDir := t.TempDir()
	src := filepath.Join(tempDir, "source")
	dst := filepath.Join(tempDir, "dest")
	if err := os.WriteFile(src, []byte("content"), 0644); err != nil {
		t.Fatalf("failed to create source file: %v", err)
	}
	cfg := &config.Config{Logger: slog.Default()}
	tx := NewTransaction(cfg)
	tx.operations = []Operation{
		&CopyFileOp{Src: src, Dst: dst, Config: cfg},
	}

	// Act
	err := tx.ExecuteAll()

	// Assert
	assert.NoError(t, err, "ExecuteAll should succeed")
	assert.FileExists(t, dst, "ExecuteAll should have created dest file")
}
