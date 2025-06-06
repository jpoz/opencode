# Diff Test Tool

A simple tool to test the `internal/diff` package with bubbletea for interactive viewing.

## Usage

### Interactive Mode (with a terminal)
```bash
# Use sample diff
./difftest --interactive

# Load diff from file
./difftest --interactive mydiff.patch

# Pipe diff from git
git diff | ./difftest --interactive
```

### Static Mode (for CI/testing)
```bash
# Use sample diff
./difftest

# Load diff from file  
./difftest mydiff.patch

# Pipe diff from git
git diff | ./difftest
```

## Controls (Interactive Mode)

- `q` or `Ctrl+C` - Quit
- `↑/k` - Scroll up one line
- `↓/j` - Scroll down one line  
- `Page Up` - Scroll up one page
- `Page Down` - Scroll down one page
- `Home` - Go to top
- `End` - Go to bottom

## Examples

Generate a test diff with git:
```bash
# Make some changes to a file
echo 'Hello, World!' > test.txt
git add test.txt
git commit -m "Initial commit"
echo 'Hello, OpenCode!' > test.txt
git diff | ./difftest --interactive
```

Test the diff package with a large diff:
```bash
git log --pretty=format:"%h %s" -n 100 > large.txt
git add large.txt
git commit -m "Add large file"
# Edit the file to make substantial changes
git diff | ./difftest --interactive
```