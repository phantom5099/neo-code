package tools

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAtomicWrite_Safety(t *testing.T) {
	// 1. 准备环境：创建一个初始文件
	tmpDir, err := os.MkdirTemp("", "neocode-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	targetFile := filepath.Join(tmpDir, "important.txt")
	originalContent := []byte("original data")
	if err := os.WriteFile(targetFile, originalContent, 0644); err != nil {
		t.Fatal(err)
	}

	// 2. 模拟一个会导致失败的写入（例如：写入空内容或模拟权限错误）
	// 注意：真正的原子性测试通常需要模拟系统调用失败，
	// 但在单元测试层面，我们可以验证：如果 Rename 没发生，原文件绝不会变。
	newContent := []byte("new corrupted data")

	// 我们手动模拟 AtomicWrite 的前半部分逻辑
	tmpFile, err := os.CreateTemp(tmpDir, "neocode-tmp-*")
	if err != nil {
		t.Fatal(err)
	}
	// 写入数据到临时文件
	_, _ = tmpFile.Write(newContent)
	_ = tmpFile.Sync()
	_ = tmpFile.Close()

	// 此时：临时文件已经写好，但我们【不调用】Rename
	// 验证：原文件内容必须依然是 "original data"
	currentContent, _ := os.ReadFile(targetFile)
	if string(currentContent) != string(originalContent) {
		t.Errorf("原子性破坏！原文件在重命名之前就被修改了。期望 %s, 实际 %s", originalContent, currentContent)
	}

	// 3. 验证正常调用 AtomicWrite 是否成功
	finalContent := []byte("final safe data")
	if err := AtomicWrite(targetFile, finalContent); err != nil {
		t.Fatalf("AtomicWrite 失败: %v", err)
	}

	// 验证：只有在 AtomicWrite 成功后，内容才会变
	updatedContent, _ := os.ReadFile(targetFile)
	if string(updatedContent) != string(finalContent) {
		t.Errorf("内容更新失败。期望 %s, 实际 %s", finalContent, updatedContent)
	}
}

func TestAtomicWrite_DirectoryCreation(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "neocode-dir-test-*")
	defer os.RemoveAll(tmpDir)

	// 测试：写入到一个不存在的深层子目录
	deepFile := filepath.Join(tmpDir, "a/b/c/test.txt")
	content := []byte("hello")

	if err := AtomicWrite(deepFile, content); err != nil {
		t.Fatalf("无法处理不存在的目录: %v", err)
	}

	if _, err := os.Stat(deepFile); os.IsNotExist(err) {
		t.Error("文件未被成功创建")
	}
}
