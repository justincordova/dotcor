package core

import (
	"fmt"
	"os"

	"github.com/justincordova/dotcor/internal/config"
	"github.com/justincordova/dotcor/internal/fs"
)

// Operation represents a reversible operation
type Operation interface {
	Do() error        // Execute the operation
	Undo() error      // Rollback the operation
	Describe() string // Human-readable description
}

// Transaction represents a sequence of operations that can be rolled back.
//
// Two usage patterns:
// 1. Direct execution: call Execute(op) for each operation immediately
// 2. Planned execution: add operations to tx.operations, then call ExecuteAll()
//
// Both patterns track executed operations in 'executed' for rollback.
type Transaction struct {
	config     *config.Config
	operations []Operation
	executed   []Operation
	committed  bool
}

func NewTransaction(cfg *config.Config) *Transaction {
	return &Transaction{
		config:     cfg,
		operations: []Operation{},
		executed:   []Operation{},
		committed:  false,
	}
}

func (t *Transaction) Execute(op Operation) error {
	t.config.Logger.Debug("executing operation", "op", op.Describe())

	if t.committed {
		t.config.Logger.Error("transaction execute failed", "error", fmt.Errorf("already committed"))
		return fmt.Errorf("transaction already committed")
	}

	if err := op.Do(); err != nil {
		t.config.Logger.Error("operation failed, rolling back",
			"op", op.Describe(),
			"error", err,
		)
		rollbackErr := t.Rollback()
		if rollbackErr != nil {
			t.config.Logger.Error("rollback failed during execute",
				"error", rollbackErr,
			)
			return fmt.Errorf("executing %s: %w (rollback also failed: %v)", op.Describe(), err, rollbackErr)
		}
		return fmt.Errorf("executing %s: %w", op.Describe(), err)
	}

	t.executed = append(t.executed, op)
	t.config.Logger.Debug("operation executed successfully", "op", op.Describe())
	return nil
}

func (t *Transaction) Rollback() error {
	t.config.Logger.Warn("rolling back transaction", "operations", len(t.executed))

	if t.committed {
		t.config.Logger.Error("rollback failed", "error", fmt.Errorf("already committed"))
		return fmt.Errorf("cannot rollback committed transaction")
	}

	for i := len(t.executed) - 1; i >= 0; i-- {
		op := t.executed[i]
		t.config.Logger.Debug("rolling back operation", "op", op.Describe(), "index", i)
		if err := op.Undo(); err != nil {
			t.config.Logger.Error("rollback failed",
				"op", op.Describe(),
				"error", err,
				"index", i,
			)
			t.executed = nil
			return fmt.Errorf("rolling back %s: %w", op.Describe(), err)
		}
	}

	t.config.Logger.Info("transaction rolled back")
	t.executed = nil
	return nil
}

func (t *Transaction) Commit() {
	t.config.Logger.Debug("committing transaction", "operations", len(t.executed))
	t.committed = true
	t.executed = nil
}

// IsCommitted returns whether the transaction has been committed
func (t *Transaction) IsCommitted() bool {
	return t.committed
}

// ExecutedCount returns the number of operations executed
func (t *Transaction) ExecutedCount() int {
	return len(t.executed)
}

// ============================================================================
// Common Operations
// ============================================================================

// MoveFileOp moves a file from Src to Dst
type MoveFileOp struct {
	Src    string
	Dst    string
	Config *config.Config
}

func (op *MoveFileOp) Do() error {
	return fs.MoveFile(op.Src, op.Dst, op.Config)
}

func (op *MoveFileOp) Undo() error {
	return fs.MoveFile(op.Dst, op.Src, op.Config)
}

func (op *MoveFileOp) Describe() string {
	return fmt.Sprintf("move %s to %s", op.Src, op.Dst)
}

// CopyFileOp copies a file from Src to Dst
type CopyFileOp struct {
	Src    string
	Dst    string
	Config *config.Config
}

