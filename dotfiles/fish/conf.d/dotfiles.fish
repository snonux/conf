set -gx DOTFILES_DIR ~/git/conf/dotfiles

function dotfiles::update
    set -l prev_pwd (pwd)
    cd $DOTFILES_DIR
    rex home
    cd "$prev_pwd"
end

function dotfiles::update::git
    set -l prev_pwd (pwd)
    cd $DOTFILES_DIR
    git pull
    git commit -a
    git push
    rex home
    cd "$prev_pwd"
end

function dotfiles::fuzzy::edit
    set -l prev_pwd (pwd)
    cd $DOTFILES_DIR
    set -l dotfile (find . -type f -not -path '*/.git/*' | fzf)
    $EDITOR "$dotfile"
    if echo "$dotfile" | grep -F -q .fish
        echo "Sourcing $dotfile"
        source "$dotfile"
    end
    cd "$prev_pwd"
end

function dotfiles::rexify
    cd $DOTFILES_DIR
    rex home
    cd -
end

function dotfiles::random::edit
    $EDITOR (find $DOTFILES_DIR -type f -not -path '*/.git/*' | shuf -n 1)
end

abbr -a .u 'dotfiles::update'
abbr -a .ug 'dotfiles::update::git'
abbr -a .e 'dotfiles::fuzzy::edit'
abbr -a .rex 'dotfiles::rexify'
abbr -a .re 'dotfiles::random::edit'
abbr -a cdconf "cd $HOME/git/conf"
abbr -a cdotfiles "cd $HOME/git/conf/dotfiles"
