package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"

	vault "github.com/hashicorp/vault/api"
)

type Config struct {
	Secrets map[string]string `yaml:"secrets"`
}

var (
	input     string
	outputDir string
	vaultAddr string
)

var rootCmd = &cobra.Command{
	Use:   "vault2file [flags] [input_file_or_directory]",
	Short: "Transfer secrets from Vault to files",
	Long:  `vault2file reads YAML files, fetches secrets from Vault, and generates corresponding ENV files.`,
	Args:  cobra.MaximumNArgs(1),
	Run:   run,
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&outputDir, "output", "o", ".", "Output directory for ENV files")
	rootCmd.PersistentFlags().StringVarP(&vaultAddr, "vault", "v", os.Getenv("VAULT_ADDR"), "Vault server address")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func run(cmd *cobra.Command, args []string) {
	if len(args) > 0 {
		input = args[0]
	} else {
		input = "."
	}

	// Initialize Vault client
	client, err := vault.NewClient(&vault.Config{Address: vaultAddr})
	if err != nil {
		log.Fatalf("Failed to create Vault client: %v", err)
	}

	// Check if input is a file or directory
	fileInfo, err := os.Stat(input)
	if err != nil {
		log.Fatalf("Error accessing input: %v", err)
	}

	if fileInfo.IsDir() {
		// Process directory
		err = filepath.Walk(input, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() && (strings.HasSuffix(info.Name(), ".yml") || strings.HasSuffix(info.Name(), ".yaml")) {
				err := processFile(client, path)
				if err != nil {
					log.Printf("Error processing %s: %v", path, err)
				}
			}
			return nil
		})
		if err != nil {
			log.Fatalf("Error walking through directory: %v", err)
		}
	} else {
		// Process single file
		if !strings.HasSuffix(input, ".yml") {
			log.Fatalf("Input file must have .yml extension")
		}
		err := processFile(client, input)
		if err != nil {
			log.Fatalf("Error processing %s: %v", input, err)
		}
	}
}

func processFile(client *vault.Client, inputPath string) error {
	// Read YAML file
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("error reading file: %v", err)
	}

	// Parse YAML
	var config Config
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return fmt.Errorf("error parsing YAML: %v", err)
	}

	// Create output file
	baseName := filepath.Base(inputPath)
	outputName := strings.TrimSuffix(baseName, filepath.Ext(baseName)) + ".env"
	outputPath := filepath.Join(outputDir, outputName)
	envFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %v", err)
	}
	defer envFile.Close()

	// Process each secret
	for key, value := range config.Secrets {
		if strings.HasPrefix(value, "vault://") {
			// Write non-Vault values directly, but quoted
			quotedValue := strconv.Quote(value)
			fmt.Fprintf(envFile, "%s=%s\n", key, quotedValue)
			continue
		}

		// Extract path and key from Vault URL
		parts := strings.SplitN(strings.TrimPrefix(value, "vault://"), "#", 2)
		if len(parts) != 2 {
			log.Printf("Invalid Vault URL for %s: %s", key, value)
			continue
		}
		path, envKey := parts[0], parts[1]

		// Fetch secret from Vault
		secret, err := client.Logical().Read(path)
		if err != nil {
			log.Printf("Failed to read secret from Vault for %s: %v", key, err)
			continue
		}

		// Write to ENV file
		if secretValue, ok := secret.Data[envKey]; ok {
			quotedValue := strconv.Quote(fmt.Sprintf("%v", secretValue))
			fmt.Fprintf(envFile, "%s=%s\n", key, quotedValue)
			continue
		}

		log.Printf("Failed to find %s in Vault secret", envKey)
	}

	log.Printf("Created %s successfully", outputPath)
	return nil
}
