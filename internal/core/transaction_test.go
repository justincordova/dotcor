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

	assert.NotNil(t, tx, "NewTransaction should return non-nil transaction")
	assert.False(t, tx.IsCommitted(), "NewTransaction should not be committed")
	assert.Equal(t, 0, tx.ExecutedCount(), "NewTransaction should have 0 executed operations")
}

func TestTransactionExecute(t *testing.T) {
	cfg := &config.Config{Logger: slog.Default()}
	tx := NewTransaction(cfg)
	op := &mockOperation{}

	err := tx.Execute(op)

	assert.NoError(t, err, "Execute should succeed")
	assert.Equal(t, 1, op.doCalls, "Execute should call Do() once")
	assert.Equal(t, 1, tx.ExecutedCount(), "ExecutedCount should be 1")
}

func TestTransactionExecuteFails(t *testing.T) {
	cfg := &config.Config{Logger: slog.Default()}
	tx := NewTransaction(cfg)
	op1 := &mockOperation{}
	op2 := &mockOperation{doErr: errors.New("operation failed")}

	if err := tx.Execute(op1); err != nil {
		t.Fatalf("First Execute() error = %v", err)
	}
	err := tx.Execute(op2)

	assert.Error(t, err, "Execute should return error when operation fails")
	assert.Equal(t, 1, op1.undoCalls, "Rollback should have called Undo() on op1")
}

func TestTransactionRollback(t *testing.T) {
	cfg := &config.Config{Logger: slog.Default()}
	tx := NewTransaction(cfg)
	op1 := &mockOperation{}
	op2 := &mockOperation{}
	if err := tx.Execute(op1); err != nil {
		t.Fatalf("failed to execute op1: %v", err)
	}
	if err := tx.Execute(op2); err != nil {
		t.Fatalf("failed to execute op2: %v", err)
	}

	err := tx.Rollback()

	assert.NoError(t, err, "Rollback should succeed")
	assert.Equal(t, 1, op1.undoCalls, "op1.Undo() should be called once")
	assert.Equal(t, 1, op2.undoCalls, "op2.Undo() should be called once")
}

func TestTransactionCommit(t *testing.T) {
	cfg := &config.Config{Logger: slog.Default()}
	tx := NewTransaction(cfg)
	op := &mockOperation{}
	if err := tx.Execute(op); err != nil {
		t.Fatalf("failed to execute op: %v", err)
	}

	tx.Commit()

	assert.True(t, tx.IsCommitted(), "Commit should mark transaction as committed")

	err := tx.Rollback()
	assert.Error(t, err, "Rollback should error after Commit")
}

func TestTransactionExecuteAfterCommit(t *testing.T) {
	cfg := &config.Config{Logger: slog.Default()}
	tx := NewTransaction(cfg)
	tx.Commit()

	op := &mockOperation{}
	err := tx.Execute(op)

	assert.Error(t, err, "Execute should error after Commit")
}

func TestCopyFileOp(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{Logger: slog.Default()}
	src := filepath.Join(tempDir, "source")
	dst := filepath.Join(tempDir, "dest")
	if err := os.WriteFile(src, []byte("content"), 0644); err != nil {
		t.Fatalf("failed to create source file: %v", err)
	}
	op := &CopyFileOp{Src: src, Dst: dst, Config: cfg}

	err := op.Do()

	assert.NoError(t, err, "CopyFileOp.Do should succeed")
	assert.FileExists(t, src, "CopyFileOp.Do should keep source")
	assert.FileExists(t, dst, "CopyFileOp.Do should create dest")

	err = op.Undo()

	assert.NoError(t, err, "CopyFileOp.Undo should succeed")
	assert.FileExists(t, src, "CopyFileOp.Undo should keep source")
	assert.NoFileExists(t, dst, "CopyFileOp.Undo should remove dest")
}

func TestCreateDirOp(t *testing.T) {
	tempDir := t.TempDir()
	newDir := filepath.Join(tempDir, "newdir")
	cfg := &config.Config{Logger: slog.Default()}
	op := &CreateDirOp{Path: newDir, Config: cfg}

	err := op.Do()

	assert.NoError(t, err, "CreateDirOp.Do should succeed")
	info, err := os.Stat(newDir)
	assert.NoError(t, err, "CreateDirOp.Do should create directory")
	assert.True(t, info.IsDir(), "CreateDirOp.Do should create a directory, not file")

	err = op.Undo()

	assert.NoError(t, err, "CreateDirOp.Undo should succeed")
	assert.NoFileExists(t, newDir, "CreateDirOp.Undo should remove empty directory")
}

