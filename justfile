set positional-arguments

dotcor_dir  := env("DOTCOR_DIR", "/tmp/dotcor-test")
dotcor_home := env("DOTCOR_HOME", "/tmp/dotcor-home")

# default recipe
default: lint test build

# run all tests
test:
    go test ./...

# run tests with coverage report
test-cover:
    go test ./... -coverprofile=coverage.out
    go tool cover -html=coverage.out

# run tests with verbose output
test-verbose:
    go test ./... -v

# build all packages
build:
    go build ./...

# build and run
run:
    go run cmd/dotcor/main.go

# clean build artifacts
clean:
    go clean
    rm -f coverage.out

# run linter
lint:
    /Users/justincordova/go/bin/golangci-lint run

# format all Go code
fmt:
    go fmt ./...

# run go vet
vet:
    go vet ./...

# download dependencies
deps:
    go mod download
    go mod tidy

# build the binary
binary:
    go build -o bin/dotcor cmd/dotcor/main.go

# install to GOPATH/bin
install:
    go install ./cmd/dotcor

# manual testing
test-manual:
    ./scripts/test-manual.sh full

test-interactive:
    ./scripts/test-manual.sh interactive

test-copy:
    ./scripts/test-manual.sh copy-dotfiles

test-clean:
    ./scripts/test-manual.sh clean

# ─── sandbox ──────────────────────────────────────────────────────────────────

