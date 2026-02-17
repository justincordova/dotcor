#!/bin/bash

# DotCor Manual Test Script
# Usage: ./scripts/test-manual.sh [command]
# Commands: init, add, edit, status, clean, full-test, copy-dotfiles

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
DOTCOR_BIN="$PROJECT_ROOT/bin/dotcor"
TEST_HOME="$PROJECT_ROOT/.manual-test"

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

success() {
    echo -e "${GREEN}[OK]${NC} $1"
}

warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

error() {
    echo -e "\033[0;31m[ERROR]${NC} $1"
}

# Ensure binary exists
check_binary() {
    if [ ! -f "$DOTCOR_BIN" ]; then
        info "Building DotCor binary..."
        cd "$PROJECT_ROOT"
        go build -o bin/dotcor ./cmd/dotcor
        success "Binary built at $DOTCOR_BIN"
    fi
}

# Setup test environment
setup_test_env() {
    info "Setting up test environment at $TEST_HOME"
    rm -rf "$TEST_HOME"
    mkdir -p "$TEST_HOME"
    export HOME="$TEST_HOME"
}

# Setup test environment with dotcor initialized (for most tests)
setup_test_env_with_init() {
    info "Setting up test environment at $TEST_HOME"
    rm -rf "$TEST_HOME"
    mkdir -p "$TEST_HOME/.dotcor"
    mkdir -p "$TEST_HOME/.config/nvim"
    export HOME="$TEST_HOME"
}

# Clean test environment
clean_test_env() {
    info "Cleaning test environment at $TEST_HOME"
    rm -rf "$TEST_HOME"
    success "Test environment cleaned"
}

# Test init
test_init() {
    setup_test_env
    info "Testing: dotcor init"
    "$DOTCOR_BIN" init
    success "Init completed"
    ls -la "$TEST_HOME/.dotcor/"
}

# Test add
test_add() {
    setup_test_env
    info "Testing: dotcor add"

    # Create test files
    echo "export TEST_VAR='hello'" > "$TEST_HOME/.testrc"
    mkdir -p "$TEST_HOME/.config/nvim"
    echo "vim config" > "$TEST_HOME/.config/nvim/init.lua"

    # Add files
    "$DOTCOR_BIN" add ~/.testrc
    "$DOTCOR_BIN" add ~/.config/nvim/init.lua

    # Verify
    "$DOTCOR_BIN" status
    success "Add completed"
}

# Test edit and sync
test_edit_sync() {
    setup_test_env
    info "Testing: edit and sync workflow"

    # Init and add
    "$DOTCOR_BIN" init
    echo "export TEST_VAR='hello'" > "$TEST_HOME/.testrc"
    "$DOTCOR_BIN" add ~/.testrc

    info "Editing file (simulating symlink magic)..."
    echo "export NEW_VAR='world'" >> "$TEST_HOME/.testrc"

    info "Checking status..."
    "$DOTCOR_BIN" status

    info "Syncing changes..."
    "$DOTCOR_BIN" sync

    success "Edit & sync completed"
}

# Test status
test_status() {
    setup_test_env
    info "Testing: dotcor status"

    # Setup files
    "$DOTCOR_BIN" init
    echo "test content" > "$TEST_HOME/.testrc"
    "$DOTCOR_BIN" add ~/.testrc

    info "Full status:"
    "$DOTCOR_BIN" status

    info "Quick status:"
    "$DOTCOR_BIN" status --quick

    success "Status test completed"
}

# Full end-to-end test
test_full() {
    info "Running full E2E test..."
    setup_test_env

    # Init
    info "Step 1: Initialize"
    "$DOTCOR_BIN" init

    # Add files
    info "Step 2: Add files"
    echo "zsh config" > "$TEST_HOME/.zshrc"
    echo "vim config" > "$TEST_HOME/.config/nvim/init.lua"
    mkdir -p "$TEST_HOME/.config/nvim"
    "$DOTCOR_BIN" add ~/.zshrc
    "$DOTCOR_BIN" add ~/.config/nvim/init.lua

    # Check status
    info "Step 3: Check status"
    "$DOTCOR_BIN" status

    # Edit file
    info "Step 4: Edit file (symlink magic)"
    echo "new line" >> "$TEST_HOME/.zshrc"
    "$DOTCOR_BIN" status

    # Sync
    info "Step 5: Sync changes"
    "$DOTCOR_BIN" sync

    # History
    info "Step 6: View history"
    "$DOTCOR_BIN" history ~/.zshrc

    # Remove
    info "Step 7: Remove file"
    "$DOTCOR_BIN" remove ~/.zshrc < /dev/null || true

    # Doctor
    info "Step 8: Run doctor"
    "$DOTCOR_BIN" doctor

    success "Full E2E test completed!"
    info "Test environment: $TEST_HOME"
    info "Clean up with: $0 clean"
}

