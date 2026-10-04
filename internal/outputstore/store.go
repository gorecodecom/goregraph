// Package outputstore publishes validated output directories under recoverable
// transactions. Built-in readers must use WithRead to obtain a stable snapshot.
package outputstore

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/gorecodecom/goregraph/internal/pathutil"
)

// ErrRecoveryRequired means a previous publication must be explicitly recovered.
var ErrRecoveryRequired = errors.New("output publication requires recovery")

const generationFile = ".goregraph-generation"

// UpdateRequest describes one complete output snapshot. Write receives a stage
// prepopulated with the previous snapshot. Validate runs before any promotion.
type UpdateRequest struct {
	Root     string
	Write    func(stage string) error
	Validate func(stage string) error
}

type journalEntry struct {
	Root               string `json:"root"`
	Stage              string `json:"stage"`
	Backup             string `json:"backup"`
	Existed            bool   `json:"existed"`
	PreviousGeneration string `json:"previous_generation"`
}

type journal struct {
	Version    int            `json:"version"`
	Generation string         `json:"generation"`
	State      string         `json:"state"`
	Entries    []journalEntry `json:"entries"`
}

// fileOperations isolates fallible filesystem mutations while retaining real
// filesystem reads and transaction decisions.
type fileOperations struct {
	rename    func(string, string) error
	mkdir     func(string, fs.FileMode) error
	removeAll func(string) error
	remove    func(string) error
	sync      func(*os.File) error
}

func systemOperations() fileOperations {
	return fileOperations{rename: durableRename, mkdir: os.Mkdir, removeAll: os.RemoveAll, remove: os.Remove, sync: (*os.File).Sync}
}

// Update publishes one output after recovering its interrupted prior update.
func Update(ctx context.Context, request UpdateRequest) error {
	return UpdateMany(ctx, []UpdateRequest{request})
}

// UpdateMany stages every output before publishing any of them. Locks use a
// canonical order. A failed promotion rolls every root back to its old snapshot.
func UpdateMany(ctx context.Context, requests []UpdateRequest) error {
	return updateMany(ctx, requests, systemOperations())
}

// UpdateManyValidated validates shared inputs once after all stages have been
// validated and synced, while all output locks are held and before promotion.
func UpdateManyValidated(ctx context.Context, requests []UpdateRequest, validate func() error) error {
	if validate == nil {
		return errors.New("shared output validation is required")
	}
	return updateManyWithValidation(ctx, requests, systemOperations(), validate)
}

// UpdateManyPrepared prepares related snapshots together before validating or
// publishing any of them. The callback receives private, prepopulated stages
// keyed by canonical output roots. Progress reports copied snapshot counts.
func UpdateManyPrepared(ctx context.Context, requests []UpdateRequest, prepare func(map[string]string) error, progress func(string, int, int)) error {
	if prepare == nil {
		return errors.New("shared output preparation is required")
	}
	return updateManyPrepared(ctx, requests, systemOperations(), prepare, nil, progress)
}

func updateMany(ctx context.Context, requests []UpdateRequest, ops fileOperations) error {
	return updateManyWithValidation(ctx, requests, ops, nil)
}

func updateManyWithValidation(ctx context.Context, requests []UpdateRequest, ops fileOperations, validate func() error) error {
	return updateManyPrepared(ctx, requests, ops, nil, validate, nil)
}

