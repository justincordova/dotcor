.PHONY: test test-cover build clean lint fmt vet run help test-manual test-clean test-interactive

# Default target
help:
	@echo "DotCor Development Commands:"
	@echo ""
	@echo "  make test          Run all tests"
	@echo "  make test-cover    Run tests with coverage report (opens in browser)"
	@echo "  make test-verbose  Run tests with verbose output"
	@echo "  make test-manual   Run full manual test in .manual-test/"
	@echo "  make test-interactive Enter interactive test mode (auto-copies ~/dotfiles)"
	@echo "  make test-copy     Copy ~/dotfiles to test environment (no shell)"
	@echo "  make test-clean    Clean .manual-test/ directory"
	@echo "  make build         Build all packages"
	@echo "  make run           Build and run dotcor"
	@echo "  make clean         Clean build artifacts"
	@echo "  make lint          Run linter (golangci-lint)"
	@echo "  make fmt           Format all Go code"
	@echo "  make vet           Run go vet"
	@echo "  make deps          Download dependencies"
	@echo ""

# Run all tests
test:
	go test ./...

# Run tests with coverage report
test-cover:
	go test ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out

# Run tests with verbose output
test-verbose:
	go test ./... -v

# Build all packages
build:
	go build ./...

# Build and run
run:
	go run cmd/dotcor/main.go

# Clean build artifacts
clean:
	go clean
	rm -f coverage.out

# Run linter (requires golangci-lint)
lint:
	golangci-lint run ./...

# Format all Go code
fmt:
	go fmt ./...

# Run go vet
vet:
	go vet ./...

# Download dependencies
deps:
	go mod download
	go mod tidy

# Build the binary
binary:
	go build -C bin/ ../cmd/dotcor

# Install to GOPATH/bin
install:
	go install ./cmd/dotcor

# Manual testing
test-manual:
	@./scripts/test-manual.sh full

test-interactive:
	@./scripts/test-manual.sh interactive

test-copy:
	@./scripts/test-manual.sh copy-dotfiles

test-clean:
	@./scripts/test-manual.sh clean

DOTCOR_DIR ?= /tmp/dotcor-test
DOTCOR_HOME ?= /tmp/dotcor-home

