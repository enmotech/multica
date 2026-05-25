package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/multica-ai/multica/server/internal/skillfrontmatter"
)

const (
	skillMaxFileSize   int64 = 1 << 20 // 1 MB
	skillMaxBundleSize int64 = 8 << 20 // 8 MB
	skillMaxFileCount        = 128
	skillMaxScanDepth        = 4
)

type scannedSkill struct {
	DirPath string
	Name    string
}

func slugifySkillName(name string) string {
	s := strings.ToLower(name)
	re := regexp.MustCompile(`[^a-z0-9]+`)
	s = re.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if s == "" {
		s = "unnamed-skill"
	}
	return s
}

func scanSkills(root string) ([]scannedSkill, error) {
	if _, err := os.Stat(filepath.Join(root, "SKILL.md")); err == nil {
		content, err := os.ReadFile(filepath.Join(root, "SKILL.md"))
		if err != nil {
			return nil, fmt.Errorf("read SKILL.md in %s: %w", root, err)
		}
		name, _ := skillfrontmatter.Parse(string(content))
		if name == "" {
			name = filepath.Base(root)
		}
		return []scannedSkill{{DirPath: root, Name: name}}, nil
	}

	var skills []scannedSkill
	visited := make(map[string]bool)
	scanDirForSkills(root, root, 0, visited, &skills)
	return skills, nil
}

func scanDirForSkills(root, current string, depth int, visited map[string]bool, skills *[]scannedSkill) {
	if depth > skillMaxScanDepth {
		return
	}
	resolved, err := filepath.EvalSymlinks(current)
	if err != nil {
		return
	}
	if visited[resolved] {
		return
	}
	visited[resolved] = true

	entries, err := os.ReadDir(current)
	if err != nil {
		return
	}

	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		path := filepath.Join(current, name)
		info, statErr := os.Stat(path)
		if statErr != nil || !info.IsDir() {
			continue
		}

		skillMdPath := filepath.Join(path, "SKILL.md")
		if _, err := os.Stat(skillMdPath); err == nil {
			content, err := os.ReadFile(skillMdPath)
			if err != nil {
				continue
			}
			skillName, _ := skillfrontmatter.Parse(string(content))
			if skillName == "" {
				skillName = filepath.Base(path)
			}
			*skills = append(*skills, scannedSkill{DirPath: path, Name: skillName})
			continue
		}

		scanDirForSkills(root, path, depth+1, visited, skills)
	}
}

type skillFileData struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type skillLoadResult struct {
	Name        string
	Description string
	Content     string
	Files       []skillFileData
}

func loadSkillFromDir(dir string) (*skillLoadResult, error) {
	skillMdPath := filepath.Join(dir, "SKILL.md")
	raw, err := os.ReadFile(skillMdPath)
	if err != nil {
		return nil, fmt.Errorf("read SKILL.md: %w", err)
	}

	name, description, body := skillfrontmatter.ParseBody(string(raw))
	if name == "" {
		return nil, fmt.Errorf("skill name is required in SKILL.md frontmatter (%s)", dir)
	}

	files, err := collectSkillFiles(dir)
	if err != nil {
		return nil, err
	}

	return &skillLoadResult{
		Name:        name,
		Description: description,
		Content:     strings.TrimSpace(body),
		Files:       files,
	}, nil
}

func collectSkillFiles(skillDir string) ([]skillFileData, error) {
	walkRoot := skillDir
	if resolved, err := filepath.EvalSymlinks(skillDir); err == nil {
		walkRoot = resolved
	}

	var files []skillFileData
	var totalSize int64

	err := filepath.WalkDir(walkRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if path == walkRoot {
			return nil
		}
		if entry.IsDir() {
			if strings.HasPrefix(entry.Name(), ".") {
				return fs.SkipDir
			}
			return nil
		}
		if strings.HasPrefix(entry.Name(), ".") {
			return nil
		}
		if strings.EqualFold(entry.Name(), "SKILL.md") {
			return nil
		}

		rel, err := filepath.Rel(walkRoot, path)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(filepath.Clean(rel))
		if rel == "." || filepath.IsAbs(rel) || strings.HasPrefix(rel, "..") {
			return nil
		}

		info, err := entry.Info()
		if err != nil || info.Size() > skillMaxFileSize {
			return fmt.Errorf("file %s exceeds %d byte limit", rel, skillMaxFileSize)
		}
		if len(files) >= skillMaxFileCount {
			return fmt.Errorf("skill exceeds %d file limit", skillMaxFileCount)
		}
		totalSize += info.Size()
		if totalSize > skillMaxBundleSize {
			return fmt.Errorf("skill exceeds %d byte total limit", skillMaxBundleSize)
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		files = append(files, skillFileData{Path: rel, Content: string(content)})
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].Path < files[j].Path
	})
	return files, nil
}

func writeSkillToDisk(content string, files []skillFileData, dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create skill directory: %w", err)
	}

	skillMdPath := filepath.Join(dir, "SKILL.md")
	if err := os.WriteFile(skillMdPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write SKILL.md: %w", err)
	}

	for _, f := range files {
		filePath := filepath.Join(dir, filepath.FromSlash(f.Path))
		fileDir := filepath.Dir(filePath)
		if err := os.MkdirAll(fileDir, 0o755); err != nil {
			return fmt.Errorf("create file directory %s: %w", fileDir, err)
		}
		if err := os.WriteFile(filePath, []byte(f.Content), 0o644); err != nil {
			return fmt.Errorf("write file %s: %w", f.Path, err)
		}
	}

	return nil
}
