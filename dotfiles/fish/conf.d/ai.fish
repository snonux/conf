abbr -a gpt chatgpt
abbr -a gpti "chatgpt --interactive"
abbr -a suggest hexai
abbr -a explain 'hexai explain'
abbr -a aic 'aichat -e'

# helix-gpt env vars used
# set -gx COPILOT_MODEL gpt-4.1 # can be changed with aimodels function
set -gx COPILOT_MODEL gpt-4o # can be changed with aimodels function
set -gx HANDLER copilot
set -gx HEXAI_PROVIDER openai

# TODO: also reconfigure aichat tool using this function
function aimodels
    # nvim for the ai tool wrapper so i can use Copilot Chat from the command line.
    set -l NVIM_DIR "$HOME/.config/nvim/"
    set -l COPILOT_CHAT_DIR "$NVIM_DIR/pack/copilotchat/start/CopilotChat.nvim/lua/CopilotChat"

    printf "gpt-4o
gpt-5
gpt-o3
gpt-4.1
claude-3.7-sonnet
claude-3.7-sonnet-thought
claude-4.0-sonnet
gemini-2.5-pro" >~/.aimodels

    set -gx COPILOT_MODEL (cat ~/.aimodels | fzf)
    set -gx OPENAI_MODEL $COPILOT_MODEL

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