sandbox-setup:
	@mkdir -p $(DOTCOR_HOME)
	@mkdir -p $(DOTCOR_DIR)/git $(DOTCOR_DIR)/nvim $(DOTCOR_DIR)/starship $(DOTCOR_DIR)/tmux $(DOTCOR_DIR)/zsh
	@mkdir -p $(DOTCOR_DIR)/alacritty $(DOTCOR_DIR)/bat $(DOTCOR_DIR)/eza $(DOTCOR_DIR)/fzf $(DOTCOR_DIR)/htop
	@mkdir -p $(DOTCOR_DIR)/lazygit $(DOTCOR_DIR)/ripgrep $(DOTCOR_DIR)/ssh $(DOTCOR_DIR)/vim $(DOTCOR_DIR)/zoxide
	@# --- unmanaged files in DOTCOR_HOME for testing the add wizard ---
	@test -f $(DOTCOR_HOME)/.bashrc || printf "export PATH=$$HOME/.local/bin:$$PATH\nalias ll='ls -la'\nalias gs='git status'\n" > $(DOTCOR_HOME)/.bashrc
	@test -f $(DOTCOR_HOME)/.profile || printf "if [ -f $$HOME/.bashrc ]; then\n  . $$HOME/.bashrc\nfi\n" > $(DOTCOR_HOME)/.profile
	@test -f $(DOTCOR_HOME)/.inputrc || printf '"\e[A": history-search-backward\n"\e[B": history-search-forward\nset completion-ignore-case on\n' > $(DOTCOR_HOME)/.inputrc
	@test -f $(DOTCOR_HOME)/.editorconfig || printf 'root = true\n\n[*]\nindent_style = tab\nindent_size = 4\nend_of_line = lf\ncharset = utf-8\ntrim_trailing_whitespace = true\n' > $(DOTCOR_HOME)/.editorconfig
	@mkdir -p $(DOTCOR_HOME)/.config/tmux
	@test -f $(DOTCOR_HOME)/.config/tmux/tmux.conf || printf "set -g prefix C-a\nbind | split-window -h\nbind - split-window -v\n" > $(DOTCOR_HOME)/.config/tmux/tmux.conf
	@mkdir -p $(DOTCOR_HOME)/.config/starship
	@test -f $(DOTCOR_HOME)/.config/starship.toml || printf "[character]\nsuccess_symbol = '[➜](bold green)'\n" > $(DOTCOR_HOME)/.config/starship.toml
	@# --- git (3 files) ---
	@test -f $(DOTCOR_DIR)/git/.gitconfig || printf "[user]\n\tname = Test User\n\temail = test@example.com\n[core]\n\teditor = nvim\n[pull]\n\trebase = true\n" > $(DOTCOR_DIR)/git/.gitconfig
	@test -f $(DOTCOR_DIR)/git/.gitignore_global || printf ".DS_Store\n*.swp\n*.swo\n*~\n.env\n" > $(DOTCOR_DIR)/git/.gitignore_global
	@test -f $(DOTCOR_DIR)/git/.gitcommit || printf "refactor: simplify config loader\n\nCo-authored-by: Test User <test@example.com>\n" > $(DOTCOR_DIR)/git/.gitcommit
	@# --- nvim (deep tree ~20 files) ---
	@test -f $(DOTCOR_DIR)/nvim/init.lua || printf 'require("config.lazy")\nrequire("config.options")\nrequire("config.keymaps")\nrequire("config.autocmds")\n' > $(DOTCOR_DIR)/nvim/init.lua
	@mkdir -p $(DOTCOR_DIR)/nvim/lua/config
	@test -f $(DOTCOR_DIR)/nvim/lua/config/options.lua || printf "vim.opt.number = true\nvim.opt.relativenumber = true\nvim.opt.tabstop = 4\nvim.opt.shiftwidth = 4\nvim.opt.expandtab = true\nvim.opt.signcolumn = 'yes'\n" > $(DOTCOR_DIR)/nvim/lua/config/options.lua
	@test -f $(DOTCOR_DIR)/nvim/lua/config/keymaps.lua || printf 'vim.g.mapleader = " "\nvim.keymap.set("n", "<leader>ff", "<cmd>Telescope find_files<cr>")\nvim.keymap.set("n", "<leader>fg", "<cmd>Telescope live_grep<cr>")\nvim.keymap.set("n", "<leader>fb", "<cmd>Telescope buffers<cr>")\n' > $(DOTCOR_DIR)/nvim/lua/config/keymaps.lua
	@test -f $(DOTCOR_DIR)/nvim/lua/config/autocmds.lua || printf 'vim.api.nvim_create_autocmd("TextYankPost", {\n  callback = function() vim.highlight.on_yank() end,\n})\n' > $(DOTCOR_DIR)/nvim/lua/config/autocmds.lua
	@test -f $(DOTCOR_DIR)/nvim/lua/config/lazy.lua || printf 'local lazypath = vim.fn.stdpath("data") .. "/lazy/lazy.nvim"\nvim.opt.rtp:prepend(lazypath)\nrequire("lazy").setup("plugins")\n' > $(DOTCOR_DIR)/nvim/lua/config/lazy.lua
	@test -f $(DOTCOR_DIR)/nvim/lua/config/utils.lua || printf 'local M = {}\nM.log = function(msg) vim.notify(msg, vim.log.levels.INFO) end\nreturn M\n' > $(DOTCOR_DIR)/nvim/lua/config/utils.lua
	@mkdir -p $(DOTCOR_DIR)/nvim/lua/plugins
	@test -f $(DOTCOR_DIR)/nvim/lua/plugins/telescope.lua || printf 'return {\n  "nvim-telescope/telescope.nvim",\n  dependencies = { "nvim-lua/plenary.nvim" },\n  config = function() require("telescope").setup() end,\n}\n' > $(DOTCOR_DIR)/nvim/lua/plugins/telescope.lua
	@test -f $(DOTCOR_DIR)/nvim/lua/plugins/lualine.lua || printf 'return {\n  "nvim-lualine/lualine.nvim",\n  config = function() require("lualine").setup({ options = { theme = "catppuccin" } }) end,\n}\n' > $(DOTCOR_DIR)/nvim/lua/plugins/lualine.lua
	@test -f $(DOTCOR_DIR)/nvim/lua/plugins/treesitter.lua || printf 'return {\n  "nvim-treesitter/nvim-treesitter",\n  build = ":TSUpdate",\n  config = function() require("nvim-treesitter.configs").setup({ ensure_installed = "all" }) end,\n}\n' > $(DOTCOR_DIR)/nvim/lua/plugins/treesitter.lua
	@test -f $(DOTCOR_DIR)/nvim/lua/plugins/neo-tree.lua || printf 'return {\n  "nvim-neo-tree/neo-tree.nvim",\n  dependencies = { "nvim-lua/plenary.nvim", "nvim-tree/nvim-web-devicons" },\n  config = function() vim.keymap.set("n", "<leader>e", "<cmd>Neotree toggle<cr>") end,\n}\n' > $(DOTCOR_DIR)/nvim/lua/plugins/neo-tree.lua
	@test -f $(DOTCOR_DIR)/nvim/lua/plugins/formatting.lua || printf 'return {\n  "stevearc/conform.nvim",\n  config = function()\n    require("conform").setup({ formatters_by_ft = { lua = { "stylua" }, go = { "gofmt" } } })\n  end,\n}\n' > $(DOTCOR_DIR)/nvim/lua/plugins/formatting.lua
	@test -f $(DOTCOR_DIR)/nvim/lua/plugins/linting.lua || printf 'return {\n  "mfussenegger/nvim-lint",\n  config = function() require("lint").linters_by_ft = { sh = { "shellcheck" } } end,\n}\n' > $(DOTCOR_DIR)/nvim/lua/plugins/linting.lua
	@test -f $(DOTCOR_DIR)/nvim/lua/plugins/autopairs.lua || printf 'return { "windwp/nvim-autopairs", config = function() require("nvim-autopairs").setup() end }\n' > $(DOTCOR_DIR)/nvim/lua/plugins/autopairs.lua
	@test -f $(DOTCOR_DIR)/nvim/lua/plugins/dashboard.lua || printf 'return {\n  "nvimdev/dashboard-nvim",\n  config = function() require("dashboard").setup({ theme = "hyper" }) end,\n}\n' > $(DOTCOR_DIR)/nvim/lua/plugins/dashboard.lua
	@test -f $(DOTCOR_DIR)/nvim/lua/plugins/git-conflict.lua || printf 'return { "akinsho/git-conflict.nvim", config = function() require("git-conflict").setup() end }\n' > $(DOTCOR_DIR)/nvim/lua/plugins/git-conflict.lua
	@mkdir -p $(DOTCOR_DIR)/nvim/lua/lsp
	@test -f $(DOTCOR_DIR)/nvim/lua/lsp/init.lua || printf 'local M = {}\nM.setup = function() require("lspconfig").gopls.setup({}) end\nreturn M\n' > $(DOTCOR_DIR)/nvim/lua/lsp/init.lua
	@test -f $(DOTCOR_DIR)/nvim/lua/lsp/completion.lua || printf 'local M = {}\nM.setup = function() require("cmp").setup({ sources = { { name = "nvim_lsp" } } }) end\nreturn M\n' > $(DOTCOR_DIR)/nvim/lua/lsp/completion.lua
	@# --- tmux (2 files) ---
	@test -f $(DOTCOR_DIR)/tmux/.tmux.conf || printf "set -g prefix C-a\nset -g base-index 1\nsetw -g pane-base-index 1\nset -g mouse on\nbind | split-window -h\nbind - split-window -v\n" > $(DOTCOR_DIR)/tmux/.tmux.conf
	@test -f $(DOTCOR_DIR)/tmux/.tmux.theme.conf || printf "set -g @catppuccin_flavour 'mocha'\nset -g @plugin 'catppuccin/tmux'\n" > $(DOTCOR_DIR)/tmux/.tmux.theme.conf
	@# --- zsh (1 file) ---
	@test -f $(DOTCOR_DIR)/zsh/.zshrc || printf "export ZSH=\"$$HOME/.oh-my-zsh\"\nZSH_THEME=\"robbyrussell\"\nplugins=(git zsh-autosuggestions zsh-syntax-highlighting)\nsource $$ZSH/oh-my-zsh.sh\n" > $(DOTCOR_DIR)/zsh/.zshrc
	@# --- starship ---
	@test -f $(DOTCOR_DIR)/starship/config.toml || printf "[character]\nsuccess_symbol = '[➜](bold green)'\nerror_symbol = '[✗](bold red)'\n\n[git_branch]\nformat = '[$$symbol$$branch]($$style) '\n" > $(DOTCOR_DIR)/starship/config.toml
	@# --- alacritty ---
	@test -f $(DOTCOR_DIR)/alacritty/alacritty.toml || printf "[window]\nopacity = 0.95\n\n[font]\nnormal.family = \"JetBrainsMono Nerd Font\"\nsize = 13.0\n" > $(DOTCOR_DIR)/alacritty/alacritty.toml
	@# --- bat ---
	@test -f $(DOTCOR_DIR)/bat/config || printf '%s\n' '--style="full"' '--italic-text=always' '--theme="Catppuccin Mocha"' > $(DOTCOR_DIR)/bat/config
	@# --- eza ---
	@test -f $(DOTCOR_DIR)/eza/config.yml || printf "icons: when\ncolor: always\nhyperlink: auto\n" > $(DOTCOR_DIR)/eza/config.yml
	@# --- fzf ---
	@test -f $(DOTCOR_DIR)/fzf/config.sh || printf 'export FZF_DEFAULT_OPTS='"'"'--height 40%% --layout=reverse --border'"'"'\nexport FZF_CTRL_T_OPTS='"'"'--preview "cat {}"'"'"'\n' > $(DOTCOR_DIR)/fzf/config.sh
	@# --- htop ---
	@test -f $(DOTCOR_DIR)/htop/htoprc || printf "fields=0 48 17 18 38 39 40 2 46 47 49 1\nsort_key=46\nsort_direction=1\nhide_threads=0\n" > $(DOTCOR_DIR)/htop/htoprc
	@# --- lazygit ---
	@test -f $(DOTCOR_DIR)/lazygit/config.yml || printf "gui:\n  showIcons: true\n  theme:\n    activeBorderColor:\n      - green\n      - bold\n" > $(DOTCOR_DIR)/lazygit/config.yml
	@# --- ripgrep ---
	@test -f $(DOTCOR_DIR)/ripgrep/config || printf '%s\n' '--smart-case' '--follow' '--hidden' '--glob=!.git/*' '--max-columns=150' > $(DOTCOR_DIR)/ripgrep/config
	@# --- ssh (2 files) ---
	@test -f $(DOTCOR_DIR)/ssh/config || printf "Host github.com\n  HostName github.com\n  User git\n  IdentityFile ~/.ssh/id_ed25519\n  AddKeysToAgent yes\n\nHost gitlab.com\n  HostName gitlab.com\n  User git\n  IdentityFile ~/.ssh/id_ed25519\n" > $(DOTCOR_DIR)/ssh/config
	@test -f $(DOTCOR_DIR)/ssh/known_hosts || printf "github.com ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIOMqqnkVzrm0SdG6UOoqKLsabgH5C9okWi0dh2l9GKJl\n" > $(DOTCOR_DIR)/ssh/known_hosts
	@# --- vim ---
	@test -f $(DOTCOR_DIR)/vim/vimrc || printf "set nocompatible\nset number\nset relativenumber\nset tabstop=4\nset shiftwidth=4\nset expandtab\nsyntax on\n" > $(DOTCOR_DIR)/vim/vimrc
	@# --- zoxide ---
	@test -f $(DOTCOR_DIR)/zoxide/config.sh || printf 'export _ZO_ECHO=1\nexport _ZO_FZF_OPTS="--height 40%%"\n' > $(DOTCOR_DIR)/zoxide/config.sh

sandbox: binary
	@rm -rf $(DOTCOR_DIR) $(DOTCOR_HOME)
	@$(MAKE) sandbox-setup
	DOTCOR_DIR=$(DOTCOR_DIR) DOTCOR_HOME=$(DOTCOR_HOME) ./bin/dotcor
