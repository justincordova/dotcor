package core

import (
	"fmt"
	"os"

	"github.com/justincordova/dotcor/internal/config"
	"github.com/justincordova/dotcor/internal/fs"
)

type Operation interface {
	Do() error
	Undo() error
	Describe() string
}

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
		t.config.Logger.Debug("transaction execute failed", "error", fmt.Errorf("already committed"))
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

// Rollback walks every executed operation in reverse and calls Undo on
// each one. Failures are recorded but do NOT abort the walk — every
// remaining operation is still attempted. The previous behaviour halted
// on the first undo error and left earlier ops in their post-Do state,
// silently breaking the documented "rolls back to a clean pre-state"
// contract: if op 5 of 10 failed to undo, ops 1–4 stayed mutated with
// no further attempt to revert them.
//
// A panic in any Undo is caught, converted to an error, and the walk
// continues — same shape as internal/stow/txn.go's fileTxn.rollback,
// which is the pattern this mirrors.
//
// The first error encountered (whether from undo failure or panic) is
// returned so the caller still sees that something went wrong.
func (t *Transaction) Rollback() error {
	t.config.Logger.Warn("rolling back transaction", "operations", len(t.executed))

	if t.committed {
		t.config.Logger.Error("rollback failed", "error", fmt.Errorf("already committed"))
		return fmt.Errorf("cannot rollback committed transaction")
	}

	var firstErr error
	for i := len(t.executed) - 1; i >= 0; i-- {
		op := t.executed[i]
		t.config.Logger.Debug("rolling back operation", "op", op.Describe(), "index", i)

		func() {
			defer func() {
				if r := recover(); r != nil {
					t.config.Logger.Error("panic in rollback", "op", op.Describe(), "error", r)
					if firstErr == nil {
						firstErr = fmt.Errorf("panic during rollback of %s: %v", op.Describe(), r)
					}
				}
			}()

			if err := op.Undo(); err != nil {
				t.config.Logger.Error("rollback step failed",
					"op", op.Describe(),
					"error", err,
					"index", i,
				)
				if firstErr == nil {
					firstErr = fmt.Errorf("rolling back %s: %w", op.Describe(), err)
				}
			}
		}()
	}

	t.executed = nil
	if firstErr != nil {
		return firstErr
	}
	t.config.Logger.Info("transaction rolled back")
	return nil
}

func (t *Transaction) Commit() {
	t.config.Logger.Debug("committing transaction", "operations", len(t.executed))
	t.committed = true
	t.executed = nil
}

func (t *Transaction) IsCommitted() bool {
	return t.committed
}

func (t *Transaction) ExecutedCount() int {
	return len(t.executed)
}

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

type CreateSymlinkOp struct {
	Target string
	Link   string
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

type RemoveSymlinkOp struct {
	Link        string
	savedTarget string
	wasRelative bool
	Config      *config.Config
}

func (op *RemoveSymlinkOp) Do() error {
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
	if backupPath == "" {
		return fmt.Errorf("backup creation failed - no backup path returned")
	}

	if !fs.PathExists(backupPath) {
		op.config.Logger.Error("backup file does not exist", "path", backupPath)
		return fmt.Errorf("backup file does not exist: %s", backupPath)
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

type CreateDirOp struct {
	Path   string
	Config *config.Config
}

func (op *CreateDirOp) Do() error {
	return fs.EnsureDir(op.Path, op.Config)
}

func (op *CreateDirOp) Undo() error {
	entries, err := os.ReadDir(op.Path)
	if err != nil {
		return err
	}
	if len(entries) > 0 {
		return nil
	}
	return os.Remove(op.Path)
}

func (op *CreateDirOp) Describe() string {
	return fmt.Sprintf("create directory %s", op.Path)
}

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

func (t *Transaction) ExecuteAll() error {
	for _, op := range t.operations {
		if err := t.Execute(op); err != nil {
			return err
		}
	}
	return nil
}
