// Package fileset 描述多文件种子的文件列表，以及逻辑偏移到物理文件的映射。
package fileset

import (
	"fmt"
	"path/filepath"
	"strings"
)

// File 种子中的一个文件。
type File struct {
	Path   []string `json:"path"`   // 相对路径分量
	Length int64    `json:"length"`
}

// RelPath 返回用当前系统分隔符连接的相对路径。
func (f File) RelPath() string {
	return filepath.Join(f.Path...)
}

// SafeRelPath 拒绝 ".." 穿越后的相对路径。
func (f File) SafeRelPath() (string, error) {
	for _, p := range f.Path {
		if p == ".." || p == "." || p == "" || strings.ContainsAny(p, `/\`) {
			return "", fmt.Errorf("非法路径分量: %q", p)
		}
	}
	if len(f.Path) == 0 {
		return "", fmt.Errorf("空路径")
	}
	return filepath.Join(f.Path...), nil
}

// Set 一组文件及其总长度。
type Set struct {
	Name  string // 顶层目录名（多文件）或文件名（单文件）
	Files []File
}

// TotalLength 返回所有文件长度之和。
func (s *Set) TotalLength() int64 {
	var n int64
	for _, f := range s.Files {
		n += f.Length
	}
	return n
}

// Single 构造单文件集合。
func Single(name string, length int64) *Set {
	return &Set{
		Name:  name,
		Files: []File{{Path: []string{name}, Length: length}},
	}
}

// Validate 检查路径合法性与长度非负。
func (s *Set) Validate() error {
	if s.Name == "" {
		return fmt.Errorf("缺少顶层名称")
	}
	if len(s.Files) == 0 {
		return fmt.Errorf("文件列表为空")
	}
	seen := map[string]bool{}
	for i, f := range s.Files {
		if f.Length < 0 {
			return fmt.Errorf("文件 %d 长度为负", i)
		}
		rel, err := f.SafeRelPath()
		if err != nil {
			return fmt.Errorf("文件 %d: %w", i, err)
		}
		if seen[rel] {
			return fmt.Errorf("重复文件路径: %s", rel)
		}
		seen[rel] = true
	}
	return nil
}
