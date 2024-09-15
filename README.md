# vault2file

vault2file is a command-line tool that reads YAML configuration files, fetches secrets from HashiCorp Vault, and generates corresponding .env files. It's designed to simplify secret management in various environments, including Docker containers.

## Installation

### Prerequisites

- Go 1.22 or later
- Access to a HashiCorp Vault instance

### Building from source

1. Clone the repository:

   ```sh
   git clone https://github.com/yourusername/vault2file.git
   cd vault2file
   ```

2. Build the binary:

   ```sh
   go build -o vault2file
   ```

3. (Optional) Move the binary to a directory in your PATH:

   ```sh
   sudo mv vault2file /usr/local/bin/
   ```

## Usage

### Basic Usage

```sh
vault2file [flags] [input_file_or_directory]
```

If no input file or directory is specified, vault2file will process all .yml files in the current directory.

### Flags

- `-o, --output string`: Output directory for ENV files (default ".")

### Examples

1. Process a single file:

   ```sh
   vault2file -o /path/to/output/dir /path/to/input/file.yml
   ```

2. Process all .yml files in a directory:

   ```sh
   vault2file -o /path/to/output/dir /path/to/input/dir
   ```

3. Process files in the current directory:

   ```sh
   vault2file -o /path/to/output/dir
   ```

### YAML File Format

Your YAML files should follow this structure:

```yaml
secrets:
  KEY_NAME: "vault://secret/path#field"
  ANOTHER_KEY: "static_value"
```

- Keys under `secrets` will become environment variable names.
- Values starting with `vault://` will be fetched from Vault.
- Other values will be treated as static and copied directly to the .env file.

## Docker Integration

To use vault2file in a Docker environment:

1. Include the vault2file binary in your Docker image.

2. Create an entrypoint script that runs vault2file before your main application:

   ```bash
   #!/bin/bash
   set -e

   # Run vault2file
   /app/vault2file -o /secrets /secrets

   # Source all .env files
   for f in /secrets/*.env; do
       if [ -f "$f" ]; then
           export $(cat $f | xargs)
       fi
   done

   # Run the main application
   exec "$@"
   ```

3. Use this script as your Docker entrypoint.

## Security Considerations

- Ensure that your Vault token has the minimum necessary permissions.
- Use Vault's AppRole or another appropriate auth method instead of static tokens when possible.
- Be cautious about logging and debugging output that might expose secrets.
- Remember that while .env files are more secure than environment variables, they're still accessible to processes running with the same user permissions.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