func TestCreateDirOpUndoNonEmpty(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{Logger: slog.Default()}
	newDir := filepath.Join(tempDir, "newdir")
	op := &CreateDirOp{Path: newDir, Config: cfg}

	err := op.Do()
	assert.NoError(t, err, "CreateDirOp.Do should succeed")

	if err := os.WriteFile(filepath.Join(newDir, "file"), []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create file in dir: %v", err)
	}

	err = op.Undo()

	assert.NoError(t, err, "CreateDirOp.Undo should succeed")
	assert.DirExists(t, newDir, "CreateDirOp.Undo should not remove non-empty directory")
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
			assert.NotEmpty(t, desc, "Describe() should not return empty string")
		})
	}
}

func TestTransactionExecuteAll(t *testing.T) {
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

	err := tx.ExecuteAll()

	assert.NoError(t, err, "ExecuteAll should succeed")
	assert.FileExists(t, dst, "ExecuteAll should have created dest file")
}

func TestTransactionRollback_UndoAll(t *testing.T) {
	cfg := &config.Config{Logger: slog.Default()}
	tx := NewTransaction(cfg)

	op1 := &mockOperation{}
	op2 := &mockOperation{}
	op3 := &mockOperation{}

	if err := tx.Execute(op1); err != nil {
		t.Fatalf("failed to execute op1: %v", err)
	}
	if err := tx.Execute(op2); err != nil {
		t.Fatalf("failed to execute op2: %v", err)
	}
	if err := tx.Execute(op3); err != nil {
		t.Fatalf("failed to execute op3: %v", err)
	}

	err := tx.Rollback()

	assert.NoError(t, err, "Rollback should succeed")
	assert.Equal(t, 1, op1.undoCalls, "op1.Undo() should be called once")
	assert.Equal(t, 1, op2.undoCalls, "op2.Undo() should be called once")
	assert.Equal(t, 1, op3.undoCalls, "op3.Undo() should be called once")
}

func TestTransactionRollback_PartialFailure(t *testing.T) {
	cfg := &config.Config{Logger: slog.Default()}
	tx := NewTransaction(cfg)

	op1 := &mockOperation{undoErr: nil}
	op2 := &mockOperation{undoErr: errors.New("undo failed")}
	op3 := &mockOperation{undoErr: nil}

	if err := tx.Execute(op1); err != nil {
		t.Fatalf("failed to execute op1: %v", err)
	}
	if err := tx.Execute(op2); err != nil {
		t.Fatalf("failed to execute op2: %v", err)
	}
	if err := tx.Execute(op3); err != nil {
		t.Fatalf("failed to execute op3: %v", err)
	}

	err := tx.Rollback()

	assert.Error(t, err, "Rollback should return error when undo fails")
	assert.Equal(t, 0, op1.undoCalls, "op1.Undo() should not be called (stops at op2 failure)")
	assert.Equal(t, 1, op2.undoCalls, "op2.Undo() should be called and fail")
	assert.Equal(t, 1, op3.undoCalls, "op3.Undo() should be called first")
}

type testPanicOp struct {
	panicMessage string
}

func (op *testPanicOp) Do() error {
	return nil
}

func (op *testPanicOp) Undo() error {
	panic(op.panicMessage)
}

func (op *testPanicOp) Describe() string {
	return "panic operation"
}

func TestTransactionPanicRecovery(t *testing.T) {
	cfg := &config.Config{Logger: slog.Default()}

	tx := NewTransaction(cfg)

	op := &testPanicOp{panicMessage: "test panic"}

	if err := tx.Execute(op); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	err := tx.Rollback()
	if err == nil {
		t.Error("should return error after panic")
	}

	if err != nil && !containsString(err.Error(), "panic") {
		t.Errorf("error should mention panic, got: %v", err)
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestRemoveFileOpBackupVerification(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	err := os.WriteFile(testFile, []byte("test"), 0644)
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	cfg := &config.Config{Logger: slog.Default()}

	op := &RemoveFileOp{
		Path:   testFile,
		config: cfg,
	}

	err = op.Do()
	if err != nil {
		t.Fatalf("Do failed: %v", err)
	}

	if op.backupPath == "" {
		t.Fatal("backup path should not be empty")
	}

	if _, err := os.Stat(op.backupPath); err != nil {
		t.Fatalf("backup file not created: %v", err)
	}
}