sandbox-setup:
    #!/usr/bin/env bash
    set -euo pipefail
    home="{{dotcor_home}}"
    repo="{{dotcor_dir}}"
    old="{{dotcor_dir}}"/../old-dotfiles

    mkdir -p "$home"
    rm -rf "$old"
    mkdir -p "$old"

    # ─── helper: create repo file + symlink it into home (simulates stowed state) ──
    stow() {
        local pkg="$1" rel="$2" content="$3"
        local dst="$repo/$pkg/$rel"
        mkdir -p "$(dirname "$dst")"
        printf '%s' "$content" > "$dst"
        local target="$home/$rel"
        mkdir -p "$(dirname "$target")"
        ln -sf "$dst" "$target"
    }

    # ─── git (3 files, all stowed) ──────────────────────────────────────────────
    stow git .gitconfig       '[user]\n\tname = Test User\n\temail = test@example.com\n[core]\n\teditor = nvim\n[pull]\n\trebase = true\n'
    stow git .gitignore_global '.DS_Store\n*.swp\n*.swo\n*~\n.env\n'
    stow git .gitcommit        'refactor: simplify config loader\n\nCo-authored-by: Test User <test@example.com>\n'

    # ─── nvim (17 files, all stowed + 2 foreign + 1 conflict) ───────────────────
    stow nvim .config/nvim/init.lua                'require("config.lazy")\nrequire("config.options")\nrequire("config.keymaps")\nrequire("config.autocmds")\n'
    stow nvim .config/nvim/lua/config/options.lua  "vim.opt.number = true\nvim.opt.relativenumber = true\nvim.opt.tabstop = 4\nvim.opt.shiftwidth = 4\nvim.opt.expandtab = true\nvim.opt.signcolumn = 'yes'\n"
    stow nvim .config/nvim/lua/config/keymaps.lua  'vim.g.mapleader = " "\nvim.keymap.set("n", "<leader>ff", "<cmd>Telescope find_files<cr>")\n'
    stow nvim .config/nvim/lua/config/autocmds.lua 'vim.api.nvim_create_autocmd("TextYankPost", {\n  callback = function() vim.highlight.on_yank() end,\n})\n'
    stow nvim .config/nvim/lua/config/lazy.lua     'local lazypath = vim.fn.stdpath("data") .. "/lazy/lazy.nvim"\nvim.opt.rtp:prepend(lazypath)\nrequire("lazy").setup("plugins")\n'
    stow nvim .config/nvim/lua/config/utils.lua    'local M = {}\nM.log = function(msg) vim.notify(msg, vim.log.levels.INFO) end\nreturn M\n'
    stow nvim .config/nvim/lua/plugins/telescope.lua 'return {\n  "nvim-telescope/telescope.nvim",\n  dependencies = { "nvim-lua/plenary.nvim" },\n  config = function() require("telescope").setup() end,\n}\n'
    stow nvim .config/nvim/lua/plugins/lualine.lua   'return {\n  "nvim-lualine/lualine.nvim",\n  config = function() require("lualine").setup({ options = { theme = "catppuccin" } }) end,\n}\n'
    stow nvim .config/nvim/lua/plugins/treesitter.lua 'return {\n  "nvim-treesitter/nvim-treesitter",\n  build = ":TSUpdate",\n  config = function() require("nvim-treesitter.configs").setup({ ensure_installed = "all" }) end,\n}\n'
    stow nvim .config/nvim/lua/plugins/neo-tree.lua   'return {\n  "nvim-neo-tree/neo-tree.nvim",\n  dependencies = { "nvim-lua/plenary.nvim", "nvim-tree/nvim-web-devicons" },\n  config = function() vim.keymap.set("n", "<leader>e", "<cmd>Neotree toggle<cr>") end,\n}\n'
    stow nvim .config/nvim/lua/plugins/formatting.lua 'return {\n  "stevearc/conform.nvim",\n  config = function()\n    require("conform").setup({ formatters_by_ft = { lua = { "stylua" }, go = { "gofmt" } } })\n  end,\n}\n'
    stow nvim .config/nvim/lua/plugins/linting.lua    'return {\n  "mfussenegger/nvim-lint",\n  config = function() require("lint").linters_by_ft = { sh = { "shellcheck" } } end,\n}\n'
    stow nvim .config/nvim/lua/plugins/autopairs.lua  'return { "windwp/nvim-autopairs", config = function() require("nvim-autopairs").setup() end }\n'
    stow nvim .config/nvim/lua/plugins/dashboard.lua   'return {\n  "nvimdev/dashboard-nvim",\n  config = function() require("dashboard").setup({ theme = "hyper" }) end,\n}\n'
    stow nvim .config/nvim/lua/plugins/git-conflict.lua 'return { "akinsho/git-conflict.nvim", config = function() require("git-conflict").setup() end }\n'
    stow nvim .config/nvim/lua/lsp/init.lua            'local M = {}\nM.setup = function() require("lspconfig").gopls.setup({}) end\nreturn M\n'
    stow nvim .config/nvim/lua/lsp/completion.lua      'local M = {}\nM.setup = function() require("cmp").setup({ sources = { { name = "nvim_lsp" } } }) end\nreturn M\n'

    # ─── nvim: foreign symlinks (from old GNU stow setup, for adopt testing) ─────
    mkdir -p "$old"/nvim/.config/nvim/lua/{plugins,config}
    printf 'return { "nvim-cmp" }\n' > "$old"/nvim/.config/nvim/lua/plugins/cmp.lua
    printf 'vim.opt.scrolloff = 8\n' > "$old"/nvim/.config/nvim/lua/config/autocmds_old.lua
    ln -sf "$old"/nvim/.config/nvim/lua/plugins/cmp.lua "$home"/.config/nvim/lua/plugins/cmp.lua
    ln -sf "$old"/nvim/.config/nvim/lua/config/autocmds_old.lua "$home"/.config/nvim/lua/config/autocmds_old.lua

    # ─── nvim: conflict (regular file blocking, not a symlink) ──────────────────
    printf 'old colorscheme config\n' > "$home"/.config/nvim/lua/config/colors.lua

    # ─── tmux (2 files, all stowed) ─────────────────────────────────────────────
    stow tmux .tmux.conf       'set -g prefix C-a\nset -g base-index 1\nsetw -g pane-base-index 1\nset -g mouse on\nbind | split-window -h\nbind - split-window -v\n'
    stow tmux .tmux.theme.conf "set -g @catppuccin_flavour 'mocha'\nset -g @plugin 'catppuccin/tmux'\n"

    # ─── zsh (1 file, stowed) ───────────────────────────────────────────────────
    stow zsh .zshrc 'export ZSH="$HOME/.oh-my-zsh"\nZSH_THEME="robbyrussell"\nplugins=(git zsh-autosuggestions zsh-syntax-highlighting)\nsource $ZSH/oh-my-zsh.sh\n'

    # ─── starship (stowed) ──────────────────────────────────────────────────────
    stow starship .config/starship/config.toml '[character]\nsuccess_symbol = '\''[➜](bold green)'\''\nerror_symbol = '\''[✗](bold red)'\''\n\n[git_branch]\nformat = '\''[$symbol$branch]($style) '\''\n'

    # ─── alacritty (stowed) ─────────────────────────────────────────────────────
    stow alacritty .config/alacritty/alacritty.toml '[window]\nopacity = 0.95\n\n[font]\nnormal.family = "JetBrainsMono Nerd Font"\nsize = 13.0\n'

    # ─── bat (stowed) ───────────────────────────────────────────────────────────
    stow bat .config/bat/config '--style="full"\n--italic-text=always\n--theme="Catppuccin Mocha"\n'

    # ─── eza (stowed) ───────────────────────────────────────────────────────────
    stow eza .config/eza/config.yml 'icons: when\ncolor: always\nhyperlink: auto\n'

    # ─── fzf (stowed) ───────────────────────────────────────────────────────────
    stow fzf .config/fzf/config.sh 'export FZF_DEFAULT_OPTS='\''--height 40%% --layout=reverse --border'\''\nexport FZF_CTRL_T_OPTS='\''--preview "cat {}"'\''\n'

    # ─── htop (stowed) ──────────────────────────────────────────────────────────
    stow htop .config/htop/htoprc 'fields=0 48 17 18 38 39 40 2 46 47 49 1\nsort_key=46\nsort_direction=1\nhide_threads=0\n'

    # ─── lazygit (stowed) ───────────────────────────────────────────────────────
    stow lazygit .config/lazygit/config.yml 'gui:\n  showIcons: true\n  theme:\n    activeBorderColor:\n      - green\n      - bold\n'

    # ─── ripgrep (stowed) ───────────────────────────────────────────────────────
    stow ripgrep .config/ripgrep/config '--smart-case\n--follow\n--hidden\n--glob=!.git/*\n--max-columns=150\n'

    # ─── ssh (2 files, stowed) ──────────────────────────────────────────────────
    stow ssh .ssh/config      'Host github.com\n  HostName github.com\n  User git\n  IdentityFile ~/.ssh/id_ed25519\n  AddKeysToAgent yes\n\nHost gitlab.com\n  HostName gitlab.com\n  User git\n  IdentityFile ~/.ssh/id_ed25519\n'
    stow ssh .ssh/known_hosts 'github.com ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIOMqqnkVzrm0SdG6UOoqKLsabgH5C9okWi0dh2l9GKJl\n'

    # ─── vim (stowed) ───────────────────────────────────────────────────────────
    stow vim vimrc 'set nocompatible\nset number\nset relativenumber\nset tabstop=4\nset shiftwidth=4\nset expandtab\nsyntax on\n'

    # ─── zoxide (stowed) ────────────────────────────────────────────────────────
    stow zoxide .config/zoxide/config.sh 'export _ZO_ECHO=1\nexport _ZO_FZF_OPTS="--height 40%%"\n'

    # ─── unmanaged files in home (for testing the add wizard) ───────────────────
    printf 'export PATH=$HOME/.local/bin:$PATH\nalias ll='\''ls -la'\''\nalias gs='\''git status'\''\n' > "$home"/.bashrc
    printf 'if [ -f $HOME/.bashrc ]; then\n  . $HOME/.bashrc\nfi\n' > "$home"/.profile
    printf '"\e[A": history-search-backward\n"\e[B": history-search-forward\nset completion-ignore-case on\n' > "$home"/.inputrc
    printf 'root = true\n\n[*]\nindent_style = tab\nindent_size = 4\nend_of_line = lf\ncharset = utf-8\ntrim_trailing_whitespace = true\n' > "$home"/.editorconfig

sandbox: (binary)
    #!/usr/bin/env bash
    set -euo pipefail
    rm -rf "{{dotcor_dir}}" "{{dotcor_home}}" "{{dotcor_dir}}"/../old-dotfiles
    just sandbox-setup
    DOTCOR_DIR="{{dotcor_dir}}" DOTCOR_HOME="{{dotcor_home}}" ./bin/dotcor
