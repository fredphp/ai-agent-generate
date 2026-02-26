// Package filesystem provides file system operations.
package filesystem

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Errors
var (
	ErrFileNotFound      = fmt.Errorf("file not found")
	ErrDirectoryNotFound = fmt.Errorf("directory not found")
	ErrInvalidPath       = fmt.Errorf("invalid path")
	ErrPathOutsideRoot   = fmt.Errorf("path outside root directory")
)

// FileInfo represents file information.
type FileInfo struct {
	Path         string
	AbsolutePath string
	Name         string
	Extension    string
	Size         int64
	IsDir        bool
	IsFile       bool
	ModTime      time.Time
	Checksum     string
}

// FileContent represents file content.
type FileContent struct {
	Info    FileInfo
	Content string
	Lines   int
}

// Config holds manager configuration.
type Config struct {
	RootDir       string
	BackupDir     string
	BackupEnabled bool
	MaxFileSize   int64
	MaxBackups    int
}

// DefaultConfig returns default config.
func DefaultConfig() Config {
	return Config{
		BackupDir:     ".ai-backup",
		BackupEnabled: true,
		MaxFileSize:   10 * 1024 * 1024,
		MaxBackups:    10,
	}
}

// DefaultIgnorePatterns contains patterns to ignore.
var DefaultIgnorePatterns = []string{
	"node_modules", "vendor", ".git", ".svn", ".hg",
	".idea", ".vscode", "dist", "build", "out", "target",
	".cache", "*.log", ".DS_Store", ".ai-backup",
}

// Manager manages file operations.
type Manager struct {
	config         Config
	ignorePatterns []*regexp.Regexp
}

// NewManager creates a new file manager.
func NewManager(config Config) (*Manager, error) {
	if config.RootDir == "" {
		return nil, ErrInvalidPath
	}

	absRoot, err := filepath.Abs(config.RootDir)
	if err != nil {
		return nil, err
	}

	if _, err := os.Stat(absRoot); os.IsNotExist(err) {
		return nil, ErrDirectoryNotFound
	}

	config.RootDir = absRoot
	if config.BackupDir == "" {
		config.BackupDir = ".ai-backup"
	}

	m := &Manager{config: config}

	m.ignorePatterns = make([]*regexp.Regexp, 0)
	for _, pattern := range DefaultIgnorePatterns {
		regex, _ := patternToRegex(pattern)
		if regex != nil {
			m.ignorePatterns = append(m.ignorePatterns, regex)
		}
	}

	return m, nil
}

// ReadFile reads a file.
func (m *Manager) ReadFile(path string) (*FileContent, error) {
	absPath, err := m.resolvePath(path)
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrFileNotFound
		}
		return nil, err
	}

	if info.IsDir() {
		return nil, fmt.Errorf("path is a directory")
	}

	content, err := os.ReadFile(absPath)
	if err != nil {
		return nil, err
	}

	relPath, _ := filepath.Rel(m.config.RootDir, absPath)
	checksum := sha256Hash(content)

	return &FileContent{
		Info: FileInfo{
			Path:         relPath,
			AbsolutePath: absPath,
			Name:         info.Name(),
			Extension:    filepath.Ext(info.Name()),
			Size:         info.Size(),
			IsDir:        false,
			IsFile:       true,
			ModTime:      info.ModTime(),
			Checksum:     checksum,
		},
		Content: string(content),
		Lines:   strings.Count(string(content), "\n") + 1,
	}, nil
}

// WriteFile writes a file with backup.
func (m *Manager) WriteFile(path, content string, createDirs bool) (*string, error) {
	absPath, err := m.resolvePath(path)
	if err != nil {
		return nil, err
	}

	var backupPath *string

	if _, err := os.Stat(absPath); err == nil && m.config.BackupEnabled {
		bp := m.createBackup(absPath)
		backupPath = &bp
	}

	if createDirs {
		os.MkdirAll(filepath.Dir(absPath), 0755)
	}

	if err := os.WriteFile(absPath, []byte(content), 0644); err != nil {
		return nil, err
	}

	return backupPath, nil
}

// FileExists checks if file exists.
func (m *Manager) FileExists(path string) bool {
	absPath, err := m.resolvePath(path)
	if err != nil {
		return false
	}
	_, err = os.Stat(absPath)
	return err == nil
}

// ScanDirectory scans a directory.
func (m *Manager) ScanDirectory(path string, recursive bool) ([]FileInfo, error) {
	absPath, err := m.resolvePath(path)
	if err != nil {
		return nil, err
	}

	var files []FileInfo

	err = filepath.WalkDir(absPath, func(walkPath string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		relPath, _ := filepath.Rel(m.config.RootDir, walkPath)

		if m.shouldIgnore(relPath, d.IsDir()) {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}

		if d.IsDir() && !recursive && walkPath != absPath {
			return fs.SkipDir
		}

		info, _ := d.Info()
		files = append(files, FileInfo{
			Path:         relPath,
			AbsolutePath: walkPath,
			Name:         d.Name(),
			Extension:    filepath.Ext(d.Name()),
			Size:         info.Size(),
			IsDir:        d.IsDir(),
			IsFile:       !d.IsDir(),
			ModTime:      info.ModTime(),
		})
		return nil
	})

	return files, err
}

