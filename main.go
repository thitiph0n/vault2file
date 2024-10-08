package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/hashicorp/vault-client-go"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
)

var Version = "dev" // this will be overwritten by the build process

type Config struct {
	Secrets map[string]string `yaml:"secrets"`
}

var (
	input     string
	outputDir string
)

var rootCmd = &cobra.Command{
	Use:   "vault2file [flags] [input_file_or_directory]",
	Short: "Transfer secrets from Vault to files",
	Long:  `vault2file reads YAML files, fetches secrets from Vault, and generates corresponding ENV files.`,
	Args:  cobra.MaximumNArgs(1),
	PreRunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			cmd.Help()
			os.Exit(0)
		}
		return nil
	},
	RunE:    run,
	Version: Version,
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&outputDir, "output", "o", ".", "Output directory for ENV files")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func run(cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		input = args[0]
	} else {
		input = "."
	}

	client, err := vault.New(
		vault.WithEnvironment(),
	)
	if err != nil {
		log.Fatalf("Error creating Vault client: %v", err)
	}

	// check if input is a file or directory
	fileInfo, err := os.Stat(input)
	if err != nil {
		log.Fatalf("Error accessing input: %v", err)
		return err
	}

	if fileInfo.IsDir() {
		// process directory
		if err = filepath.Walk(input, func(path string, info os.FileInfo, err error) error {
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
		}); err != nil {
			log.Fatalf("Error walking through directory: %v", err)
			return err
		}

		return nil
	}

	// process single file
	if !strings.HasSuffix(input, ".yml") || strings.HasSuffix(input, ".yaml") {
		return fmt.Errorf("input file must have .yml extension")
	}

	if err := processFile(client, input); err != nil {
		log.Fatalf("Error processing %s: %v", input, err)
		return err
	}

	return nil
}

func processFile(client *vault.Client, inputPath string) error {
	ctx := context.Background()

	// read YAML file
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("error reading file: %v", err)
	}

	// parse YAML
	var config Config
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return fmt.Errorf("error parsing YAML: %v", err)
	}

	// create output file
	baseName := filepath.Base(inputPath)
	outputName := strings.TrimSuffix(baseName, filepath.Ext(baseName)) + ".env"
	outputPath := filepath.Join(outputDir, outputName)
	envFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %v", err)
	}
	defer envFile.Close()

	// process each secret
	for key, value := range config.Secrets {
		if !strings.HasPrefix(value, "vault://") {
			// write non-Vault values directly, but quoted
			quotedValue := strconv.Quote(value)
			fmt.Fprintf(envFile, "%s=%s\n", key, quotedValue)
			continue
		}

		// extract path and key from Vault URL
		parts := strings.SplitN(strings.TrimPrefix(value, "vault://"), "#", 2)
		if len(parts) != 2 {
			log.Printf("Invalid Vault URL for %s: %s", key, value)
			continue
		}
		path, envKey := parts[0], parts[1]

		// extract mount path from path
		parts = strings.SplitN(path, "/", 2)
		if len(parts) != 2 {
			log.Printf("Invalid path for %s: %s", key, path)
			continue
		}

		mountPath, path := parts[0], parts[1]

		// fetch secret from Vault
		secret, err := client.Secrets.KvV2Read(ctx, path, vault.WithMountPath(mountPath))
		if err != nil {
			log.Printf("Failed to read secret from Vault for %s: %v", key, err)
			continue
		}

		// write to ENV file
		if secretValue, ok := secret.Data.Data[envKey]; ok {
			quotedValue := strconv.Quote(fmt.Sprintf("%v", secretValue))
			fmt.Fprintf(envFile, "%s=%s\n", key, quotedValue)
			continue
		}

		log.Printf("Failed to find %s in Vault secret", envKey)
	}

	log.Printf("Created %s successfully", outputPath)
	return nil
}
