package tools

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// stubMetadata 快速构建测试用 metadata map。
func stubMetadata(keyValue ...string) map[string]string {
	m := make(map[string]string, len(keyValue)/2)
	for i := 0; i+1 < len(keyValue); i += 2 {
		m[keyValue[i]] = keyValue[i+1]
	}
	return m
}

func assertContains(t *testing.T, got, expected string) {
	t.Helper()
	if !strings.Contains(got, expected) {
		t.Fatalf("expected %q in summary, got %q", expected, got)
	}
}

func assertMaxRuneCount(t *testing.T, got string, max int) {
	t.Helper()
	if utf8.RuneCountInString(got) > max {
		t.Fatalf("summary exceeds %d runes: %d", max, utf8.RuneCountInString(got))
	}
}

func TestBashSummarizer(t *testing.T) {
	t.Parallel()

	t.Run("normal_output", func(t *testing.T) {
		content := "line1\nline2\nline3\nline4\nline5\nline6\nline7\nline8"
		meta := stubMetadata("workdir", "/home/user/project")
		got := bashSummarizer(content, meta, false)
		assertContains(t, got, "[exit=0]")
		assertContains(t, got, "workdir=/home/user/project")
		assertContains(t, got, "line8")
		assertMaxRuneCount(t, got, 200)
	})

	t.Run("error_output", func(t *testing.T) {
		content := "error: command not found"
		meta := stubMetadata("workdir", "/tmp")
		got := bashSummarizer(content, meta, true)
		assertContains(t, got, "[exit=non-zero]")
	})

	t.Run("short_output", func(t *testing.T) {
		content := "ok"
		got := bashSummarizer(content, nil, false)
		assertContains(t, got, "ok")
	})

	t.Run("empty_content", func(t *testing.T) {
		got := bashSummarizer("", nil, false)
		assertContains(t, got, "[exit=0]")
	})
}

func TestReadFileSummarizer(t *testing.T) {
	t.Parallel()

	t.Run("normal_file", func(t *testing.T) {
		content := "package main\n\nfunc main() {\n\tfmt.Println(\"hello\")\n}\n"
		meta := stubMetadata("path", "/home/user/main.go")
		got := readFileSummarizer(content, meta, false)
		assertContains(t, got, "/home/user/main.go")
		assertContains(t, got, "lines=")
		assertContains(t, got, "first=package main")
		assertMaxRuneCount(t, got, 200)
	})

	t.Run("missing_path", func(t *testing.T) {
		got := readFileSummarizer("content", nil, false)
		if got != "" {
			t.Fatalf("expected empty string for missing path, got %q", got)
		}
	})
}

func TestWriteFileSummarizer(t *testing.T) {
	t.Parallel()

	t.Run("normal", func(t *testing.T) {
		meta := stubMetadata("path", "/home/user/test.go", "bytes", "1024")
		got := writeFileSummarizer("", meta, false)
		assertContains(t, got, "/home/user/test.go")
		assertContains(t, got, "1024 bytes")
	})

	t.Run("missing_path", func(t *testing.T) {
		got := writeFileSummarizer("", stubMetadata("bytes", "100"), false)
		if got != "" {
			t.Fatalf("expected empty for missing path, got %q", got)
		}
	})
}

func TestEditSummarizer(t *testing.T) {
	t.Parallel()

	t.Run("with_relative_path", func(t *testing.T) {
		meta := stubMetadata("relative_path", "src/main.go", "path", "/abs/src/main.go", "search_length", "50", "replacement_length", "60")
		got := editSummarizer("", meta, false)
		assertContains(t, got, "src/main.go")
		assertContains(t, got, "search=50")
	})

	t.Run("fallback_to_abs_path", func(t *testing.T) {
		meta := stubMetadata("path", "/abs/src/main.go", "search_length", "10", "replacement_length", "20")
		got := editSummarizer("", meta, false)
		assertContains(t, got, "/abs/src/main.go")
	})

	t.Run("missing_path", func(t *testing.T) {
		got := editSummarizer("", stubMetadata("search_length", "10"), false)
		if got != "" {
			t.Fatalf("expected empty for missing path, got %q", got)
		}
	})
}

