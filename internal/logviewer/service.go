package logviewer

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	OrderAsc  = "asc"
	OrderDesc = "desc"
)

var ErrInvalidLogFile = errors.New("invalid log file")

type Service struct {
	dir string
}

type FileInfo struct {
	Name    string     `json:"name"`
	Size    int64      `json:"size"`
	ModTime *time.Time `json:"mod_time,omitempty"`
}

type Entry struct {
	File    string         `json:"file"`
	Time    *time.Time     `json:"time,omitempty"`
	Level   string         `json:"level,omitempty"`
	Message string         `json:"message,omitempty"`
	Caller  string         `json:"caller,omitempty"`
	Fields  map[string]any `json:"fields,omitempty"`
	Raw     string         `json:"raw"`
}

type Query struct {
	File   string
	Level  string
	Text   string
	Order  string
	Limit  int
	Offset int
}

type Result struct {
	Items  []Entry    `json:"items"`
	Total  int        `json:"total"`
	Files  []FileInfo `json:"files"`
	Limit  int        `json:"limit"`
	Offset int        `json:"offset"`
	Order  string     `json:"order"`
}

func New(dir string) *Service {
	return &Service{dir: dir}
}

func AllowedFiles() []string {
	return []string{"app.log", "sync.log", "telegram.log", "error.log"}
}

func (s *Service) Files() []FileInfo {
	files := make([]FileInfo, 0, len(AllowedFiles()))
	for _, name := range AllowedFiles() {
		info := FileInfo{Name: name}
		stat, err := os.Stat(filepath.Join(s.dir, name))
		if err == nil {
			modTime := stat.ModTime().UTC()
			info.Size = stat.Size()
			info.ModTime = &modTime
		}
		files = append(files, info)
	}
	return files
}

func (s *Service) Path(name string) (string, error) {
	if !isAllowedFile(name) {
		return "", ErrInvalidLogFile
	}
	return filepath.Join(s.dir, name), nil
}

func (s *Service) List(query Query) (Result, error) {
	query = normalizeQuery(query)
	files, err := queryFiles(query.File)
	if err != nil {
		return Result{}, err
	}

	entries := make([]Entry, 0)
	for _, file := range files {
		fileEntries, err := s.readFile(file, query)
		if err != nil {
			return Result{}, err
		}
		entries = append(entries, fileEntries...)
	}

	sort.SliceStable(entries, func(i, j int) bool {
		return entryBefore(entries[i], entries[j])
	})
	if query.Order == OrderDesc {
		reverse(entries)
	}

	total := len(entries)
	start := min(query.Offset, total)
	end := min(start+query.Limit, total)
	return Result{
		Items:  entries[start:end],
		Total:  total,
		Files:  s.Files(),
		Limit:  query.Limit,
		Offset: query.Offset,
		Order:  query.Order,
	}, nil
}

func (s *Service) readFile(name string, query Query) ([]Entry, error) {
	path, err := s.Path(name)
	if err != nil {
		return nil, err
	}
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return []Entry{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("open log file %s: %w", name, err)
	}
	defer file.Close()

	var entries []Entry
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		raw := scanner.Text()
		entry := parseEntry(name, raw)
		if matches(entry, query) {
			entries = append(entries, entry)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan log file %s: %w", name, err)
	}
	return entries, nil
}

func normalizeQuery(query Query) Query {
	query.File = strings.TrimSpace(query.File)
	query.Level = strings.ToLower(strings.TrimSpace(query.Level))
	query.Text = strings.ToLower(strings.TrimSpace(query.Text))
	query.Order = strings.ToLower(strings.TrimSpace(query.Order))
	if query.Order != OrderAsc && query.Order != OrderDesc {
		query.Order = OrderDesc
	}
	if query.Limit <= 0 {
		query.Limit = 200
	}
	if query.Limit > 1000 {
		query.Limit = 1000
	}
	if query.Offset < 0 {
		query.Offset = 0
	}
	return query
}

func queryFiles(name string) ([]string, error) {
	if name == "" {
		return AllowedFiles(), nil
	}
	if !isAllowedFile(name) {
		return nil, ErrInvalidLogFile
	}
	return []string{name}, nil
}

func isAllowedFile(name string) bool {
	if name != filepath.Base(name) {
		return false
	}
	for _, allowed := range AllowedFiles() {
		if name == allowed {
			return true
		}
	}
	return false
}

func parseEntry(file string, raw string) Entry {
	entry := Entry{File: file, Raw: raw}
	var data map[string]any
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		entry.Message = raw
		return entry
	}
	entry.Fields = map[string]any{}
	for key, value := range data {
		switch key {
		case "ts":
			if text, ok := value.(string); ok {
				if t, err := parseLogTime(text); err == nil {
					utc := t.UTC()
					entry.Time = &utc
				}
			}
		case "level":
			entry.Level, _ = value.(string)
		case "msg":
			entry.Message, _ = value.(string)
		case "caller":
			entry.Caller, _ = value.(string)
		default:
			entry.Fields[key] = value
		}
	}
	if len(entry.Fields) == 0 {
		entry.Fields = nil
	}
	return entry
}

func parseLogTime(value string) (time.Time, error) {
	layouts := []string{
		time.RFC3339Nano,
		"2006-01-02T15:04:05.000Z0700",
		"2006-01-02T15:04:05Z0700",
	}
	var lastErr error
	for _, layout := range layouts {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return parsed, nil
		}
		lastErr = err
	}
	return time.Time{}, lastErr
}

func matches(entry Entry, query Query) bool {
	if query.Level != "" && strings.ToLower(entry.Level) != query.Level {
		return false
	}
	if query.Text == "" {
		return true
	}
	return strings.Contains(strings.ToLower(entry.Raw), query.Text) ||
		strings.Contains(strings.ToLower(entry.Message), query.Text) ||
		strings.Contains(strings.ToLower(entry.Caller), query.Text) ||
		strings.Contains(strings.ToLower(entry.File), query.Text)
}

func entryBefore(a, b Entry) bool {
	if a.Time != nil && b.Time != nil && !a.Time.Equal(*b.Time) {
		return a.Time.Before(*b.Time)
	}
	if a.Time != nil && b.Time == nil {
		return true
	}
	if a.Time == nil && b.Time != nil {
		return false
	}
	if a.File != b.File {
		return a.File < b.File
	}
	return a.Raw < b.Raw
}

func reverse(entries []Entry) {
	for i, j := 0, len(entries)-1; i < j; i, j = i+1, j-1 {
		entries[i], entries[j] = entries[j], entries[i]
	}
}
