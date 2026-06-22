package chezmoi

import (
	"path"
	"strings"
)

var emptySourceRelPath SourceRelPath

// A SourceRelPath is a relative path to an entry in the source state.
type SourceRelPath struct {
	relPath RelPath
	isDir   bool
}

// NewSourceRelDirPath returns a new SourceRelPath for a directory.
func NewSourceRelDirPath(relPath string) SourceRelPath {
	return SourceRelPath{
		relPath: NewRelPath(relPath),
		isDir:   true,
	}
}

// NewSourceRelPath returns a new SourceRelPath.
func NewSourceRelPath(relPath string) SourceRelPath {
	return SourceRelPath{
		relPath: NewRelPath(relPath),
	}
}

// Dir returns p's directory.
func (p SourceRelPath) Dir() SourceRelPath {
	return SourceRelPath{
		relPath: p.relPath.Dir(),
		isDir:   true,
	}
}

// IsEmpty returns true if p is empty.
func (p SourceRelPath) IsEmpty() bool {
	return p == SourceRelPath{}
}

// Join appends sourceRelPaths to p.
func (p SourceRelPath) Join(sourceRelPaths ...SourceRelPath) SourceRelPath {
	if len(sourceRelPaths) == 0 {
		return p
	}
	relPaths := make([]RelPath, len(sourceRelPaths))
	for i, sourceRelPath := range sourceRelPaths {
		relPaths[i] = sourceRelPath.relPath
	}
	return SourceRelPath{
		relPath: p.relPath.Join(relPaths...),
		isDir:   sourceRelPaths[len(sourceRelPaths)-1].isDir,
	}
}

// MarshalText implements encoding.TextMarshaler.MarshalText.
func (p SourceRelPath) MarshalText() ([]byte, error) {
	return []byte(p.relPath.String()), nil
}

// RelPath returns p as a relative path.
func (p SourceRelPath) RelPath() RelPath {
	return p.relPath
}

// Split returns the p's file and directory.
func (p SourceRelPath) Split() (dirSourceRelPath, fileSourceRelPath SourceRelPath) {
	dir, file := p.relPath.Split()
	return NewSourceRelDirPath(dir.String()), NewSourceRelPath(file.String())
}

func (p SourceRelPath) String() string {
	return p.relPath.String()
}

// TargetRelPath returns the relative path of p's target.
func (p SourceRelPath) TargetRelPath(encryptedSuffix string) (RelPath, error) {
	sourceNames := strings.Split(p.relPath.String(), "/")
	relPathStrs := make([]string, 0, len(sourceNames))
	if p.isDir {
		for i, sourceName := range sourceNames {
			if i == 0 && sourceName == "" {
				// Special case: we allow the first element to be empty.
				relPathStrs = append(relPathStrs, "")
				continue
			}
			dirAttr, err := parseDirAttr(sourceName)
			if err != nil {
				return RelPath{}, err
			}
			relPathStrs = append(relPathStrs, dirAttr.TargetName)
		}
	} else {
		for _, sourceName := range sourceNames[:len(sourceNames)-1] {
			dirAttr, err := parseDirAttr(sourceName)
			if err != nil {
				return RelPath{}, err
			}
			relPathStrs = append(relPathStrs, dirAttr.TargetName)
		}
		fileAttr, err := parseFileAttr(sourceNames[len(sourceNames)-1], encryptedSuffix)
		if err != nil {
			return RelPath{}, err
		}
		relPathStrs = append(relPathStrs, fileAttr.TargetName)
	}
	return NewRelPath(path.Join(relPathStrs...)), nil
}

// NewSourceRelPathFromAbsPath returns a new SourceRelPath from an absolute
// path within sourceDirAbsPath, checking that it is actually within the source
// directory and does not escape via parent directory traversal.
func NewSourceRelPathFromAbsPath(
	system System,
	sourceDirAbsPath AbsPath,
	sourceAbsPath AbsPath,
) (SourceRelPath, error) {
	sourceRelPathStr, err := sourceAbsPath.TrimDirPrefix(sourceDirAbsPath)
	if err != nil {
		return SourceRelPath{}, err
	}
	if strings.HasPrefix(sourceRelPathStr.String(), "..") {
		return SourceRelPath{}, &NotInAbsDirError{
			pathAbsPath: sourceAbsPath,
			dirAbsPath:  sourceDirAbsPath,
		}
	}
	fileInfo, err := system.Lstat(sourceAbsPath)
	if err != nil {
		return SourceRelPath{}, err
	}
	if fileInfo.IsDir() {
		return NewSourceRelDirPath(sourceRelPathStr.String()), nil
	}
	return NewSourceRelPath(sourceRelPathStr.String()), nil
}

// SourceAbsPathToTargetRelPath converts a source absolute path to a target
// relative path, using the given source directory, encryption suffix, and
// system for file type detection.
func SourceAbsPathToTargetRelPath(
	system System,
	sourceDirAbsPath AbsPath,
	sourceAbsPath AbsPath,
	encryptedSuffix string,
) (RelPath, error) {
	sourceRelPath, err := NewSourceRelPathFromAbsPath(system, sourceDirAbsPath, sourceAbsPath)
	if err != nil {
		return RelPath{}, err
	}
	return sourceRelPath.TargetRelPath(encryptedSuffix)
}
