package core

import (
	"errors"
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
	operations []Operation // Planned operations (for ExecuteAll pattern)
	executed   []Operation // Operations that have been executed (for rollback)
	committed  bool
}

// NewTransaction creates a new transaction
func NewTransaction() *Transaction {
	return &Transaction{
		operations: []Operation{},
		executed:   []Operation{},
		committed:  false,
	}
}

// Execute runs an operation and registers it for potential rollback
func (t *Transaction) Execute(op Operation) error {
	if t.committed {
		return fmt.Errorf("transaction already committed")
	}

	if err := op.Do(); err != nil {
		// Operation failed, rollback all previously executed operations
		t.Rollback()
		return fmt.Errorf("executing %s: %w", op.Describe(), err)
	}

	t.executed = append(t.executed, op)
	return nil
}

// Rollback undoes all executed operations in reverse order.
// If multiple rollback operations fail, all errors are collected and joined.
func (t *Transaction) Rollback() error {
	if t.committed {
		return fmt.Errorf("cannot rollback committed transaction")
	}

	var errs []error

	// Undo in reverse order
	for i := len(t.executed) - 1; i >= 0; i-- {
		op := t.executed[i]
		if err := op.Undo(); err != nil {
			// Continue rolling back other operations even if one fails
			errs = append(errs, fmt.Errorf("rolling back %s: %w", op.Describe(), err))
		}
	}

	t.executed = nil

	if len(errs) > 0 {
		return fmt.Errorf("rollback errors: %w", errors.Join(errs...))
	}
	return nil
}

// Commit marks transaction as successful (clears rollback list)
func (t *Transaction) Commit() {
	t.committed = true
	t.executed = nil // Clear executed list, no longer needed
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
	Src string
	Dst string
}

func (op *MoveFileOp) Do() error {
	return fs.MoveFile(op.Src, op.Dst)
}

func (op *MoveFileOp) Undo() error {
	return fs.MoveFile(op.Dst, op.Src)
}

func (op *MoveFileOp) Describe() string {
	return fmt.Sprintf("move %s to %s", op.Src, op.Dst)
}

// CopyFileOp copies a file from Src to Dst
type CopyFileOp struct {
	Src string
	Dst string
}

func (op *CopyFileOp) Do() error {
	return fs.CopyFile(op.Src, op.Dst)
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
}

func (op *CreateSymlinkOp) Do() error {
	return fs.CreateSymlink(op.Target, op.Link)
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
	return fs.CreateSymlink(op.savedTarget, op.Link)
}

func (op *RemoveSymlinkOp) Describe() string {
	return fmt.Sprintf("remove symlink %s", op.Link)
}

// RemoveFileOp removes a file (backs up for undo)
type RemoveFileOp struct {
	Path       string
	backupPath string // Backup path for undo
}

func (op *RemoveFileOp) Do() error {
	// Create backup before removing
	backupPath, err := CreateBackup(op.Path)
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
	return RestoreBackup(op.backupPath, op.Path)
}

func (op *RemoveFileOp) Describe() string {
	return fmt.Sprintf("remove file %s", op.Path)
}

// CreateDirOp creates a directory
type CreateDirOp struct {
	Path string
}

func (op *CreateDirOp) Do() error {
	return fs.EnsureDir(op.Path)
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
}

func (op *WriteFileOp) Do() error {
	// Check if file exists and backup
	if fs.FileExists(op.Path) {
		op.existed = true
		backupPath, err := CreateBackup(op.Path)
		if err != nil {
			return fmt.Errorf("creating backup: %w", err)
		}
		op.backupPath = backupPath
	}

	return os.WriteFile(op.Path, op.Content, op.Mode)
}

func (op *WriteFileOp) Undo() error {
	if op.existed && op.backupPath != "" {
		return RestoreBackup(op.backupPath, op.Path)
	}
	// File didn't exist before, remove it
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
	tx := NewTransaction()

	// Get full repo file path
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
