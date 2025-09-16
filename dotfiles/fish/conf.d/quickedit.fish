set -gx QUICKEDIT_DIR ~/QuickEdit

function quickedit::postaction
    set -l file_path $argv[1]
    set -l make_run 0

    if test -f Makefile
        make
        set make_run 1
    end

    # Go to git toplevel dir (if exists)
    cd (dirname $file_path)
    set -l git_dir (git rev-parse --show-toplevel 2>/dev/null)
    if test $status -eq 0
        cd $git_dir
    end
    if not test $make_run -eq 1
        if test -f Makefile
            make
        end
    end
    if test -d .git
        git commit -a -m Update
        git pull
        git push
    end
end

function quickedit
    set -l prev_dir (pwd)
    set -l grep_pattern .

    if test (count $argv) -gt 0
        set grep_pattern $argv[1]
    end

    cd $QUICKEDIT_DIR
    set files (find -L . -type f -not -path '*/.*' | grep -E "$grep_pattern")

    switch (count $files)
        case 0
            echo No result found
            return
        case 1
            set file_path $files[1]
        case '*'
            set file_path (printf '%s\n' $files | fzf)
    end

    if editor::helix::open_with_lock $file_path
        quickedit::postaction $file_path
    end

    cd $prev_dir
end

function quickedit::direct
    set -l dir $argv[1]
    set -l file $argv[2]
    cd $dir

    if editor::helix::open_with_lock $file
        quickedit::postaction $file
    end

    cd -
end

function quickedit::scratchpad
    quickedit::direct ~/Notes Scratchpad.md
end

function quickedit::quicknote
    quickedit::direct ~/Notes QuickNote.md
end

function quickedit::performance
    quickedit::direct ~/Notes Performance.md
end

abbr -a e quickedit
abbr -a scratch quickedit::scratchpad
abbr -a S quickedit::scratchpad
abbr -a quicknote quickedit::quicknote
abbr -a performance quickedit::performance
abbr -a goals quickedit::performance
abbr -a er "ranger $QUICKEDIT_DIR"
abbr -a cdquickedit "cd $QUICKEDIT_DIR"
abbr -a cdnotes 'cd ~/Notes'
abbr -a cdfish 'cd ~/.config/fish/conf.d'
abbr -a cddocs 'cd ~/Documents'
abbr -a cdocs 'cd ~/Documents'
