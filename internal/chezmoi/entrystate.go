package chezmoi

import (
	"bytes"
	"errors"
	"io/fs"
	"log/slog"
	"runtime"

	"chezmoi.io/chezmoi/v2/internal/chezmoilog"
)

var ErrTextConvFailed = errors.New("textconv failed")

// An EntryStateType is an entry state type.
type EntryStateType string

// Entry state types.
const (
	EntryStateTypeDir     EntryStateType = "dir"
	EntryStateTypeFile    EntryStateType = "file"
	EntryStateTypeSymlink EntryStateType = "symlink"
	EntryStateTypeRemove  EntryStateType = "remove"
	EntryStateTypeScript  EntryStateType = "script"
)

// An EntryState represents the state of an entry. A nil EntryState is
// equivalent to EntryStateTypeAbsent.
type EntryState struct {
	Type           EntryStateType `json:"type"                     yaml:"type"`
	Mode           fs.FileMode    `json:"mode,omitempty"           yaml:"mode,omitempty"`
	ContentsSHA256 HexBytes       `json:"contentsSHA256,omitempty" yaml:"contentsSHA256,omitempty"` //nolint:tagliatelle
	contents       []byte
	overwrite      bool
}

// Contents returns s's contents, if available.
func (s *EntryState) Contents() []byte {
	return s.contents
}

// Equal returns true if s is equal to other.
func (s *EntryState) Equal(other *EntryState) bool {
	if s.Type != other.Type {
		return false
	}
	if runtime.GOOS != "windows" && s.Mode.Perm() != other.Mode.Perm() {
		return false
	}
	return bytes.Equal(s.ContentsSHA256, other.ContentsSHA256)
}

// Equivalent returns true if s is equivalent to other.
func (s *EntryState) Equivalent(other *EntryState) bool {
	switch {
	case s == nil:
		return other == nil || other.Type == EntryStateTypeRemove
	case other == nil:
		return s.Type == EntryStateTypeRemove
	default:
		return s.Equal(other)
	}
}

// LogValue implements log/slog.LogValuer.LogValue.
func (s *EntryState) LogValue() slog.Value {
	if s == nil {
		return slog.Value{}
	}
	attrs := []slog.Attr{
		slog.String("Type", string(s.Type)),
		slog.Int("Mode", int(s.Mode)),
		chezmoilog.Stringer("ContentsSHA256", s.ContentsSHA256),
	}
	if len(s.contents) != 0 {
		attrs = append(attrs, chezmoilog.FirstFewBytes("contents", s.contents))
	}
	if s.overwrite {
		attrs = append(attrs, slog.Bool("overwrite", s.overwrite))
	}
	return slog.GroupValue(attrs...)
}

// Overwrite returns true if s should be overwritten by default.
func (s *EntryState) Overwrite() bool {
	return s.overwrite
}

// StateChangeDetails contains fine-grained details about what changed between
// two entry states.
type StateChangeDetails struct {
	TypeChanged     bool
	ContentsChanged bool
	ModeChanged     bool
}

// DetectChanges returns fine-grained details about the changes between a and b.
func DetectChanges(a, b *EntryState) StateChangeDetails {
	switch {
	case a == nil && b == nil:
		return StateChangeDetails{}
	case a == nil:
		return StateChangeDetails{
			TypeChanged:     b.Type != EntryStateTypeRemove,
			ContentsChanged: len(b.ContentsSHA256) != 0,
			ModeChanged:     b.Mode.Perm() != 0,
		}
	case b == nil:
		return StateChangeDetails{
			TypeChanged:     a.Type != EntryStateTypeRemove,
			ContentsChanged: len(a.ContentsSHA256) != 0,
			ModeChanged:     a.Mode.Perm() != 0,
		}
	}
	details := StateChangeDetails{}
	if a.Type != b.Type {
		details.TypeChanged = true
	}
	if !bytes.Equal(a.ContentsSHA256, b.ContentsSHA256) {
		details.ContentsChanged = true
	}
	if runtime.GOOS != "windows" && a.Mode.Perm() != b.Mode.Perm() {
		details.ModeChanged = true
	}
	return details
}

// StateComparisonResult represents the result of comparing three entry states.
type StateComparisonResult struct {
	TargetMatchesActual  bool
	LastWrittenMatchesActual bool
	SilentUpdateNeeded   bool
	ApplyNeeded          bool
	TargetVsActual       StateChangeDetails
	LastWrittenVsActual  StateChangeDetails
}