func updateManyPrepared(ctx context.Context, requests []UpdateRequest, ops fileOperations, prepare func(map[string]string) error, validate func() error, progress func(string, int, int)) error {
	ordered, err := normalizeRequests(requests)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	for _, request := range ordered {
		if err := Recover(ctx, request.Root); err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(request.Root), 0755); err != nil {
			return err
		}
	}
	writers, err := acquireWriterLocks(ctx, requestRoots(ordered))
	if err != nil {
		return err
	}
	defer releaseLocks(writers)
	locks, err := acquireRoots(ctx, requestRoots(ordered), true)
	if err != nil {
		return err
	}
	defer func() { releaseLocks(locks) }()
	for _, request := range ordered {
		if err := rejectPendingJournal(request.Root); err != nil {
			return err
		}
		if err := removeAbandonedStages(request.Root, ops); err != nil {
			return err
		}
	}
	transaction, err := prepareJournal(ordered)
	if err != nil {
		return err
	}
	failPrepared := func(cause error) error { return errors.Join(cause, discardStages(transaction, ops)) }
	if prepare != nil {
		stages := make(map[string]string, len(ordered))
		for i, entry := range transaction.Entries {
			if err := ctx.Err(); err != nil {
				return failPrepared(err)
			}
			if progress != nil {
				progress(entry.Root, i, len(ordered))
			}
			if err := ops.mkdir(entry.Stage, 0755); err != nil {
				return failPrepared(err)
			}
			if entry.Existed {
				if err := copyTree(ctx, entry.Root, entry.Stage, ops); err != nil {
					return failPrepared(err)
				}
			}
			stages[entry.Root] = entry.Stage
			if progress != nil {
				progress(entry.Root, i+1, len(ordered))
			}
		}
		if err := prepare(stages); err != nil {
			return failPrepared(err)
		}
	}
	for i, request := range ordered {
		if err := ctx.Err(); err != nil {
			return failPrepared(err)
		}
		entry := transaction.Entries[i]
		failStage := func(err error) error { return failPrepared(fmt.Errorf("stage output %s: %w", entry.Root, err)) }
		if prepare == nil {
			if err := ops.mkdir(entry.Stage, 0755); err != nil {
				return failStage(err)
			}
			if entry.Existed {
				if err := copyTree(ctx, entry.Root, entry.Stage, ops); err != nil {
					return failStage(err)
				}
			}
		}
		if err := request.Write(entry.Stage); err != nil {
			return failStage(err)
		}
		if err := directoryExists(entry.Stage); err != nil {
			return failStage(err)
		}
		if err := ctx.Err(); err != nil {
			return failPrepared(err)
		}
		if err := writeGeneration(entry.Stage, transaction.Generation, ops); err != nil {
			return failStage(err)
		}
		if request.Validate != nil {
			if err := request.Validate(entry.Stage); err != nil {
				return failStage(err)
			}
		}
		if err := syncTree(ctx, entry.Stage, ops); err != nil {
			return failStage(err)
		}
		if generation, err := readGeneration(entry.Stage); err != nil || generation != transaction.Generation {
			return failPrepared(errors.Join(err, errors.New("staged generation marker changed")))
		}
	}
	if err := ctx.Err(); err != nil {
		return failPrepared(err)
	}
	if validate != nil {
		if err := validate(); err != nil {
			return failPrepared(err)
		}
		if err := ctx.Err(); err != nil {
			return failPrepared(err)
		}
	}
	// Preparation uses shared snapshot locks. Only the recoverable publication
	// excludes readers, after every new snapshot and input check is ready.
	releaseLocks(locks)
	locks = nil
	locks, err = acquireRoots(ctx, requestRoots(ordered), false)
	if err != nil {
		return failPrepared(err)
	}
	for _, entry := range transaction.Entries {
		if err := rejectPendingJournal(entry.Root); err != nil {
			return failPrepared(err)
		}
		if err := verifyPrevious(entry); err != nil {
			return failPrepared(err)
		}
	}
	if err := ctx.Err(); err != nil {
		return failPrepared(err)
	}
	if err := persistPrepared(transaction, ops); err != nil {
		return errors.Join(err, cleanupPrepared(transaction, ops))
	}
	transaction.State = "publishing"
	if err := writeJournal(journalPath(transaction.Entries[0].Root), transaction, ops); err != nil {
		return errors.Join(err, fmt.Errorf("%w: publishing decision", ErrRecoveryRequired))
	}
	for _, entry := range transaction.Entries {
		if err := ctx.Err(); err != nil {
			return rollbackFailure(transaction, ops, err)
		}
		if err := verifyPrevious(entry); err != nil {
			return rollbackFailure(transaction, ops, err)
		}
		if entry.Existed {
			if err := ops.rename(entry.Root, entry.Backup); err != nil {
				return rollbackFailure(transaction, ops, err)
			}
		}
		if err := ops.rename(entry.Stage, entry.Root); err != nil {
			return rollbackFailure(transaction, ops, err)
		}
	}
	if err := ctx.Err(); err != nil {
		return rollbackFailure(transaction, ops, err)
	}
	transaction.State = "committed"
	if err := writeJournal(journalPath(transaction.Entries[0].Root), transaction, ops); err != nil {
		// A failed flush after the rename has an uncertain commit outcome. Keep
		// backups and journals; recovery reads the durable decision before acting.
		return errors.Join(err, fmt.Errorf("%w: commit decision", ErrRecoveryRequired))
	}
	if err := cleanupCommitted(transaction, ops); err != nil {
		return errors.Join(err, ErrRecoveryRequired)
	}
	return nil
}

