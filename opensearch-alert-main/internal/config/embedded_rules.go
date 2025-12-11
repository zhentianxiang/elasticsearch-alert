package config

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
)

//go:embed embedded_rules
var embeddedRulesFS embed.FS

// BootstrapEmbeddedRules 将内置规则写入目标目录
// 当 overwrite=false 时，已存在的同名文件将被跳过
func BootstrapEmbeddedRules(targetDir string, overwrite bool, logger *logrus.Logger) (int, error) {
	if targetDir == "" {
		return 0, fmt.Errorf("目标目录不能为空")
	}

	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return 0, fmt.Errorf("创建规则目录失败: %w", err)
	}

	entries, err := fs.ReadDir(embeddedRulesFS, "embedded_rules")
	if err != nil {
		return 0, fmt.Errorf("读取内置规则失败: %w", err)
	}

	written := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if filepath.Ext(name) != ".yaml" {
			continue
		}

		destPath := filepath.Join(targetDir, name)
		if _, statErr := os.Stat(destPath); statErr == nil && !overwrite {
			if logger != nil {
				logger.Debugf("规则已存在，跳过: %s", destPath)
			}
			continue
		}

		data, readErr := embeddedRulesFS.ReadFile(filepath.Join("embedded_rules", name))
		if readErr != nil {
			return written, fmt.Errorf("读取内置规则 %s 失败: %w", name, readErr)
		}

		if writeErr := os.WriteFile(destPath, data, 0644); writeErr != nil {
			return written, fmt.Errorf("写入规则 %s 失败: %w", destPath, writeErr)
		}

		written++
		if logger != nil {
			logger.Infof("已生成内置规则: %s", destPath)
		}
	}

	return written, nil
}
