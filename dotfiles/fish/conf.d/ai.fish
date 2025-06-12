abbr -a gpt chatgpt
abbr -a gpti "chatgpt --interactive"
abbr -a suggest 'gh copilot suggest'
abbr -a explain 'gh copilot explain'
abbr -a aic 'aichat -e'

# helix-gpt env vars used
# set -gx COPILOT_MODEL gpt-4.1 # can be changed with aimodels function
set -gx COPILOT_MODEL o3 # can be changed with aimodels function
set -gx HANDLER copilot

# TODO: also reconfigure aichat tool using this function
function aimodels
    # nvim for the ai tool wrapper so i can use Copilot Chat from the command line.
    set -l NVIM_DIR "$HOME/.config/nvim/"
    set -l COPILOT_CHAT_DIR "$NVIM_DIR/pack/copilotchat/start/CopilotChat.nvim/lua/CopilotChat"

    printf "gpt-4o
gpt-o3
gpt-4.1
claude-3.7-sonnet
claude-3.7-sonnet-thought" >~/.aimodels

    set -gx COPILOT_MODEL (cat ~/.aimodels | fzf)

    if test -d $COPILOT_CHAT_DIR
        set -l model_config "$COPILOT_CHAT_DIR/config-$COPILOT_MODEL.lua"
        if test -f "$model_config"
            echo "Using CopilotChat config from $model_config"
            cp -v $model_config "$COPILOT_CHAT_DIR/config.lua"
        else
            echo "No config found at $model_config"
        end
    end
end