// WithRead holds an OS-backed shared lock for the callback. It never recovers or
// modifies a pending publication, and returns ErrRecoveryRequired for journals.
func WithRead(ctx context.Context, root string, read func(committed string) error) error {
	if read == nil {
		return errors.New("output reader is required")
	}
	canonical, err := canonicalRoot(root)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	initializing, err := acquireInitializationLocks(ctx, []string{canonical})
	if err != nil {
		return err
	}
	defer releaseLocks(initializing)
	lock, err := acquireFileLock(ctx, lockPath(canonical), true)
	if err != nil {
		return err
	}
	defer lock.release()
	if err := rejectPendingJournal(canonical); err != nil {
		return err
	}
	if err := directoryExists(canonical); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := read(canonical); err != nil {
		return err
	}
	return ctx.Err()
}

// WithReads holds shared locks for a set of optional output roots. Existing
// parents are required; absent output directories remain available for caller
// fallback. Duplicate roots share one lock and callbacks must not nest loads.
func WithReads(ctx context.Context, roots []string, read func() error) error {
	return withReads(ctx, roots, read, true)
}

// WithExistingReads holds shared output locks without creating missing lock files.
// Missing locks fail closed; callers must arrange authorized initialization.
func WithExistingReads(ctx context.Context, roots []string, read func() error) error {
	return withReads(ctx, roots, read, false)
}

func withReads(ctx context.Context, roots []string, read func() error, createMissing bool) error {
	return withReadDependencies(ctx, roots, nil, read, createMissing)
}

// WithReadDependencies locks every output, but waits for first publication only
// at required roots. A committed workspace can therefore be read while an
// additional, not-yet-indexed project is being prepared.
func WithReadDependencies(ctx context.Context, required, dependencies []string, read func() error, createMissing bool) error {
	if len(required) == 0 {
		return errors.New("at least one required read output is needed")
	}
	return withReadDependencies(ctx, required, dependencies, read, createMissing)
}

