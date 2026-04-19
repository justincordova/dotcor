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
    mkdir -p "{{dotcor_home}}"
    mkdir -p "{{dotcor_dir}}"/{git,nvim,starship,tmux,zsh,alacritty,bat,eza,fzf,htop,lazygit,ripgrep,ssh,vim,zoxide}

    # unmanaged files in home for testing the add wizard
    home="{{dotcor_home}}"
    test -f "$home"/.bashrc       || printf 'export PATH=$HOME/.local/bin:$PATH\nalias ll='\''ls -la'\''\nalias gs='\''git status'\''\n' > "$home"/.bashrc
    test -f "$home"/.profile      || printf 'if [ -f $HOME/.bashrc ]; then\n  . $HOME/.bashrc\nfi\n' > "$home"/.profile
    test -f "$home"/.inputrc      || printf '"\e[A": history-search-backward\n"\e[B": history-search-forward\nset completion-ignore-case on\n' > "$home"/.inputrc
    test -f "$home"/.editorconfig || printf 'root = true\n\n[*]\nindent_style = tab\nindent_size = 4\nend_of_line = lf\ncharset = utf-8\ntrim_trailing_whitespace = true\n' > "$home"/.editorconfig
    mkdir -p "$home"/.config/tmux
    test -f "$home"/.config/tmux/tmux.conf || printf 'set -g prefix C-a\nbind | split-window -h\nbind - split-window -v\n' > "$home"/.config/tmux/tmux.conf
    mkdir -p "$home"/.config/starship
    test -f "$home"/.config/starship.toml || printf '[character]\nsuccess_symbol = '\''[➜](bold green)'\''\n' > "$home"/.config/starship.toml

    # unmanaged nvim config tree for testing folder add
    mkdir -p "$home"/.config/nvim/lua/{config,plugins}
    test -f "$home"/.config/nvim/init.lua                  || printf 'require("config.lazy")\nrequire("config.options")\n' > "$home"/.config/nvim/init.lua
    test -f "$home"/.config/nvim/lua/config/options.lua    || printf 'vim.opt.number = true\nvim.opt.relativenumber = true\n' > "$home"/.config/nvim/lua/config/options.lua
    test -f "$home"/.config/nvim/lua/config/keymaps.lua    || printf 'vim.g.mapleader = " "\n' > "$home"/.config/nvim/lua/config/keymaps.lua
    test -f "$home"/.config/nvim/lua/plugins/telescope.lua || printf 'return { "nvim-telescope/telescope.nvim" }\n' > "$home"/.config/nvim/lua/plugins/telescope.lua
    test -f "$home"/.config/nvim/lua/plugins/lualine.lua   || printf 'return { "nvim-lualine/lualine.nvim" }\n' > "$home"/.config/nvim/lua/plugins/lualine.lua

    repo="{{dotcor_dir}}"

    # git (3 files)
    test -f "$repo"/git/.gitconfig       || printf '[user]\n\tname = Test User\n\temail = test@example.com\n[core]\n\teditor = nvim\n[pull]\n\trebase = true\n' > "$repo"/git/.gitconfig
    test -f "$repo"/git/.gitignore_global || printf '.DS_Store\n*.swp\n*.swo\n*~\n.env\n' > "$repo"/git/.gitignore_global
    test -f "$repo"/git/.gitcommit        || printf 'refactor: simplify config loader\n\nCo-authored-by: Test User <test@example.com>\n' > "$repo"/git/.gitcommit

    # nvim (deep tree ~20 files)
    test -f "$repo"/nvim/init.lua                  || printf 'require("config.lazy")\nrequire("config.options")\nrequire("config.keymaps")\nrequire("config.autocmds")\n' > "$repo"/nvim/init.lua
    mkdir -p "$repo"/nvim/lua/{config,plugins,lsp}
    test -f "$repo"/nvim/lua/config/options.lua    || printf "vim.opt.number = true\nvim.opt.relativenumber = true\nvim.opt.tabstop = 4\nvim.opt.shiftwidth = 4\nvim.opt.expandtab = true\nvim.opt.signcolumn = 'yes'\n" > "$repo"/nvim/lua/config/options.lua
    test -f "$repo"/nvim/lua/config/keymaps.lua    || printf 'vim.g.mapleader = " "\nvim.keymap.set("n", "<leader>ff", "<cmd>Telescope find_files<cr>")\nvim.keymap.set("n", "<leader>fg", "<cmd>Telescope live_grep<cr>")\nvim.keymap.set("n", "<leader>fb", "<cmd>Telescope buffers<cr>")\n' > "$repo"/nvim/lua/config/keymaps.lua
    test -f "$repo"/nvim/lua/config/autocmds.lua   || printf 'vim.api.nvim_create_autocmd("TextYankPost", {\n  callback = function() vim.highlight.on_yank() end,\n})\n' > "$repo"/nvim/lua/config/autocmds.lua
    test -f "$repo"/nvim/lua/config/lazy.lua       || printf 'local lazypath = vim.fn.stdpath("data") .. "/lazy/lazy.nvim"\nvim.opt.rtp:prepend(lazypath)\nrequire("lazy").setup("plugins")\n' > "$repo"/nvim/lua/config/lazy.lua
    test -f "$repo"/nvim/lua/config/utils.lua      || printf 'local M = {}\nM.log = function(msg) vim.notify(msg, vim.log.levels.INFO) end\nreturn M\n' > "$repo"/nvim/lua/config/utils.lua
    test -f "$repo"/nvim/lua/plugins/telescope.lua || printf 'return {\n  "nvim-telescope/telescope.nvim",\n  dependencies = { "nvim-lua/plenary.nvim" },\n  config = function() require("telescope").setup() end,\n}\n' > "$repo"/nvim/lua/plugins/telescope.lua
    test -f "$repo"/nvim/lua/plugins/lualine.lua   || printf 'return {\n  "nvim-lualine/lualine.nvim",\n  config = function() require("lualine").setup({ options = { theme = "catppuccin" } }) end,\n}\n' > "$repo"/nvim/lua/plugins/lualine.lua
    test -f "$repo"/nvim/lua/plugins/treesitter.lua || printf 'return {\n  "nvim-treesitter/nvim-treesitter",\n  build = ":TSUpdate",\n  config = function() require("nvim-treesitter.configs").setup({ ensure_installed = "all" }) end,\n}\n' > "$repo"/nvim/lua/plugins/treesitter.lua
    test -f "$repo"/nvim/lua/plugins/neo-tree.lua   || printf 'return {\n  "nvim-neo-tree/neo-tree.nvim",\n  dependencies = { "nvim-lua/plenary.nvim", "nvim-tree/nvim-web-devicons" },\n  config = function() vim.keymap.set("n", "<leader>e", "<cmd>Neotree toggle<cr>") end,\n}\n' > "$repo"/nvim/lua/plugins/neo-tree.lua
    test -f "$repo"/nvim/lua/plugins/formatting.lua || printf 'return {\n  "stevearc/conform.nvim",\n  config = function()\n    require("conform").setup({ formatters_by_ft = { lua = { "stylua" }, go = { "gofmt" } } })\n  end,\n}\n' > "$repo"/nvim/lua/plugins/formatting.lua
    test -f "$repo"/nvim/lua/plugins/linting.lua    || printf 'return {\n  "mfussenegger/nvim-lint",\n  config = function() require("lint").linters_by_ft = { sh = { "shellcheck" } } end,\n}\n' > "$repo"/nvim/lua/plugins/linting.lua
    test -f "$repo"/nvim/lua/plugins/autopairs.lua  || printf 'return { "windwp/nvim-autopairs", config = function() require("nvim-autopairs").setup() end }\n' > "$repo"/nvim/lua/plugins/autopairs.lua
    test -f "$repo"/nvim/lua/plugins/dashboard.lua   || printf 'return {\n  "nvimdev/dashboard-nvim",\n  config = function() require("dashboard").setup({ theme = "hyper" }) end,\n}\n' > "$repo"/nvim/lua/plugins/dashboard.lua
    test -f "$repo"/nvim/lua/plugins/git-conflict.lua || printf 'return { "akinsho/git-conflict.nvim", config = function() require("git-conflict").setup() end }\n' > "$repo"/nvim/lua/plugins/git-conflict.lua
    test -f "$repo"/nvim/lua/lsp/init.lua            || printf 'local M = {}\nM.setup = function() require("lspconfig").gopls.setup({}) end\nreturn M\n' > "$repo"/nvim/lua/lsp/init.lua
    test -f "$repo"/nvim/lua/lsp/completion.lua      || printf 'local M = {}\nM.setup = function() require("cmp").setup({ sources = { { name = "nvim_lsp" } } }) end\nreturn M\n' > "$repo"/nvim/lua/lsp/completion.lua

    # tmux (2 files)
    test -f "$repo"/tmux/.tmux.conf       || printf 'set -g prefix C-a\nset -g base-index 1\nsetw -g pane-base-index 1\nset -g mouse on\nbind | split-window -h\nbind - split-window -v\n' > "$repo"/tmux/.tmux.conf
    test -f "$repo"/tmux/.tmux.theme.conf || printf "set -g @catppuccin_flavour 'mocha'\nset -g @plugin 'catppuccin/tmux'\n" > "$repo"/tmux/.tmux.theme.conf

    # zsh (1 file)
    test -f "$repo"/zsh/.zshrc || printf 'export ZSH="$HOME/.oh-my-zsh"\nZSH_THEME="robbyrussell"\nplugins=(git zsh-autosuggestions zsh-syntax-highlighting)\nsource $ZSH/oh-my-zsh.sh\n' > "$repo"/zsh/.zshrc

    # starship
    test -f "$repo"/starship/config.toml || printf '[character]\nsuccess_symbol = '\''[➜](bold green)'\''\nerror_symbol = '\''[✗](bold red)'\''\n\n[git_branch]\nformat = '\''[$symbol$branch]($style) '\''\n' > "$repo"/starship/config.toml

    # alacritty
    test -f "$repo"/alacritty/alacritty.toml || printf '[window]\nopacity = 0.95\n\n[font]\nnormal.family = "JetBrainsMono Nerd Font"\nsize = 13.0\n' > "$repo"/alacritty/alacritty.toml

    # bat
    test -f "$repo"/bat/config || printf '%s\n' '--style="full"' '--italic-text=always' '--theme="Catppuccin Mocha"' > "$repo"/bat/config

    # eza
    test -f "$repo"/eza/config.yml || printf 'icons: when\ncolor: always\nhyperlink: auto\n' > "$repo"/eza/config.yml

    # fzf
    test -f "$repo"/fzf/config.sh || printf 'export FZF_DEFAULT_OPTS='\''--height 40%% --layout=reverse --border'\''\nexport FZF_CTRL_T_OPTS='\''--preview "cat {}"'\''\n' > "$repo"/fzf/config.sh

    # htop
    test -f "$repo"/htop/htoprc || printf 'fields=0 48 17 18 38 39 40 2 46 47 49 1\nsort_key=46\nsort_direction=1\nhide_threads=0\n' > "$repo"/htop/htoprc

    # lazygit
    test -f "$repo"/lazygit/config.yml || printf 'gui:\n  showIcons: true\n  theme:\n    activeBorderColor:\n      - green\n      - bold\n' > "$repo"/lazygit/config.yml

    # ripgrep
    test -f "$repo"/ripgrep/config || printf '%s\n' '--smart-case' '--follow' '--hidden' '--glob=!.git/*' '--max-columns=150' > "$repo"/ripgrep/config

    # ssh (2 files)
    test -f "$repo"/ssh/config       || printf 'Host github.com\n  HostName github.com\n  User git\n  IdentityFile ~/.ssh/id_ed25519\n  AddKeysToAgent yes\n\nHost gitlab.com\n  HostName gitlab.com\n  User git\n  IdentityFile ~/.ssh/id_ed25519\n' > "$repo"/ssh/config
    test -f "$repo"/ssh/known_hosts  || printf 'github.com ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIOMqqnkVzrm0SdG6UOoqKLsabgH5C9okWi0dh2l9GKJl\n' > "$repo"/ssh/known_hosts

    # vim
    test -f "$repo"/vim/vimrc || printf 'set nocompatible\nset number\nset relativenumber\nset tabstop=4\nset shiftwidth=4\nset expandtab\nsyntax on\n' > "$repo"/vim/vimrc

    # zoxide
    test -f "$repo"/zoxide/config.sh || printf 'export _ZO_ECHO=1\nexport _ZO_FZF_OPTS="--height 40%%"\n' > "$repo"/zoxide/config.sh

sandbox: (binary)
    #!/usr/bin/env bash
    set -euo pipefail
    rm -rf "{{dotcor_dir}}" "{{dotcor_home}}"
    just sandbox-setup
    DOTCOR_DIR="{{dotcor_dir}}" DOTCOR_HOME="{{dotcor_home}}" ./bin/dotcor
