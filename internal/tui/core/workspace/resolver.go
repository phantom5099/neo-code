package workspace

import "strings"

// SelectSessionWorkdir 优先返回会话工作目录，缺失时回退到默认工作目录。
func SelectSessionWorkdir(sessionWorkdir string, defaultWorkdir string) string {
	workdir := strings.TrimSpace(sessionWorkdir)
	if workdir != "" {
		return workdir
	}
	return strings.TrimSpace(defaultWorkdir)
}