func withReadDependencies(ctx context.Context, required, dependencies []string, read func() error, createMissing bool) error {
	if read == nil {
		return errors.New("output reader is required")
	}
	roots := append(append([]string(nil), required...), dependencies...)
	canonical := make([]string, 0, len(roots))
	seen := make(map[string]bool)
	initializationKeys := make(map[string]bool)
	for i, root := range roots {
		value, err := canonicalRoot(root)
		if err != nil {
			return fmt.Errorf("resolve output root %s: %w", root, err)
		}
		if err := directoryExists(filepath.Dir(value)); err != nil {
			return err
		}
		if i < len(required) {
			initializationKeys[pathKey(value)] = true
		}
		if !seen[pathKey(value)] {
			seen[pathKey(value)] = true
			canonical = append(canonical, value)
		}
	}
	sort.Slice(canonical, func(i, j int) bool { return pathKey(canonical[i]) < pathKey(canonical[j]) })
	var initializationRoots []string
	for _, root := range canonical {
		if initializationKeys[pathKey(root)] {
			initializationRoots = append(initializationRoots, root)
		}
	}
	initializing, err := acquireInitializationLocks(ctx, initializationRoots)
	if err != nil {
		return err
	}
	defer releaseLocks(initializing)
	locks, err := acquireRootsMode(ctx, canonical, true, createMissing)
	if err != nil {
		return err
	}
	defer releaseLocks(locks)
	for _, root := range canonical {
		if err := rejectPendingJournal(root); err != nil {
			return err
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := read(); err != nil {
		return err
	}
	return ctx.Err()
}

// Recover explicitly repairs one interrupted publication, including its sibling
// outputs. Missing journals are a no-op. Recovery is safe to repeat.
func Recover(ctx context.Context, root string) error {
	canonical, err := canonicalRoot(root)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	initial, err := readJournal(journalPath(canonical))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return errors.Join(ErrRecoveryRequired, err)
	}
	if err := validateJournalMetadata(initial, canonical); err != nil {
		return errors.Join(ErrRecoveryRequired, err)
	}
	decision, err := readJournal(journalPath(initial.Entries[0].Root))
	if err != nil {
		return errors.Join(ErrRecoveryRequired, err)
	}
	if !sameTransaction(initial, decision) {
		return fmt.Errorf("%w: conflicting coordinator", ErrRecoveryRequired)
	}
	if decision.State == "finalized" {
		return recoverFinalized(ctx, decision)
	}
	roots, absent, err := recoveryRoots(decision)
	if err != nil {
		return errors.Join(ErrRecoveryRequired, err)
	}
	locks, err := acquireRoots(ctx, roots, false)
	if err != nil {
		latest, readErr := readJournal(journalPath(initial.Entries[0].Root))
		if readErr == nil && sameTransaction(initial, latest) && latest.State == "finalized" {
			return recoverFinalized(ctx, latest)
		}
		return err
	}
	defer releaseLocks(locks)
	local, err := readJournal(journalPath(canonical))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return errors.Join(ErrRecoveryRequired, err)
	}
	if !sameTransaction(initial, local) {
		return fmt.Errorf("%w: journal changed while acquiring locks", ErrRecoveryRequired)
	}
	coordinator, err := readJournal(journalPath(initial.Entries[0].Root))
	if err != nil {
		return errors.Join(ErrRecoveryRequired, err)
	}
	if !sameTransaction(initial, coordinator) {
		return fmt.Errorf("%w: conflicting coordinator", ErrRecoveryRequired)
	}
	if err := validateJournal(coordinator, canonical); err != nil {
		return errors.Join(ErrRecoveryRequired, err)
	}
	if len(absent) > 0 && coordinator.State != decision.State {
		return fmt.Errorf("%w: coordinator changed while acquiring locks", ErrRecoveryRequired)
	}
	for root := range absent {
		if _, err := os.Lstat(filepath.Dir(root)); !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("%w: absent output parent reappeared", ErrRecoveryRequired)
		}
	}
	if coordinator.State == "finalized" {
		if err := removeJournals(coordinator, systemOperations()); err != nil {
			return errors.Join(ErrRecoveryRequired, err)
		}
		return nil
	}
	for _, entry := range coordinator.Entries {
		peer, err := readJournal(journalPath(entry.Root))
		if errors.Is(err, os.ErrNotExist) && coordinator.State == "prepared" {
			continue
		}
		if err != nil {
			return errors.Join(ErrRecoveryRequired, err)
		}
		if !sameTransaction(coordinator, peer) {
			return fmt.Errorf("%w: conflicting sibling journal", ErrRecoveryRequired)
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	ops := systemOperations()
	switch coordinator.State {
	case "prepared":
		err = cleanupPreparedExcept(coordinator, ops, absent)
	case "publishing":
		err = rollback(coordinator, ops)
	case "committed":
		err = cleanupCommitted(coordinator, ops)
	case "cleaning":
		err = resumeCommittedCleanup(coordinator, ops)
	}
	if err != nil {
		return errors.Join(ErrRecoveryRequired, err)
	}
	return nil
}

func recoveryRoots(transaction journal) ([]string, map[string]bool, error) {
	var roots []string
	absent := make(map[string]bool)
	for _, entry := range transaction.Entries {
		// Preparation has not promoted any output. A moved project must not
		// prevent discarding preparation for the remaining unchanged outputs.
		if transaction.State == "prepared" || transaction.State == "finalized" {
			if _, err := os.Lstat(filepath.Dir(entry.Root)); errors.Is(err, os.ErrNotExist) {
				absent[entry.Root] = true
				continue
			} else if err != nil {
				return nil, nil, err
			}
		}
		roots = append(roots, entry.Root)
	}
	return roots, absent, nil
}

func recoverFinalized(ctx context.Context, transaction journal) error {
	if err := validateJournalMetadata(transaction, transaction.Entries[0].Root); err != nil {
		return errors.Join(ErrRecoveryRequired, err)
	}
	roots, absent, err := recoveryRoots(transaction)
	if err != nil {
		return err
	}
	locks, err := acquireRoots(ctx, roots, false)
	if err != nil {
		return err
	}
	defer releaseLocks(locks)
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := removeJournalsExcept(transaction, systemOperations(), absent); err != nil {
		return errors.Join(ErrRecoveryRequired, err)
	}
	return nil
}

func normalizeRequests(requests []UpdateRequest) ([]UpdateRequest, error) {
	if len(requests) == 0 {
		return nil, errors.New("at least one output is required")
	}
	ordered := append([]UpdateRequest(nil), requests...)
	for i := range ordered {
		if ordered[i].Write == nil {
			return nil, errors.New("output writer is required")
		}
		root, err := canonicalRoot(ordered[i].Root)
		if err != nil {
			return nil, fmt.Errorf("resolve output root %s: %w", ordered[i].Root, err)
		}
		ordered[i].Root = root
	}
	sort.Slice(ordered, func(i, j int) bool { return pathKey(ordered[i].Root) < pathKey(ordered[j].Root) })
	for i := range ordered {
		for j := 0; j < i; j++ {
			if sameOrNested(ordered[j].Root, ordered[i].Root) || sameOrNested(ordered[i].Root, ordered[j].Root) {
				return nil, errors.New("output roots must be distinct and non-nested")
			}
		}
	}
	return ordered, nil
}

func canonicalRoot(root string) (string, error) {
	if strings.TrimSpace(root) == "" {
		return "", errors.New("output root is empty")
	}
	abs, err := filepath.Abs(filepath.Clean(root))
	if err != nil {
		return "", err
	}
	if filepath.Dir(abs) == abs {
		return "", errors.New("filesystem root cannot be an output")
	}
	for _, prefix := range []string{".goregraph-stage-", ".goregraph-backup-", ".goregraph-journal-", ".goregraph-lock-"} {
		if strings.HasPrefix(strings.ToLower(filepath.Base(abs)), prefix) {
			return "", errors.New("transaction paths cannot be output roots")
		}
	}
	if _, err := os.Lstat(filepath.Join(abs, ".git")); err == nil {
		return "", errors.New("source repository cannot be an output root")
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	if info, err := os.Lstat(abs); err == nil && (info.Mode()&os.ModeSymlink != 0 || !info.IsDir()) {
		return "", errors.New("output root must be a real directory")
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	parent := filepath.Dir(abs)
	var missing []string
	for {
		_, err := os.Stat(parent)
		if err == nil {
			break
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		missing = append(missing, filepath.Base(parent))
		next := filepath.Dir(parent)
		if next == parent {
			return "", err
		}
		parent = next
	}
	resolved, err := pathutil.Resolve(parent)
	if err != nil {
		return "", err
	}
	for i := len(missing) - 1; i >= 0; i-- {
		resolved = filepath.Join(resolved, missing[i])
	}
	return filepath.Join(resolved, filepath.Base(abs)), nil
}

func pathKey(path string) string {
	if runtime.GOOS == "windows" {
		return strings.ToLower(filepath.Clean(path))
	}
	return filepath.Clean(path)
}

func sameOrNested(parent, child string) bool {
	rel, err := filepath.Rel(pathKey(parent), pathKey(child))
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

func rootKey(root string) string {
	digest := sha256.Sum256([]byte(pathKey(root)))
	return hex.EncodeToString(digest[:12])
}
func lockPath(root string) string {
	return filepath.Join(filepath.Dir(root), ".goregraph-lock-"+rootKey(root)+".lock")
}
func journalPath(root string) string {
	return filepath.Join(filepath.Dir(root), ".goregraph-journal-"+rootKey(root)+".json")
}
func stagePath(root, generation string) string {
	return filepath.Join(filepath.Dir(root), ".goregraph-stage-"+generation+"-"+rootKey(root))
}
func backupPath(root, generation string) string {
	return filepath.Join(filepath.Dir(root), ".goregraph-backup-"+generation+"-"+rootKey(root))
}

func requestRoots(requests []UpdateRequest) []string {
	roots := make([]string, len(requests))
	for i, request := range requests {
		roots[i] = request.Root
	}
	return roots
}

func acquireRoots(ctx context.Context, roots []string, shared bool) ([]*fileLock, error) {
	return acquireRootsMode(ctx, roots, shared, true)
}

func acquireRootsMode(ctx context.Context, roots []string, shared, createMissing bool) ([]*fileLock, error) {
	var locks []*fileLock
	for _, root := range roots {
		lock, err := acquireFileLockMode(ctx, lockPath(root), shared, createMissing)
		if err != nil {
			releaseLocks(locks)
			return nil, fmt.Errorf("lock output %s: %w", root, err)
		}
		locks = append(locks, lock)
	}
	return locks, nil
}

func releaseLocks(locks []*fileLock) {
	for i := len(locks) - 1; i >= 0; i-- {
		_ = locks[i].release()
	}
}

func rejectPendingJournal(root string) error {
	_, err := os.Lstat(journalPath(root))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	return fmt.Errorf("%w: %s", ErrRecoveryRequired, root)
}

func prepareJournal(requests []UpdateRequest) (journal, error) {
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return journal{}, err
	}
	result := journal{Version: 1, Generation: hex.EncodeToString(random[:]), State: "prepared"}
	for _, request := range requests {
		entry := journalEntry{Root: request.Root, Stage: stagePath(request.Root, result.Generation), Backup: backupPath(request.Root, result.Generation)}
		if err := directoryExists(entry.Root); err == nil {
			entry.Existed = true
			generation, err := readGeneration(entry.Root)
			if err != nil {
				return journal{}, err
			}
			entry.PreviousGeneration = generation
		} else if !errors.Is(err, os.ErrNotExist) {
			return journal{}, err
		}
		for _, reserved := range []string{entry.Stage, entry.Backup} {
			if _, err := os.Lstat(reserved); !errors.Is(err, os.ErrNotExist) {
				if err == nil {
					err = errors.New("reserved transaction path exists")
				}
				return journal{}, err
			}
		}
		result.Entries = append(result.Entries, entry)
	}
	return result, nil
}

func persistPrepared(transaction journal, ops fileOperations) error {
	for _, entry := range transaction.Entries {
		if err := writeJournal(journalPath(entry.Root), transaction, ops); err != nil {
			return err
		}
	}
	return nil
}

func writeJournal(path string, value journal, ops fileOperations) error {
	body, err := json.Marshal(value)
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".goregraph-journal-tmp-")
	if err != nil {
		return err
	}
	temporary := file.Name()
	defer os.Remove(temporary)
	if err := file.Chmod(0600); err != nil {
		file.Close()
		return err
	}
	if _, err := file.Write(body); err != nil {
		file.Close()
		return err
	}
	if err := ops.sync(file); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return ops.rename(temporary, path)
}

func readJournal(path string) (journal, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return journal{}, err
	}
	if !info.Mode().IsRegular() || info.Size() > 1024*1024 {
		return journal{}, errors.New("unsafe publication journal")
	}
	file, err := os.Open(path)
	if err != nil {
		return journal{}, err
	}
	defer file.Close()
	decoder := json.NewDecoder(io.LimitReader(file, 1024*1024))
	decoder.DisallowUnknownFields()
	var value journal
	if err := decoder.Decode(&value); err != nil {
		return journal{}, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return journal{}, errors.New("trailing publication journal data")
	}
	return value, nil
}

func validateJournalMetadata(value journal, requested string) error {
	if value.Version != 1 || len(value.Entries) == 0 || len(value.Generation) != 32 {
		return errors.New("invalid publication journal header")
	}
	if _, err := hex.DecodeString(value.Generation); err != nil {
		return errors.New("invalid publication generation")
	}
	switch value.State {
	case "prepared", "publishing", "committed", "cleaning", "finalized":
	default:
		return errors.New("invalid publication state")
	}
	found := false
	for i, entry := range value.Entries {
		if !filepath.IsAbs(entry.Root) || filepath.Clean(entry.Root) != entry.Root || filepath.Dir(entry.Root) == entry.Root {
			return errors.New("noncanonical journal root")
		}
		if pathKey(entry.Root) == pathKey(requested) {
			found = true
		}
		if entry.Stage != stagePath(entry.Root, value.Generation) || entry.Backup != backupPath(entry.Root, value.Generation) {
			return errors.New("journal path escapes intended output parent")
		}
		if i > 0 && pathKey(value.Entries[i-1].Root) >= pathKey(entry.Root) {
			return errors.New("unordered or duplicate journal roots")
		}
		for j := 0; j < i; j++ {
			if sameOrNested(value.Entries[j].Root, entry.Root) || sameOrNested(entry.Root, value.Entries[j].Root) {
				return errors.New("nested journal roots")
			}
		}
	}
	if !found {
		return errors.New("journal does not include requested output")
	}
	return nil
}

func validateJournal(value journal, requested string) error {
	if err := validateJournalMetadata(value, requested); err != nil {
		return err
	}
	if value.State == "finalized" {
		return nil
	}
	for _, entry := range value.Entries {
		canonical, err := canonicalRoot(entry.Root)
		if err != nil {
			return err
		}
		if pathKey(canonical) != pathKey(entry.Root) {
			return errors.New("noncanonical journal root")
		}
		for _, reserved := range []string{entry.Stage, entry.Backup} {
			if info, err := os.Lstat(reserved); err == nil && (!info.IsDir() || info.Mode()&os.ModeSymlink != 0) {
				return errors.New("unsafe transaction directory")
			} else if err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
		}
	}
	return nil
}
func sameTransaction(left, right journal) bool {
	if left.Version != right.Version || left.Generation != right.Generation || len(left.Entries) != len(right.Entries) {
		return false
	}
	for i := range left.Entries {
		if left.Entries[i] != right.Entries[i] {
			return false
		}
	}
	return true
}

func directoryExists(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("output path is not a real directory")
	}
	return nil
}

func readGeneration(root string) (string, error) {
	path := filepath.Join(root, generationFile)
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() || info.Size() > 128 {
		return "", errors.New("unsafe generation marker")
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(body)), nil
}

func writeGeneration(stage, generation string, ops fileOperations) error {
	path := filepath.Join(stage, generationFile)
	if info, err := os.Lstat(path); err == nil && !info.Mode().IsRegular() {
		return errors.New("unsafe staged generation marker")
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	_, writeErr := file.WriteString(generation + "\n")
	if writeErr == nil {
		writeErr = ops.sync(file)
	}
	return errors.Join(writeErr, file.Close())
}

func verifyPrevious(entry journalEntry) error {
	err := directoryExists(entry.Root)
	if !entry.Existed {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err == nil {
			return errors.New("output appeared during staging")
		}
		return err
	}
	if err != nil {
		return err
	}
	generation, err := readGeneration(entry.Root)
	if err != nil {
		return err
	}
	if generation != entry.PreviousGeneration {
		return errors.New("output generation changed during staging")
	}
	return nil
}

func rollbackFailure(transaction journal, ops fileOperations, cause error) error {
	if err := rollback(transaction, ops); err != nil {
		return errors.Join(cause, err, ErrRecoveryRequired)
	}
	return cause
}

func rollback(transaction journal, ops fileOperations) error {
	if err := validateJournal(transaction, transaction.Entries[0].Root); err != nil {
		return err
	}
	// Validate every generation before touching any sibling output.
	for _, entry := range transaction.Entries {
		if err := validateRollbackEntry(entry, transaction.Generation); err != nil {
			return err
		}
	}
	for i := len(transaction.Entries) - 1; i >= 0; i-- {
		entry := transaction.Entries[i]
		if err := directoryExists(entry.Root); err == nil {
			generation, err := readGeneration(entry.Root)
			if err != nil {
				return err
			}
			if generation == transaction.Generation {
				if err := ops.rename(entry.Root, entry.Stage); err != nil {
					return err
				}
			}
		}
		if err := directoryExists(entry.Backup); err == nil {
			if err := ops.rename(entry.Backup, entry.Root); err != nil {
				return err
			}
		}
	}
	transaction.State = "prepared"
	if err := writeJournal(journalPath(transaction.Entries[0].Root), transaction, ops); err != nil {
		return err
	}
	return cleanupPrepared(transaction, ops)
}

func validateRollbackEntry(entry journalEntry, generation string) error {
	rootErr, backupErr := directoryExists(entry.Root), directoryExists(entry.Backup)
	if rootErr != nil && !errors.Is(rootErr, os.ErrNotExist) {
		return rootErr
	}
	if backupErr != nil && !errors.Is(backupErr, os.ErrNotExist) {
		return backupErr
	}
	if backupErr == nil {
		if !entry.Existed {
			return errors.New("unexpected backup for a new output")
		}
		old, err := readGeneration(entry.Backup)
		if err != nil {
			return err
		}
		if old != entry.PreviousGeneration {
			return errors.New("backup generation mismatch")
		}
	}
	if rootErr == nil {
		current, err := readGeneration(entry.Root)
		if err != nil {
			return err
		}
		if current == generation {
			if _, err := os.Lstat(entry.Stage); !errors.Is(err, os.ErrNotExist) {
				return errors.New("rollback stage is not vacant")
			}
			if entry.Existed && backupErr != nil {
				return errors.New("last good backup is missing")
			}
		} else if !entry.Existed || current != entry.PreviousGeneration || backupErr == nil {
			return errors.New("unexpected committed output during recovery")
		}
	} else if entry.Existed && backupErr != nil {
		return errors.New("last good output is missing")
	}
	return nil
}

func cleanupPrepared(transaction journal, ops fileOperations) error {
	return cleanupPreparedExcept(transaction, ops, nil)
}

func cleanupPreparedExcept(transaction journal, ops fileOperations, absent map[string]bool) error {
	if err := validateJournal(transaction, transaction.Entries[0].Root); err != nil {
		return errors.Join(err, ErrRecoveryRequired)
	}
	for _, entry := range transaction.Entries {
		if absent[entry.Root] {
			if _, err := os.Lstat(filepath.Dir(entry.Root)); !errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("%w: absent output parent reappeared", ErrRecoveryRequired)
			}
			continue
		}
		if err := verifyPrevious(entry); err != nil {
			return errors.Join(err, ErrRecoveryRequired)
		}
		if _, err := os.Lstat(entry.Backup); !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("%w: unexpected prepared backup", ErrRecoveryRequired)
		}
	}
	for _, entry := range transaction.Entries {
		if absent[entry.Root] {
			continue
		}
		if err := ops.removeAll(entry.Stage); err != nil {
			return errors.Join(err, ErrRecoveryRequired)
		}
	}
	return finalizePublicationExcept(transaction, ops, absent)
}

