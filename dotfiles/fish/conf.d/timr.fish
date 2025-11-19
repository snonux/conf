function timr_prompt -d "Display timr timr_status in the prompt"
    if command -v timr >/dev/null
        set -l timr_status (timr prompt)
        if test -n "$timr_status"
            set -l icon (string sub -l 1 -- "$timr_status")
            set -l time (string sub -s 2 -- "$timr_status")
            if test "$icon" = "▶"
                set_color green
            else
                set_color yellow
            end
            printf '%s' "$icon"
            set_color normal
            printf ' %s' "$time"
        end
    end
end

complete -c timr -n __fish_use_subcommand -a start -d "Start the timer"
complete -c timr -n __fish_use_subcommand -a stop -d "Stop the timer"
complete -c timr -n __fish_use_subcommand -a pause -d "Pause the timer"
complete -c timr -n __fish_use_subcommand -a status -d "Show the timer status"
complete -c timr -n __fish_use_subcommand -a reset -d "Reset the timer"
complete -c timr -n __fish_use_subcommand -a live -d "Show the live timer"
complete -c timr -n __fish_use_subcommand -a prompt -d "Show the prompt status"