func (op *CopyFileOp) Do() error {
	return fs.CopyFile(op.Src, op.Dst, op.Config)
}

func (op *CopyFileOp) Undo() error {
	return os.Remove(op.Dst)
}

func (op *CopyFileOp) Describe() string {
	return fmt.Sprintf("copy %s to %s", op.Src, op.Dst)
}

// CreateSymlinkOp creates a symlink
type CreateSymlinkOp struct {
	Target string // The file the symlink points to
	Link   string // The symlink path
	Config *config.Config
}

func (op *CreateSymlinkOp) Do() error {
	return fs.CreateSymlink(op.Target, op.Link, op.Config)
}

func (op *CreateSymlinkOp) Undo() error {
	return os.Remove(op.Link)
}

func (op *CreateSymlinkOp) Describe() string {
	return fmt.Sprintf("create symlink %s -> %s", op.Link, op.Target)
}

// RemoveSymlinkOp removes a symlink (saves target for undo)
type RemoveSymlinkOp struct {
	Link        string
	savedTarget string // Saved for undo
	wasRelative bool
	Config      *config.Config
}

func (op *RemoveSymlinkOp) Do() error {
	// Save the target before removing
	target, err := fs.ReadSymlink(op.Link)
	if err != nil {
		return err
	}
	op.savedTarget = target

	isRel, _ := fs.IsRelativeSymlink(op.Link)
	op.wasRelative = isRel

	return os.Remove(op.Link)
}

func (op *RemoveSymlinkOp) Undo() error {
	// Use safe symlink creation with validation
	return fs.CreateSymlink(op.savedTarget, op.Link, op.Config)
}

func (op *RemoveSymlinkOp) Describe() string {
	return fmt.Sprintf("remove symlink %s", op.Link)
}

type RemoveFileOp struct {
	Path       string
	backupPath string
	config     *config.Config
}

func (op *RemoveFileOp) Do() error {
	backupPath, err := CreateBackup(op.Path, op.config)
	if err != nil {
		return fmt.Errorf("creating backup: %w", err)
	}
	op.backupPath = backupPath

	return os.Remove(op.Path)
}

func (op *RemoveFileOp) Undo() error {
	if op.backupPath == "" {
		return fmt.Errorf("no backup available for undo")
	}
	return RestoreBackup(op.backupPath, op.Path, op.config)
}

func (op *RemoveFileOp) Describe() string {
	return fmt.Sprintf("remove file %s", op.Path)
}

// CreateDirOp creates a directory
type CreateDirOp struct {
	Path   string
	Config *config.Config
}

func (op *CreateDirOp) Do() error {
	return fs.EnsureDir(op.Path, op.Config)
}

func (op *CreateDirOp) Undo() error {
	// Only remove if directory is empty
	entries, err := os.ReadDir(op.Path)
	if err != nil {
		return err
	}
	if len(entries) > 0 {
		return nil // Don't remove non-empty directories
	}
	return os.Remove(op.Path)
}

func (op *CreateDirOp) Describe() string {
	return fmt.Sprintf("create directory %s", op.Path)
}

// AddToConfigOp adds a managed file to config
type AddToConfigOp struct {
	Config    *config.Config
	File      config.ManagedFile
	fileIndex int // Track index for proper undo
}

func (op *AddToConfigOp) Do() error {
	op.Config.ManagedFiles = append(op.Config.ManagedFiles, op.File)
	op.fileIndex = len(op.Config.ManagedFiles) - 1
	return op.Config.SaveConfig()
}

func (op *AddToConfigOp) Undo() error {
	if op.fileIndex >= 0 && op.fileIndex < len(op.Config.ManagedFiles) {
		op.Config.ManagedFiles = append(
			op.Config.ManagedFiles[:op.fileIndex],
			op.Config.ManagedFiles[op.fileIndex+1:]...,
		)
		return op.Config.SaveConfig()
	}
	return fmt.Errorf("invalid file index: %d", op.fileIndex)
}