func cleanupCommitted(transaction journal, ops fileOperations) error {
	if err := validateJournal(transaction, transaction.Entries[0].Root); err != nil {
		return err
	}
	for _, entry := range transaction.Entries {
		if err := directoryExists(entry.Root); err != nil {
			return err
		}
		generation, err := readGeneration(entry.Root)
		if err != nil {
			return err
		}
		if generation != transaction.Generation {
			return errors.New("committed generation mismatch")
		}
		if err := directoryExists(entry.Backup); err == nil {
			old, err := readGeneration(entry.Backup)
			if err != nil {
				return err
			}
			if old != entry.PreviousGeneration {
				return errors.New("backup generation mismatch")
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	transaction.State = "cleaning"
	if err := writeJournal(journalPath(transaction.Entries[0].Root), transaction, ops); err != nil {
		return err
	}
	return resumeCommittedCleanup(transaction, ops)
}

func resumeCommittedCleanup(transaction journal, ops fileOperations) error {
	if err := validateJournal(transaction, transaction.Entries[0].Root); err != nil {
		return err
	}
	// The durable cleaning decision authorizes deleting disposable paths even
	// if an earlier RemoveAll already removed their generation markers.
	for _, entry := range transaction.Entries {
		if err := directoryExists(entry.Root); err != nil {
			return err
		}
		generation, err := readGeneration(entry.Root)
		if err != nil {
			return err
		}
		if generation != transaction.Generation {
			return errors.New("committed generation mismatch")
		}
	}
	for _, entry := range transaction.Entries {
		if err := ops.removeAll(entry.Stage); err != nil {
			return err
		}
		if err := ops.removeAll(entry.Backup); err != nil {
			return err
		}
	}
	return finalizePublication(transaction, ops)
}

func finalizePublication(transaction journal, ops fileOperations) error {
	return finalizePublicationExcept(transaction, ops, nil)
}

func finalizePublicationExcept(transaction journal, ops fileOperations, absent map[string]bool) error {
	transaction.State = "finalized"
	if err := writeJournal(journalPath(transaction.Entries[0].Root), transaction, ops); err != nil {
		return err
	}
	return removeJournalsExcept(transaction, ops, absent)
}

func removeJournals(transaction journal, ops fileOperations) error {
	return removeJournalsExcept(transaction, ops, nil)
}

func removeJournalsExcept(transaction journal, ops fileOperations, absent map[string]bool) error {
	if transaction.State != "finalized" {
		return errors.New("publication is not finalized")
	}
	if err := validateJournalMetadata(transaction, transaction.Entries[0].Root); err != nil {
		return err
	}
	// Finalization is durable before any peer becomes available to another
	// transaction. Retire only this transaction's records, coordinator last.
	for i := len(transaction.Entries) - 1; i >= 0; i-- {
		if absent[transaction.Entries[i].Root] {
			continue
		}
		path := journalPath(transaction.Entries[i].Root)
		current, err := readJournal(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		if !sameTransaction(transaction, current) {
			continue
		}
		if err := ops.remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if err := syncDirectory(filepath.Dir(path)); err != nil {
			return err
		}
	}
	return nil
}
func copyTree(ctx context.Context, source, target string, ops fileOperations) error {
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if path == source {
			return nil
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		destination := filepath.Join(target, rel)
		if entry.Type()&os.ModeSymlink != 0 {
			return errors.New("output snapshots cannot contain symlinks")
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return ops.mkdir(destination, 0755)
		}
		if !info.Mode().IsRegular() {
			return errors.New("output snapshots require regular files")
		}
		input, err := os.Open(path)
		if err != nil {
			return err
		}
		defer input.Close()
		output, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, info.Mode().Perm())
		if err != nil {
			return err
		}
		_, copyErr := io.CopyBuffer(output, contextReader{ctx: ctx, reader: input}, make([]byte, 64*1024))
		if copyErr == nil {
			copyErr = ops.sync(output)
		}
		if err := errors.Join(copyErr, output.Close()); err != nil {
			return err
		}
		return os.Chtimes(destination, info.ModTime(), info.ModTime())
	})
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (reader contextReader) Read(buffer []byte) (int, error) {
	if err := reader.ctx.Err(); err != nil {
		return 0, err
	}
	return reader.reader.Read(buffer)
}

func syncTree(ctx context.Context, root string, ops fileOperations) error {
	if err := directoryExists(root); err != nil {
		return err
	}
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return errors.New("staged output contains a symlink")
		}
		if entry.IsDir() {
			return syncDirectory(path)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return errors.New("staged output contains a special file")
		}
		file, err := os.OpenFile(path, os.O_RDWR, 0)
		if err != nil {
			return err
		}
		return errors.Join(ops.sync(file), file.Close())
	})
}
