package openai

import (
	"errors"
	"io"
	"strings"
	"testing"

	"neo-code/internal/provider"
)

// --- boundedSSEReader 单元测试 ---

func TestBoundedSSEReader_ReadLine_Normal(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
		isEOF bool
	}{
		{
			name:  "single line with newline",
			input: "data: hello\n",
			want:  "data: hello",
			isEOF: false,
		},
		{
			name:  "line with CRLF",
			input: "data: world\r\n",
			want:  "data: world",
			isEOF: false,
		},
		{
			name:  "empty line",
			input: "\n",
			want:  "",
			isEOF: false,
		},
		{
			name:  "SSE comment line",
			input: ": heartbeat\n",
			want:  ": heartbeat",
			isEOF: false,
		},
		{
			name:  "EOF without trailing newline (io.EOF)",
			input: "data: partial",
			want:  "data: partial",
			isEOF: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := newBoundedSSEReader(strings.NewReader(tt.input))
			got, err := r.ReadLine()
			if got != tt.want {
				t.Fatalf("ReadLine() = %q, want %q", got, tt.want)
			}
			if tt.isEOF && !errors.Is(err, io.EOF) {
				t.Fatalf("expected io.EOF, got %v", err)
			}
			if !tt.isEOF && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestBoundedSSEReader_ReadLine_MultipleLines(t *testing.T) {
	t.Parallel()

	r := newBoundedSSEReader(strings.NewReader("line1\nline2\n\nline4\n"))

	line1, err := r.ReadLine()
	if err != nil || line1 != "line1" {
		t.Fatalf("first line: got %q, err = %v", line1, err)
	}

	line2, err := r.ReadLine()
	if err != nil || line2 != "line2" {
		t.Fatalf("second line: got %q, err = %v", line2, err)
	}

	// 空行
	empty, err := r.ReadLine()
	if err != nil || empty != "" {
		t.Fatalf("empty line: got %q, err = %v", empty, err)
	}

	line4, err := r.ReadLine()
	if err != nil || line4 != "line4" {
		t.Fatalf("fourth line: got %q, err = %v", line4, err)
	}

	// EOF
	_, err = r.ReadLine()
	if !errors.Is(err, io.EOF) {
		t.Fatalf("expected EOF after all lines, got %v", err)
	}
}

func TestBoundedSSEReader_L1_LineTooLong(t *testing.T) {
	t.Parallel()

	longLine := strings.Repeat("x", maxSSELineSize+1) + "\n"
	r := newBoundedSSEReader(strings.NewReader(longLine))

	_, err := r.ReadLine()
	if err == nil {
		t.Fatal("expected ErrLineTooLong for oversized line")
	}
	if !errors.Is(err, provider.ErrLineTooLong) {
		t.Fatalf("expected ErrLineTooLong, got %v", err)
	}
}

func TestBoundedSSEReader_L1_BoundaryExactLimit(t *testing.T) {
	t.Parallel()

	// 恰好等于上限的行应该正常通过（不含 \n）
	exactLine := strings.Repeat("a", maxSSELineSize) + "\n"
	r := newBoundedSSEReader(strings.NewReader(exactLine))

	got, err := r.ReadLine()
	if err != nil {
		t.Fatalf("unexpected error at exact limit: %v", err)
	}
	if len(got) != maxSSELineSize {
		t.Fatalf("expected line length %d, got %d", maxSSELineSize, len(got))
	}
}

func TestBoundedSSEReader_L3_StreamTooLarge(t *testing.T) {
	t.Parallel()

	// 构造输入：多行小内容但总量超过 maxStreamTotalSize
	var sb strings.Builder
	for i := 0; i < 100; i++ {
		sb.WriteString("data: chunk\n")
	}
	// 填充到超过总量上限
	remaining := maxStreamTotalSize - int64(sb.Len()) + 1
	sb.WriteString(strings.Repeat("x", int(remaining)) + "\n")

	r := newBoundedSSEReader(strings.NewReader(sb.String()))

	// 前面的行应能正常读取
	for i := 0; i < 100; i++ {
		_, err := r.ReadLine()
		if err != nil {
			t.Fatalf("unexpected error on normal line %d: %v", i, err)
		}
	}

	// 超限的行应返回 ErrStreamTooLarge
	_, err := r.ReadLine()
	if err == nil {
		t.Fatal("expected ErrStreamTooLarge")
	}
	if !errors.Is(err, provider.ErrStreamTooLarge) {
		t.Fatalf("expected ErrStreamTooLarge, got %v", err)
	}
}

func TestTrimLineEnding(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  string
	}{
		{"hello\n", "hello"},
		{"hello\r\n", "hello"},
		{"hello\r\n\n", "hello"}, // 连续换行符全部去除
		{"hello", "hello"},
		{"\n", ""},
		{"\r\n", ""},
		{"\r", ""}, // 孤立 \r 也去除
		{"", ""},
	}

	for _, tt := range tests {
		got := trimLineEnding(tt.input)
		if got != tt.want {
			t.Fatalf("trimLineEnding(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