func (op *AddToConfigOp) Describe() string {
	return fmt.Sprintf("add %s to config", op.File.SourcePath)
}

// RemoveFromConfigOp removes a managed file from config
type RemoveFromConfigOp struct {
	Config     *config.Config
	savedFile  config.ManagedFile // Saved for undo
	sourcePath string
	fileIndex  int // Track index of removed file
}

func (op *RemoveFromConfigOp) Do() error {
	// Find by index instead of source path
	for i, mf := range op.Config.ManagedFiles {
		if mf.SourcePath == op.sourcePath {
			op.savedFile = mf
			op.fileIndex = i
			op.Config.ManagedFiles = append(
				op.Config.ManagedFiles[:i],
				op.Config.ManagedFiles[i+1:]...,
			)
			return op.Config.SaveConfig()
		}
	}
	return fmt.Errorf("managed file not found: %s", op.sourcePath)
}

func (op *RemoveFromConfigOp) Undo() error {
	// Insert back at correct position
	op.Config.ManagedFiles = append(
		op.Config.ManagedFiles[:op.fileIndex],
		append([]config.ManagedFile{op.savedFile}, op.Config.ManagedFiles[op.fileIndex:]...)...,
	)
	return op.Config.SaveConfig()
}

func (op *RemoveFromConfigOp) Describe() string {
	return fmt.Sprintf("remove %s from config", op.sourcePath)
}

// WriteFileOp writes content to a file (backs up existing for undo)
type WriteFileOp struct {
	Path       string
	Content    []byte
	Mode       os.FileMode
	backupPath string
	existed    bool
	config     *config.Config
}

func (op *WriteFileOp) Do() error {
	if fs.PathExists(op.Path) {
		op.existed = true
		backupPath, err := CreateBackup(op.Path, op.config)
		if err != nil {
			return fmt.Errorf("creating backup: %w", err)
		}
		op.backupPath = backupPath
	}

	return os.WriteFile(op.Path, op.Content, op.Mode)
}

func (op *WriteFileOp) Undo() error {
	if op.existed && op.backupPath != "" {
		return RestoreBackup(op.backupPath, op.Path, op.config)
	}
	return os.Remove(op.Path)
}

func (op *WriteFileOp) Describe() string {
	return fmt.Sprintf("write file %s", op.Path)
}

// ============================================================================
// Compound Operations (for convenience)
// ============================================================================

// AddFileTransaction creates a transaction for adding a file to dotcor.
// It builds a planned transaction - call ExecuteAll() to run the operations.
// Steps: move to repo -> create symlink -> add to config
// Note: Backup is handled separately by the caller (backups are kept regardless of rollback).
func AddFileTransaction(cfg *config.Config, sourcePath string, repoPath string, mf config.ManagedFile) (*Transaction, error) {
	tx := NewTransaction(cfg)

	fullRepoPath, err := config.GetRepoFilePath(cfg, repoPath)
	if err != nil {
		return nil, err
	}

	// Expand source path
	expandedSource, err := config.ExpandPath(sourcePath)
	if err != nil {
		return nil, err
	}

	// 1. Move file to repo
	tx.operations = append(tx.operations, &MoveFileOp{
		Src: expandedSource,
		Dst: fullRepoPath,
	})

	// 2. Create symlink
	tx.operations = append(tx.operations, &CreateSymlinkOp{
		Target: fullRepoPath,
		Link:   expandedSource,
	})

	// 3. Add to config
	tx.operations = append(tx.operations, &AddToConfigOp{
		Config: cfg,
		File:   mf,
	})

	return tx, nil
}

// ExecuteAll executes all operations in the transaction
func (t *Transaction) ExecuteAll() error {
	for _, op := range t.operations {
		if err := t.Execute(op); err != nil {
			return err
		}
	}
	return nil
}
