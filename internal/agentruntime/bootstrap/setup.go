package bootstrap

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"neo-code/internal/agentruntime/interaction"
	"neo-code/internal/config"
)

type setupDecision int

const (
	setupRetry setupDecision = iota
	setupContinue
	setupExit
)

var (
	resolveWorkspaceRoot = interaction.ResolveWorkspaceRoot
	setWorkspaceRoot     = interaction.SetWorkspaceRoot
	initializeSecurity   = interaction.InitializeSecurity
	ensureConfigFile     = config.EnsureConfigFile
	validateChatAPIKey   = interaction.ValidateChatAPIKey
	writeAppConfig       = config.WriteAppConfig
)

func PrepareWorkspace(workspaceFlag string) (string, error) {
	workspaceRoot, err := resolveWorkspaceRoot(workspaceFlag)
	if err != nil {
		return "", fmt.Errorf("resolve workspace root: %w", err)
	}
	if err := setWorkspaceRoot(workspaceRoot); err != nil {
		return "", fmt.Errorf("set workspace root: %w", err)
	}
	if err := initializeSecurity(filepath.Join(workspaceRoot, "internal", "config", "security")); err != nil {
		return "", fmt.Errorf("initialize security policy: %w", err)
	}
	return workspaceRoot, nil
}

func EnsureAPIKeyInteractive(ctx context.Context, scanner *bufio.Scanner, configPath string) (bool, error) {
	cfg, created, err := ensureConfigFile(configPath)
	if err != nil {
		return false, err
	}
	if created {
		fmt.Printf("Created %s\n", configPath)
	}

	for {
		if cfg.RuntimeAPIKey() == "" {
			envName := cfg.APIKeyEnvVarName()
			fmt.Printf("Environment variable %s is not set. Use /apikey <env_name>, /provider <name>, or /switch <model> to change the configuration, or set the variable and then run /retry.\n", envName)
			fmt.Printf("Windows example: setx %s \"your-api-key\"\n", envName)
			result, handleErr := handleSetupDecision(scanner, cfg, false, configPath)
			if handleErr != nil {
				return false, handleErr
			}
			if result == setupExit {
				return false, nil
			}
			continue
		}

		if err := validateChatAPIKey(ctx, cfg); err == nil {
			if saveErr := writeAppConfig(configPath, cfg); saveErr != nil {
				return false, saveErr
			}
			config.GlobalAppConfig = cfg
			fmt.Println("API key validation passed.")
			return true, nil
		} else if errors.Is(err, interaction.ErrInvalidAPIKey) {
			fmt.Printf("The API key in environment variable %s is invalid: %v\n", cfg.APIKeyEnvVarName(), err)
			result, handleErr := handleSetupDecision(scanner, cfg, false, configPath)
			if handleErr != nil {
				return false, handleErr
			}
			if result == setupExit {
				return false, nil
			}
			continue
		} else if errors.Is(err, interaction.ErrAPIKeyValidationSoft) {
			fmt.Printf("Could not verify the API key in environment variable %s: %v\n", cfg.APIKeyEnvVarName(), err)
			result, handleErr := handleSetupDecision(scanner, cfg, true, configPath)
			if handleErr != nil {
				return false, handleErr
			}
			if result == setupExit {
				return false, nil
			}
			if result == setupContinue {
				config.GlobalAppConfig = cfg
				return true, nil
			}
			continue
		} else {
			fmt.Printf("Model validation failed: %v\n", err)
			result, handleErr := handleSetupDecision(scanner, cfg, false, configPath)
			if handleErr != nil {
				return false, handleErr
			}
			if result == setupExit {
				return false, nil
			}
			if result == setupContinue {
				config.GlobalAppConfig = cfg
				return true, nil
			}
		}
	}
}

func handleSetupDecision(scanner *bufio.Scanner, cfg *config.AppConfiguration, allowContinue bool, configPath string) (setupDecision, error) {
	for {
		prompt := "Choose /retry, /apikey <env_name>, /provider <name>, /switch <model>, or /exit > "
		if allowContinue {
			prompt = "Choose /retry, /continue, /apikey <env_name>, /provider <name>, /switch <model>, or /exit > "
		}
		decision, ok, inputErr := readInteractiveLine(scanner, prompt)
		if inputErr != nil {
			return setupExit, inputErr
		}
		if !ok {
			return setupExit, nil
		}

		fields := strings.Fields(strings.TrimSpace(decision))
		if len(fields) == 0 {
			continue
		}

		switch strings.ToLower(fields[0]) {
		case "/retry":
			return setupRetry, nil
		case "/apikey":
			if len(fields) < 2 {
				fmt.Println("Usage: /apikey <env_name>")
				continue
			}
			applyAPIKeyEnvName(cfg, fields[1])
			fmt.Printf("Switched the API key environment variable name to: %s\n", cfg.APIKeyEnvVarName())
			return setupRetry, nil
		case "/continue":
			if !allowContinue {
				fmt.Println("/continue is only available when the API key cannot be verified due to a network or service issue.")
				continue
			}
			if saveErr := writeAppConfig(configPath, cfg); saveErr != nil {
				return setupExit, saveErr
			}
			fmt.Println("Continuing with the current API key and model.")
			return setupContinue, nil
		case "/provider":
			if len(fields) < 2 {
				fmt.Println("Usage: /provider <name>")
				printSupportedProviders()
				continue
			}
			providerName, ok := interaction.NormalizeProviderName(fields[1])
			if !ok {
				fmt.Printf("Unsupported provider %q\n", fields[1])
				printSupportedProviders()
				continue
			}
			if err := cfg.SetSelectedProvider(providerName); err != nil {
				fmt.Printf("Failed to switch provider: %v\n", err)
				continue
			}
			fmt.Printf("Switched provider to: %s\n", providerName)
			fmt.Printf("Reset the current model to the default: %s\n", cfg.CurrentModelName())
			return setupRetry, nil
		case "/switch":
			if len(fields) < 2 {
				fmt.Println("Usage: /switch <model>")
				continue
			}
			target := strings.Join(fields[1:], " ")
			cfg.SetCurrentModel(target)
			fmt.Printf("Switched model to: %s\n", target)
			return setupRetry, nil
		case "/exit":
			return setupExit, nil
		default:
			if allowContinue {
				fmt.Println("Enter /retry, /continue, /apikey <env_name>, /provider <name>, /switch <model>, or /exit.")
			} else {
				fmt.Println("Enter /retry, /apikey <env_name>, /provider <name>, /switch <model>, or /exit.")
			}
		}
	}
}

func applyAPIKeyEnvName(cfg *config.AppConfiguration, envName string) {
	if cfg == nil {
		return
	}
	cfg.SetAPIKeyEnvVarName(strings.TrimSpace(envName))
}

func readInteractiveLine(scanner *bufio.Scanner, prompt string) (string, bool, error) {
	for {
		fmt.Print(prompt)
		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				return "", false, err
			}
			return "", false, nil
		}
		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			fmt.Println("Input cannot be empty.")
			continue
		}
		if input == "/exit" {
			return "", false, nil
		}
		return input, true, nil
	}
}

func printSupportedProviders() {
	fmt.Println("Supported providers:")
	for _, name := range interaction.SupportedProviders() {
		fmt.Printf("  %s\n", name)
	}
}
