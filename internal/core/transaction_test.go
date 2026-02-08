package core

import (
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/justincordova/dotcor/internal/config"
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
	cfg := &config.Config{Logger: slog.Default()}
	tx := NewTransaction(cfg)

	if tx == nil {
		t.Fatal("NewTransaction() returned nil")
	}
	if tx.IsCommitted() {
		t.Error("NewTransaction() should not be committed")
	}
	if tx.ExecutedCount() != 0 {
		t.Error("NewTransaction() should have 0 executed operations")
	}
}

func TestTransactionExecute(t *testing.T) {
	cfg := &config.Config{Logger: slog.Default()}
	tx := NewTransaction(cfg)
	op := &mockOperation{}

	err := tx.Execute(op)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if op.doCalls != 1 {
		t.Errorf("Execute() called Do() %d times, want 1", op.doCalls)
	}
	if tx.ExecutedCount() != 1 {
		t.Errorf("ExecutedCount() = %d, want 1", tx.ExecutedCount())
	}
}

func TestTransactionExecuteFails(t *testing.T) {
	cfg := &config.Config{Logger: slog.Default()}
	tx := NewTransaction(cfg)

	// First operation succeeds
	op1 := &mockOperation{}
	if err := tx.Execute(op1); err != nil {
		t.Fatalf("First Execute() error = %v", err)
	}

	// Second operation fails
	op2 := &mockOperation{doErr: errors.New("operation failed")}
	err := tx.Execute(op2)
	if err == nil {
		t.Error("Execute() should return error when operation fails")
	}

	// First operation should have been rolled back
	if op1.undoCalls != 1 {
		t.Errorf("Rollback should have called Undo() on op1, got %d calls", op1.undoCalls)
	}
}

func TestTransactionRollback(t *testing.T) {
	cfg := &config.Config{Logger: slog.Default()}
	tx := NewTransaction(cfg)

	op1 := &mockOperation{}
	op2 := &mockOperation{}

	tx.Execute(op1)
	tx.Execute(op2)

	err := tx.Rollback()
	if err != nil {
		t.Fatalf("Rollback() error = %v", err)
	}

	// Both operations should be rolled back in reverse order
	if op1.undoCalls != 1 {
		t.Errorf("op1.Undo() called %d times, want 1", op1.undoCalls)
	}
	if op2.undoCalls != 1 {
		t.Errorf("op2.Undo() called %d times, want 1", op2.undoCalls)
	}
}

func TestTransactionCommit(t *testing.T) {
	cfg := &config.Config{Logger: slog.Default()}
	tx := NewTransaction(cfg)
	op := &mockOperation{}
	tx.Execute(op)

	tx.Commit()

	if !tx.IsCommitted() {
		t.Error("Commit() should mark transaction as committed")
	}

	// Rollback should fail after commit
	err := tx.Rollback()
	if err == nil {
		t.Error("Rollback() should error after Commit()")
	}
}

func TestTransactionExecuteAfterCommit(t *testing.T) {
	cfg := &config.Config{Logger: slog.Default()}
	tx := NewTransaction(cfg)
	tx.Commit()

	op := &mockOperation{}
	err := tx.Execute(op)
	if err == nil {
		t.Error("Execute() should error after Commit()")
	}
}

func TestMoveFileOp(t *testing.T) {
	// Create temp dir
	tempDir, err := os.MkdirTemp("", "dotcor-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create source file
	cfg := &config.Config{Logger: slog.Default()}
	src := filepath.Join(tempDir, "source")
	dst := filepath.Join(tempDir, "dest")
	if err := os.WriteFile(src, []byte("content"), 0644); err != nil {
		t.Fatalf("failed to create source file: %v", err)
	}

	op := &CopyFileOp{Src: src, Dst: dst, Config: cfg}

	// Do the operation
	if err := op.Do(); err != nil {
		t.Fatalf("CopyFileOp.Do() error = %v", err)
	}

	// Verify both source and dest exist
	if _, err := os.Stat(src); os.IsNotExist(err) {
		t.Error("CopyFileOp.Do() should keep source")
	}
	if _, err := os.Stat(dst); os.IsNotExist(err) {
		t.Error("CopyFileOp.Do() should create dest")
	}

	// Undo the operation
	if err := op.Undo(); err != nil {
		t.Fatalf("CopyFileOp.Undo() error = %v", err)
	}

	// Verify source still exists but dest is gone
	if _, err := os.Stat(src); os.IsNotExist(err) {
		t.Error("CopyFileOp.Undo() should keep source")
	}
	if _, err := os.Stat(dst); !os.IsNotExist(err) {
		t.Error("CopyFileOp.Undo() should remove dest")
	}
}

func TestCreateDirOp(t *testing.T) {
	// Create temp dir
	tempDir, err := os.MkdirTemp("", "dotcor-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create new directory
	newDir := filepath.Join(tempDir, "newdir")
	cfg := &config.Config{Logger: slog.Default()}
	op := &CreateDirOp{Path: newDir, Config: cfg}

	// Do the operation
	if err := op.Do(); err != nil {
		t.Fatalf("CreateDirOp.Do() error = %v", err)
	}

	// Verify directory exists
	info, err := os.Stat(newDir)
	if os.IsNotExist(err) {
		t.Error("CreateDirOp.Do() should create directory")
	}
	if !info.IsDir() {
		t.Error("CreateDirOp.Do() should create a directory, not file")
	}

	// Undo the operation (should remove empty dir)
	if err := op.Undo(); err != nil {
		t.Fatalf("CreateDirOp.Undo() error = %v", err)
	}

	// Verify directory is gone
	if _, err := os.Stat(newDir); !os.IsNotExist(err) {
		t.Error("CreateDirOp.Undo() should remove empty directory")
	}
}

func TestCreateDirOpUndoNonEmpty(t *testing.T) {
	// Create temp dir
	tempDir, err := os.MkdirTemp("", "dotcor-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cfg := &config.Config{Logger: slog.Default()}
	newDir := filepath.Join(tempDir, "newdir")
	op := &CreateDirOp{Path: newDir, Config: cfg}

	// Do operation
	if err := op.Do(); err != nil {
		t.Fatalf("CreateDirOp.Do() error = %v", err)
	}

	// Add a file to make it non-empty
	if err := os.WriteFile(filepath.Join(newDir, "file"), []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create file in dir: %v", err)
	}

	// Undo should NOT remove non-empty directory
	if err := op.Undo(); err != nil {
		t.Fatalf("CreateDirOp.Undo() error = %v", err)
	}

	// Directory should still exist
	if _, err := os.Stat(newDir); os.IsNotExist(err) {
		t.Error("CreateDirOp.Undo() should not remove non-empty directory")
	}
}

func TestOperationDescribe(t *testing.T) {
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
			desc := tt.op.Describe()
			if desc == "" {
				t.Error("Describe() should not return empty string")
			}
		})
	}
}

func TestTransactionExecuteAll(t *testing.T) {
	// Create temp dir
	tempDir, err := os.MkdirTemp("", "dotcor-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create source file
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

	// Execute all operations
	if err := tx.ExecuteAll(); err != nil {
		t.Fatalf("ExecuteAll() error = %v", err)
	}

	// Verify operation was executed
	if _, err := os.Stat(dst); os.IsNotExist(err) {
		t.Error("ExecuteAll() should have created dest file")
	}
}
