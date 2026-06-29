# Installing Geoffrussy Globally

Geoffrussy can be installed globally so you can use it from any project directory.

## Method 1: Automated Install (Recommended)

Use the provided install script:

```bash
./install.sh
```

The script will:
1. Build the geoffrussy binary
2. Detect your OS (Linux, macOS, or Windows)
3. Install to `/usr/local/bin` (Linux/macOS) or `~/bin` (Windows)
4. Add to your PATH if needed
5. Verify the installation

### After Installation

Test it works:
```bash
cd /tmp
mkdir test-project && cd test-project
geoffrussy init
```

## Method 2: Using Make

```bash
make install-system
```

This uses the same script as Method 1.

## Method 3: Using Go

```bash
go install ./cmd/geoffrussy
```

This installs to `~/go/bin/` which should be in your PATH.

## Method 4: Manual Install

### Linux/macOS

```bash
# Build
go build -o geoffrussy ./cmd/geoffrussy

# Install
sudo cp geoffrussy /usr/local/bin/
sudo chmod +x /usr/local/bin/geoffrussy

# Verify
geoffrussy version
```

### Windows (Git Bash/MSYS)

```bash
# Build
go build -o geoffrussy.exe ./cmd/geoffrussy

# Install
cp geoffrussy.exe ~/bin/
chmod +x ~/bin/geoffrussy.exe
```

## Adding to PATH (if needed)

### Bash (~/.bashrc)
```bash
export PATH="$HOME/go/bin:$PATH"           # If using go install
# or
export PATH="/usr/local/bin:$PATH"         # If using /usr/local/bin
```

### Zsh (~/.zshrc)
```bash
export PATH="$HOME/go/bin:$PATH"
# or
export PATH="/usr/local/bin:$PATH"
```

### Fish (~/.config/fish/config.fish)
```bash
set -gx PATH $HOME/go/bin $PATH
# or
set -gx PATH /usr/local/bin $PATH
```

Reload your shell:
```bash
source ~/.bashrc   # or ~/.zshrc, etc.
```

## Verifying Installation

Check if Geoffrussy is in your PATH:
```bash
which geoffrussy
```

Run version to verify:
```bash
geoffrussy version
```

## Uninstalling

### Using Make
```bash
make uninstall
```

### Manually

```bash
# Linux/macOS
sudo rm /usr/local/bin/geoffrussy

# Windows
rm ~/bin/geoffrussy
```

## Troubleshooting

### "command not found: geoffrussy"

1. Check if it's installed:
   ```bash
   ls /usr/local/bin/geoffrussy   # Linux/macOS
   ls ~/bin/geoffrussy            # Windows
   ```

2. Check your PATH:
   ```bash
   echo $PATH
   ```

3. Add to PATH (see "Adding to PATH" section above)

4. Reload your shell:
   ```bash
   source ~/.bashrc
   ```

### Permission Denied

Use sudo for Linux/macOS:
```bash
sudo ./install.sh
# or
sudo make install-system
```

### Wrong Architecture

Geoffrussy builds for your current OS/architecture. If you need cross-platform:
```bash
make build-all
```

Then install the appropriate binary manually:
```bash
cp bin/geoffrussy-linux-amd64 /usr/local/bin/geoffrussy
```

## Installation Locations

| OS | Install Location | Notes |
|-----|-----------------|--------|
| Linux | `/usr/local/bin` | Requires sudo |
| macOS | `/usr/local/bin` | Requires sudo |
| Windows (Git Bash) | `~/bin` | No sudo needed |
| Go install | `~/go/bin` | No sudo needed |
