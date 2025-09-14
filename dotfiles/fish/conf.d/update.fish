function update::tools
    set pids

    echo "Installing/updating gofumpt"
    go install mvdan.cc/gofumpt@latest &
    set -a pids $last_pid

    echo "Installing/updating mage"
    go install github.com/magefile/mage@latest &
    set -a pids $last_pid

    echo "Installing/updating golangci-lint"
    go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest &
    set -a pids $last_pid

    echo "Installing/updating goimports"
    go install golang.org/x/tools/cmd/goimports@latest &
    set -a pids $last_pid

    for prog in hexai hexai-lsp hexai-tmux-action
        echo "Installing/updating $prog from codeberg.org/snonux/hexai/cmd/$prog@latest"
        go install codeberg.org/snonux/hexai/cmd/$prog@latest &
        set -a pids $last_pid
    end

    for prog in tasksamurai timr
        echo "Installing/updating $prog from codeberg.org/snonux/$prog/cmd/$prog@latest"
        go install codeberg.org/snonux/$prog/cmd/$prog@latest &
        set -a pids $last_pid
    end

    if test (uname) = Darwin
        echo 'Updating cursor-agent on macOS'
        cursor-agent update
    end
    set -a pids $last_pid

    if test (uname) = Linux
        eco "Installing/updating tgpt"
        go install github.com/aandrew-me/tgpt/v2@latest &
        set -a pids $last_pid

        for prog in gos gitsyncer
            echo "Installing/updating $prog from codeberg.org/snonux/$prog/cmd/$prog@latest"
            go install codeberg.org/snonux/$prog/cmd/$prog@latest
        end

        echo "Installing/updating @anthropic-ai/claude-code globally via npm"
        doas npm uninstall -g @anthropic-ai/claude-code
        doas npm install -g @anthropic-ai/claude-code

        # doas npm uninstall -g @qwen-code/qwen-code@latest
        # doas npm install -g @qwen-code/qwen-code@latest

        echo "Installing/updating @openai/codex globally via npm"
        doas npm uninstall -g @openai/codex
        doas npm install -g @openai/codex

        echo "Installing/updating @google/gemini-cli globally via npm"
        doas npm uninstall -g @google/gemini-cli
        doas npm install -g @google/gemini-cli

        # echo "Installing/updating @sourcegraph/amp globally via npm"
        # doas npm uninstall -g @sourcegraph/amp
        # doas npm install -g @sourcegraph/amp

        echo "Installing/updating opencode-ai globally via npm"
        doas npm uninstall -g opencode-ai
        doas npm install -g opencode-ai
    end

    for pid in $pids
        wait $pid
    end
end