# Create sample dotfiles for testing (replaces old copy-dotfiles)
test_create_samples() {
    info "Creating sample dotfiles in $TEST_HOME"
    setup_test_env

    # Create sample dotfiles for testing
    info "Creating sample dotfiles..."
    
    # Shell configs
    cat > "$TEST_HOME/.zshrc" << 'EOF'
# Test Zsh Configuration
export PATH="$HOME/bin:$PATH"
alias ll='ls -la'
alias ..='cd ..'

# Prompt
PROMPT='%n@%m %~ $ '
EOF

    cat > "$TEST_HOME/.bashrc" << 'EOF'
# Test Bash Configuration
export PATH="$HOME/bin:$PATH"
alias ll='ls -la'
alias ..='cd ..'

# Prompt
PS1='\u@\h \w $ '
EOF

    # Git config
    cat > "$TEST_HOME/.gitconfig" << 'EOF'
[user]
    name = Test User
    email = test@example.com
[core]
    editor = vim
[init]
    defaultBranch = main
EOF

    cat > "$TEST_HOME/.gitignore_global" << 'EOF'
# Global Git Ignore
.DS_Store
*.log
*.tmp
.env
node_modules/
EOF

    # Tmux config
    cat > "$TEST_HOME/.tmux.conf" << 'EOF'
# Test Tmux Configuration
set -g prefix C-a
unbind C-b
bind C-a send-prefix

# Enable mouse
set -g mouse on

# Status bar
set -g status-style bg=blue,fg=white
EOF

    # Neovim config
    mkdir -p "$TEST_HOME/.config/nvim"
    cat > "$TEST_HOME/.config/nvim/init.lua" << 'EOF'
-- Test Neovim Configuration
vim.opt.number = true
vim.opt.relativenumber = true
vim.opt.tabstop = 2
vim.opt.shiftwidth = 2
vim.opt.expandtab = true

-- Test keymap
vim.keymap.set('n', '<leader>e', ':Explore<CR>')
EOF

    cat > "$TEST_HOME/.config/nvim/test.lua" << 'EOF'
-- Additional test file
print("Hello from Neovim!")
EOF

    # Add .local with some test data
    mkdir -p "$TEST_HOME/.local/share/dotcor"
    echo "Test local data file" > "$TEST_HOME/.local/share/test-data.txt"

    # Create test file for adopt testing (independent, not in dotcor repo)
    mkdir -p "$TEST_HOME/external_files"
    cat > "$TEST_HOME/external_files/test_dotfile" << 'EOF'
# Test configuration file for adopt command
# This file should be imported via adopt symlink

export ADOPT_TEST_VAR="adopted_value"
alias adopt-test="echo 'This file was adopted!'"
EOF
    # Create symlink for adopt testing (points to external file)
    ln -s "external_files/test_dotfile" "$TEST_HOME/.adopt_test_link"

    success "Sample dotfiles created"
    info "Created: .zshrc, .bashrc, .gitconfig, .gitignore_global, .tmux.conf"
    info "Created: .config/nvim/ (init.lua, test.lua)"
    info "Created: .local/share/ (test-data.txt)"
    info "Created: external_files/test_dotfile + symlink ~/.adopt_test_link (for adopt testing)"

    export HOME="$TEST_HOME"
    export PATH="$PROJECT_ROOT/bin:$PATH"

    info "Entering interactive test mode"
    info "Your test HOME is: $HOME"
    echo ""
    success "Ready! Try commands like:"
    echo "  dotcor init"
    echo "  dotcor status"
    echo "  dotcor add ~/.zshrc"
    echo "  dotcor add ~/.config/nvim/*.lua"
    echo ""
    info "Exit when done"
    info "Clean up with: $0 clean"
    echo ""

    # Spawn a shell with test environment, starting in TEST_HOME
    cd "$TEST_HOME" || exit 1
    bash
}