// ListFiles lists all files.
func (m *Manager) ListFiles(path string, recursive bool, extensions []string) ([]FileInfo, error) {
	files, err := m.ScanDirectory(path, recursive)
	if err != nil {
		return nil, err
	}

	extMap := make(map[string]bool)
	for _, ext := range extensions {
		extMap[strings.ToLower(ext)] = true
	}

	var result []FileInfo
	for _, f := range files {
		if !f.IsFile {
			continue
		}
		if len(extMap) > 0 && !extMap[strings.ToLower(f.Extension)] {
			continue
		}
		result = append(result, f)
	}

	return result, nil
}

// CopyFile copies a file.
func (m *Manager) CopyFile(src, dst string) error {
	content, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, content, 0644)
}

// RestoreBackup restores from backup.
func (m *Manager) RestoreBackup(backupPath string) error {
	content, err := os.ReadFile(backupPath)
	if err != nil {
		return err
	}

	// Extract original path from backup path
	relPath := strings.TrimPrefix(backupPath, m.config.RootDir)
	relPath = strings.TrimPrefix(relPath, string(filepath.Separator))
	relPath = strings.TrimSuffix(relPath, ".bak")
	relPath = strings.TrimSuffix(relPath, ".ai-backup")

	absPath := filepath.Join(m.config.RootDir, relPath)
	return os.WriteFile(absPath, content, 0644)
}

// GetRoot returns root directory.
func (m *Manager) GetRoot() string {
	return m.config.RootDir
}

// Helper methods
func (m *Manager) resolvePath(path string) (string, error) {
	path = filepath.Clean(path)

	if filepath.IsAbs(path) {
		rel, err := filepath.Rel(m.config.RootDir, path)
		if err != nil || strings.HasPrefix(rel, "..") {
			return "", ErrPathOutsideRoot
		}
		return path, nil
	}

	absPath := filepath.Join(m.config.RootDir, path)
	absPath = filepath.Clean(absPath)

	rel, err := filepath.Rel(m.config.RootDir, absPath)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", ErrPathOutsideRoot
	}

	return absPath, nil
}

func (m *Manager) shouldIgnore(path string, isDir bool) bool {
	path = filepath.ToSlash(path)
	for _, pattern := range m.ignorePatterns {
		if pattern.MatchString(path) || pattern.MatchString(filepath.Base(path)) {
			return true
		}
	}
	return false
}

func (m *Manager) createBackup(filePath string) string {
	content, _ := os.ReadFile(filePath)
	if content == nil {
		return ""
	}

	backupDir := filepath.Join(m.config.RootDir, m.config.BackupDir)
	os.MkdirAll(backupDir, 0755)

	relPath, _ := filepath.Rel(m.config.RootDir, filePath)
	backupName := fmt.Sprintf("%s.%s.bak", filepath.Base(filePath), time.Now().Format("20060102-150405"))
	backupPath := filepath.Join(backupDir, filepath.Dir(relPath), backupName)

	os.MkdirAll(filepath.Dir(backupPath), 0755)
	os.WriteFile(backupPath, content, 0644)

	// Cleanup old backups
	m.cleanupOldBackups(filePath)

	return backupPath
}

func (m *Manager) cleanupOldBackups(filePath string) {
	if m.config.MaxBackups <= 0 {
		return
	}

	backupDir := filepath.Join(m.config.RootDir, m.config.BackupDir)
	relPath, _ := filepath.Rel(m.config.RootDir, filePath)
	subDir := filepath.Join(backupDir, filepath.Dir(relPath))

	entries, err := os.ReadDir(subDir)
	if err != nil {
		return
	}

	baseName := filepath.Base(filePath)
	var backups []string
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, baseName) && strings.HasSuffix(name, ".bak") {
			backups = append(backups, filepath.Join(subDir, name))
		}
	}

	sort.Strings(backups)

	for i := 0; i < len(backups)-m.config.MaxBackups; i++ {
		os.Remove(backups[i])
	}
}

// Utility functions
func patternToRegex(pattern string) (*regexp.Regexp, error) {
	regex := "^"
	for _, ch := range pattern {
		switch ch {
		case '*':
			regex += ".*"
		case '?':
			regex += "."
		case '.', '^', '$', '+', '{', '}', '[', ']', '|', '(', ')':
			regex += "\\" + string(ch)
		default:
			regex += string(ch)
		}
	}
	regex += "$"
	return regexp.Compile(regex)
}

func sha256Hash(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}
