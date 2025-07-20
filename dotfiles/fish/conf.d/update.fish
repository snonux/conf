function update::tools
    echo "Installing/updating golangci-lint"
    go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
    echo "Installing/updating goimports"
    go install golang.org/x/tools/cmd/goimports@latest

    set pids
    for prog in tasksamurai timr
        echo "Installing/updating $prog from codeberg.org/snonux/$prog/cmd/$prog@latest"
        go install codeberg.org/snonux/$prog/cmd/$prog@latest &
        set -a pids $last_pid
    end
    for pid in $pids
        wait $pid
    end
    if test (uname) = Linux
        for prog in gos gitsyncer
            echo "Installing/updating $prog from codeberg.org/snonux/$prog/cmd/$prog@latest"
            go install codeberg.org/snonux/$prog/cmd/$prog@latest
        end
        echo "Installing/updating @anthropic-ai/claude-code globally via npm"
        doas npm uninstall -g @anthropic-ai/claude-code
        doas npm install -g @anthropic-ai/claude-code

        echo "Installing/updating @openai/codex globally via npm"
        doas npm uninstall -g @openai/codex
        doas npm install -g @openai/codex

        echo "Installing/updating @google/gemini-cli globally via npm"
        doas npm uninstall -g @google/gemini-cli
        doas npm install -g @google/gemini-cli

        echo "Installing/updating opencode-ai globally via npm"
        doas npm uninstall -g opencode-ai
        doas npm install -g opencode-ai
    end
end