# Interactive test mode
test_interactive() {
    check_binary
    # Clear test directory and create sample files
    info "Setting up test environment at $TEST_HOME"
    rm -rf "$TEST_HOME"
    mkdir -p "$TEST_HOME"

    # Create sample dotfiles for testing
    info "Creating sample dotfiles..."
    
    # Shell configs
    cat > "$TEST_HOME/.zshrc" << 'EOF'
# Test Zsh Configuration
export PATH="$HOME/bin:$PATH"
alias ll='ls -la'
alias ..='cd ..'

# Prompt
PROMPT='%n@%m %~ $ '
EOF

    cat > "$TEST_HOME/.bashrc" << 'EOF'
# Test Bash Configuration
export PATH="$HOME/bin:$PATH"
alias ll='ls -la'
alias ..='cd ..'

# Prompt
PS1='\u@\h \w $ '
EOF

    # Git config
    cat > "$TEST_HOME/.gitconfig" << 'EOF'
[user]
    name = Test User
    email = test@example.com
[core]
    editor = vim
[init]
    defaultBranch = main
EOF

    cat > "$TEST_HOME/.gitignore_global" << 'EOF'
# Global Git Ignore
.DS_Store
*.log
*.tmp
.env
node_modules/
EOF

    # Tmux config
    cat > "$TEST_HOME/.tmux.conf" << 'EOF'
# Test Tmux Configuration
set -g prefix C-a
unbind C-b
bind C-a send-prefix

# Enable mouse
set -g mouse on

# Status bar
set -g status-style bg=blue,fg=white
EOF

    # Neovim config
    mkdir -p "$TEST_HOME/.config/nvim"
    cat > "$TEST_HOME/.config/nvim/init.lua" << 'EOF'
-- Test Neovim Configuration
vim.opt.number = true
vim.opt.relativenumber = true
vim.opt.tabstop = 2
vim.opt.shiftwidth = 2
vim.opt.expandtab = true

-- Test keymap
vim.keymap.set('n', '<leader>e', ':Explore<CR>')
EOF

    cat > "$TEST_HOME/.config/nvim/test.lua" << 'EOF'
-- Additional test file
print("Hello from Neovim!")
EOF

    # Add .local with some test data
    mkdir -p "$TEST_HOME/.local/share/dotcor"
    echo "Test local data file" > "$TEST_HOME/.local/share/test-data.txt"

    # Create test file for adopt testing (independent, not in dotcor repo)
    mkdir -p "$TEST_HOME/external_files"
    cat > "$TEST_HOME/external_files/test_dotfile" << 'EOF'
# Test configuration file for adopt command
# This file should be imported via adopt symlink

export ADOPT_TEST_VAR="adopted_value"
alias adopt-test="echo 'This file was adopted!'"
EOF
    # Create symlink for adopt testing (points to external file)
    ln -s "external_files/test_dotfile" "$TEST_HOME/.adopt_test_link"

    success "Sample dotfiles created"
    info "Created: .zshrc, .bashrc, .gitconfig, .gitignore_global, .tmux.conf"
    info "Created: .config/nvim/ (init.lua, test.lua)"
    info "Created: .local/share/ (test-data.txt)"
    info "Created: external_files/test_dotfile + symlink ~/.adopt_test_link (for adopt testing)"

    export HOME="$TEST_HOME"
    export PATH="$PROJECT_ROOT/bin:$PATH"

    info "Entering interactive test mode"
    info "Your test HOME is: $HOME"
    info "Binary is at: $DOTCOR_BIN"
    info "PATH includes: $PROJECT_ROOT/bin"
    echo ""
    success "Ready! Try commands like:"
    echo "  dotcor init"
    echo "  dotcor status"
    echo "  dotcor add ~/.zshrc"
    echo "  dotcor add ~/.config/nvim/*.lua"
    echo "  dotcor --version"
    echo ""
    info "Exit when done (test files preserved in $TEST_HOME)"
    info "Clean up with: $0 clean"
    echo ""

    # Spawn a shell with test environment, starting in TEST_HOME
    cd "$TEST_HOME" || exit 1
    bash
}

# Show help
show_help() {
    cat << EOF
DotCor Manual Test Script

Usage: $0 <command>

Commands:
  init           Test dotcor init
  add            Test dotcor add
  edit-sync      Test edit & sync workflow
  status         Test dotcor status
  full           Run full end-to-end test
  interactive    Enter interactive test mode (creates sample dotfiles!)
  create-samples Create sample dotfiles and enter shell
  clean          Clean test environment
  help           Show this help

Examples:
  $0 interactive    # Test interactively (creates sample files)
  $0 create-samples # Create samples and enter shell
  $0 full          # Run all tests automatically
  $0 clean         # Clean up test files

Notes:
  - Binary location: $DOTCOR_BIN
  - Test directory: $TEST_HOME
  - Builds binary if not present
  - interactive mode creates sample dotfiles (.zshrc, .bashrc, .gitconfig, etc.)
EOF
}

# Main
check_binary

case "${1:-help}" in
    init)
        test_init
        ;;
    add)
        test_add
        ;;
    edit-sync)
        test_edit_sync
        ;;
    status)
        test_status
        ;;
    full)
        test_full
        ;;
    interactive)
        test_interactive
        ;;
    create-samples)
        test_create_samples
        ;;
    clean)
        clean_test_env
        ;;
    help|--help|-h)
        show_help
        ;;
    *)
        echo "Unknown command: $1"
        show_help
        exit 1
        ;;
esac