func TestGrepSummarizer(t *testing.T) {
	t.Parallel()

	t.Run("with_matches", func(t *testing.T) {
		content := "src/a.go:10:match1\nsrc/b.go:20:match2\nsrc/c.go:30:match3\nsrc/d.go:40:match4"
		meta := stubMetadata("root", "/home/user", "matched_files", "4", "matched_lines", "4")
		got := grepSummarizer(content, meta, false)
		assertContains(t, got, "root=/home/user")
		assertContains(t, got, "files=4")
		assertMaxRuneCount(t, got, 200)
	})

	t.Run("empty_content", func(t *testing.T) {
		meta := stubMetadata("root", "/home", "matched_files", "0", "matched_lines", "0")
		got := grepSummarizer("", meta, false)
		assertContains(t, got, "files=0")
	})
}

func TestGlobSummarizer(t *testing.T) {
	t.Parallel()

	t.Run("with_files", func(t *testing.T) {
		content := "src/a.go\nsrc/b.go\nsrc/c.go\nsrc/d.go"
		meta := stubMetadata("count", "4")
		got := globSummarizer(content, meta, false)
		assertContains(t, got, "4 files")
		assertMaxRuneCount(t, got, 200)
	})

	t.Run("no_matches", func(t *testing.T) {
		meta := stubMetadata("count", "0")
		got := globSummarizer("", meta, false)
		assertContains(t, got, "0 files")
	})
}

func TestWebfetchSummarizer(t *testing.T) {
	t.Parallel()

	t.Run("with_truncated_flag", func(t *testing.T) {
		meta := stubMetadata("truncated", "true")
		got := webfetchSummarizer("", meta, false)
		assertContains(t, got, "truncated=true")
	})

	t.Run("minimal", func(t *testing.T) {
		got := webfetchSummarizer("", nil, false)
		assertContains(t, got, "[summary] webfetch")
	})
}

func TestRegisterBuiltinSummarizers(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	RegisterBuiltinSummarizers(registry)

	toolNames := []string{
		ToolNameBash, ToolNameFilesystemReadFile, ToolNameFilesystemWriteFile,
		ToolNameFilesystemEdit, ToolNameFilesystemGrep, ToolNameFilesystemGlob,
		ToolNameWebFetch,
	}
	for _, name := range toolNames {
		if registry.MicroCompactSummarizer(name) == nil {
			t.Errorf("expected summarizer for %q to be registered", name)
		}
	}

	// 不在注册列表中的工具应返回 nil
	if registry.MicroCompactSummarizer("unknown_tool") != nil {
		t.Fatal("expected nil for unknown tool")
	}
}

func TestRegisterSummarizer(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()

	// 注册
	called := false
	registry.RegisterSummarizer("test_tool", func(content string, metadata map[string]string, isError bool) string {
		called = true
		return "summary"
	})

	s := registry.MicroCompactSummarizer("test_tool")
	if s == nil {
		t.Fatal("expected summarizer to be registered")
	}
	result := s("content", nil, false)
	if !called {
		t.Fatal("expected summarizer to be called")
	}
	if result != "summary" {
		t.Fatalf("expected 'summary', got %q", result)
	}

	// 移除
	registry.RegisterSummarizer("test_tool", nil)
	if registry.MicroCompactSummarizer("test_tool") != nil {
		t.Fatal("expected nil after removal")
	}
}

func TestTruncateRunes(t *testing.T) {
	t.Parallel()

	t.Run("short", func(t *testing.T) {
		got := truncateRunes("hello", 10)
		if got != "hello" {
			t.Fatalf("expected unchanged, got %q", got)
		}
	})

	t.Run("exact", func(t *testing.T) {
		got := truncateRunes("hello", 5)
		if got != "hello" {
			t.Fatalf("expected unchanged, got %q", got)
		}
	})

	t.Run("truncated", func(t *testing.T) {
		got := truncateRunes("hello world", 5)
		if got != "hello..." {
			t.Fatalf("expected 'hello...', got %q", got)
		}
	})

	t.Run("chinese", func(t *testing.T) {
		got := truncateRunes("你好世界测试", 3)
		if got != "你好世..." {
			t.Fatalf("expected '你好世...', got %q", got)
		}
	})
}
