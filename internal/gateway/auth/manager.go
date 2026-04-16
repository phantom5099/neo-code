package auth

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	// DefaultAuthRelativePath 定义默认凭证文件相对路径。
	DefaultAuthRelativePath = ".neocode/auth.json"
	// credentialSchemaVersion 定义凭证文件结构版本号。
	credentialSchemaVersion = 1
	// tokenRandomByteLength 定义静默认证 Token 的随机字节长度。
	tokenRandomByteLength = 32
)

const (
	authDirPerm  = 0o700
	authFilePerm = 0o600
)

// Credentials 表示持久化在磁盘上的认证凭证结构。
type Credentials struct {
	Version   int       `json:"version"`
	Token     string    `json:"token"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Manager 负责加载或生成本地静默认证 Token，并提供校验能力。
type Manager struct {
	path        string
	credentials Credentials
}

// NewManager 创建并初始化认证管理器；若凭证文件不存在或无效则自动重建。
func NewManager(path string) (*Manager, error) {
	resolvedPath, err := resolveAuthPath(path)
	if err != nil {
		return nil, err
	}

	manager := &Manager{
		path: resolvedPath,
	}
	if loadErr := manager.loadOrCreate(); loadErr != nil {
		return nil, loadErr
	}
	return manager, nil
}

// Path 返回认证凭证文件路径。
func (m *Manager) Path() string {
	if m == nil {
		return ""
	}
	return m.path
}

// Token 返回当前有效 Token。
func (m *Manager) Token() string {
	if m == nil {
		return ""
	}
	return strings.TrimSpace(m.credentials.Token)
}

// ValidateToken 校验输入 Token 是否与本地凭证一致。
func (m *Manager) ValidateToken(token string) bool {
	if m == nil {
		return false
	}
	return strings.TrimSpace(token) != "" && strings.TrimSpace(token) == strings.TrimSpace(m.credentials.Token)
}

// LoadTokenFromFile 从指定路径读取静默认证 Token。
func LoadTokenFromFile(path string) (string, error) {
	resolvedPath, err := resolveAuthPath(path)
	if err != nil {
		return "", err
	}
	credentials, err := readCredentials(resolvedPath)
	if err != nil {
		return "", err
	}
	token := strings.TrimSpace(credentials.Token)
	if token == "" {
		return "", fmt.Errorf("gateway auth: token is empty in %s", resolvedPath)
	}
	return token, nil
}

// DefaultAuthPath 返回默认认证文件路径。
func DefaultAuthPath() (string, error) {
	return resolveAuthPath("")
}

// loadOrCreate 加载现有凭证，若不存在或内容无效则自动重建。
func (m *Manager) loadOrCreate() error {
	if m == nil {
		return fmt.Errorf("gateway auth: manager is nil")
	}

	if err := ensureAuthDir(filepath.Dir(m.path)); err != nil {
		return err
	}

	credentials, readErr := readCredentials(m.path)
	if readErr == nil && isValidCredentials(credentials) {
		m.credentials = credentials
		return nil
	}

	createdCredentials, createErr := buildCredentials(time.Now().UTC())
	if createErr != nil {
		return createErr
	}
	if writeErr := writeCredentials(m.path, createdCredentials); writeErr != nil {
		return writeErr
	}
	m.credentials = createdCredentials
	return nil
}

// resolveAuthPath 解析认证文件路径并清理空白。
func resolveAuthPath(path string) (string, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed != "" {
		return filepath.Clean(trimmed), nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("gateway auth: resolve user home dir: %w", err)
	}
	return filepath.Join(homeDir, DefaultAuthRelativePath), nil
}

// ensureAuthDir 确保认证目录存在并在 Unix 上收紧目录权限。
func ensureAuthDir(dir string) error {
	if err := os.MkdirAll(dir, authDirPerm); err != nil {
		return fmt.Errorf("gateway auth: create auth dir: %w", err)
	}
	if err := applyAuthDirPermission(dir); err != nil {
		return err
	}
	return nil
}

// readCredentials 读取并解析认证凭证文件。
func readCredentials(path string) (Credentials, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Credentials{}, fmt.Errorf("gateway auth: read auth file: %w", err)
	}

	var credentials Credentials
	if err := json.Unmarshal(raw, &credentials); err != nil {
		return Credentials{}, fmt.Errorf("gateway auth: decode auth file: %w", err)
	}
	return credentials, nil
}

// buildCredentials 生成新的认证凭证结构。
func buildCredentials(now time.Time) (Credentials, error) {
	token, err := generateToken()
	if err != nil {
		return Credentials{}, err
	}
	return Credentials{
		Version:   credentialSchemaVersion,
		Token:     token,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// generateToken 生成高强度随机 Token。
func generateToken() (string, error) {
	seed := make([]byte, tokenRandomByteLength)
	if _, err := rand.Read(seed); err != nil {
		return "", fmt.Errorf("gateway auth: generate token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(seed), nil
}

// writeCredentials 持久化凭证文件并在 Unix 上收紧文件权限。
func writeCredentials(path string, credentials Credentials) error {
	raw, err := json.MarshalIndent(credentials, "", "  ")
	if err != nil {
		return fmt.Errorf("gateway auth: encode credentials: %w", err)
	}
	raw = append(raw, '\n')
	if err := os.WriteFile(path, raw, authFilePerm); err != nil {
		return fmt.Errorf("gateway auth: write auth file: %w", err)
	}
	if err := applyAuthFilePermission(path); err != nil {
		return err
	}
	return nil
}

// isValidCredentials 判断凭证内容是否完整可用。
func isValidCredentials(credentials Credentials) bool {
	return credentials.Version >= credentialSchemaVersion && strings.TrimSpace(credentials.Token) != ""
}