// CompareStates compares targetEntryState, lastWrittenEntryState, and actualEntryState
// and returns a StateComparisonResult with flags indicating what actions are needed.
// This function provides a unified way to compare states across apply, diff, status,
// and verify commands.
func CompareStates(targetEntryState, lastWrittenEntryState, actualEntryState *EntryState) StateComparisonResult {
	targetMatchesActual := targetEntryState.Equivalent(actualEntryState)
	lastWrittenMatchesActual := lastWrittenEntryState.Equivalent(actualEntryState)

	return StateComparisonResult{
		TargetMatchesActual:     targetMatchesActual,
		LastWrittenMatchesActual: lastWrittenMatchesActual,
		SilentUpdateNeeded:      targetMatchesActual && !lastWrittenMatchesActual,
		ApplyNeeded:             !targetMatchesActual,
		TargetVsActual:          DetectChanges(targetEntryState, actualEntryState),
		LastWrittenVsActual:     DetectChanges(lastWrittenEntryState, actualEntryState),
	}
}

// TextConvResult holds the result of a textconv operation, including any error.
type TextConvResult struct {
	ConvertedContents []byte
	Converted         bool
	Err               error
}

// StateDecision represents the complete, unified decision for a single target
// entry. It is produced once per entry and consumed by apply, diff, status, and
// verify commands to ensure consistent behavior.
type StateDecision struct {
	TargetRelPath          RelPath
	TargetEntryState       *EntryState
	LastWrittenEntryState  *EntryState
	ActualEntryState       *EntryState
	Comparison             StateComparisonResult
	FromTextConvResult     TextConvResult
	ToTextConvResult       TextConvResult
	SkipApplyResult        bool
	SkipApplyErr           error

	TargetIsNil bool
	ActualIsNil bool
	TargetIsEmpty bool

	NeedApply          bool
	NeedSilentUpdate   bool
	NeedReportDrift    bool
	NeedUpdateLastWritten bool
}

// MakeStateDecision produces a single, authoritative StateDecision for a
// target entry. All commands (apply, diff, status, verify) should use this
// function to ensure consistent behavior across the codebase.
func MakeStateDecision(
	targetRelPath RelPath,
	targetEntryState, lastWrittenEntryState, actualEntryState *EntryState,
	skipApplyResult bool,
	skipApplyErr error,
	textConvFunc func(path string, data []byte) ([]byte, bool, error),
) StateDecision {
	decision := StateDecision{
		TargetRelPath:         targetRelPath,
		TargetEntryState:      targetEntryState,
		LastWrittenEntryState: lastWrittenEntryState,
		ActualEntryState:      actualEntryState,
		SkipApplyResult:       skipApplyResult,
		SkipApplyErr:          skipApplyErr,
		TargetIsNil:           targetEntryState == nil,
		ActualIsNil:           actualEntryState == nil,
	}

	if targetEntryState != nil {
		decision.TargetIsEmpty = targetEntryState.Type == EntryStateTypeRemove ||
			(targetEntryState.Type == EntryStateTypeFile && len(targetEntryState.ContentsSHA256) == 0)
	}

	decision.Comparison = CompareStates(targetEntryState, lastWrittenEntryState, actualEntryState)

	if textConvFunc != nil && actualEntryState != nil && len(actualEntryState.contents) != 0 {
		converted, convertedFlag, err := textConvFunc(targetRelPath.String(), actualEntryState.contents)
		decision.FromTextConvResult = TextConvResult{
			ConvertedContents: converted,
			Converted:         convertedFlag,
			Err:               err,
		}
	}

	if textConvFunc != nil && targetEntryState != nil && len(targetEntryState.contents) != 0 {
		converted, convertedFlag, err := textConvFunc(targetRelPath.String(), targetEntryState.contents)
		decision.ToTextConvResult = TextConvResult{
			ConvertedContents: converted,
			Converted:         convertedFlag,
			Err:               err,
		}
	}

	decision.NeedSilentUpdate = decision.Comparison.SilentUpdateNeeded
	decision.NeedApply = decision.Comparison.ApplyNeeded && !skipApplyResult
	decision.NeedReportDrift = !decision.Comparison.TargetMatchesActual ||
		decision.FromTextConvResult.Err != nil ||
		decision.ToTextConvResult.Err != nil
	decision.NeedUpdateLastWritten = (decision.NeedApply || decision.NeedSilentUpdate) &&
		!skipApplyResult

	return decision
}
