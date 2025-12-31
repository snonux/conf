function taskwarrior::fuzzy::_select
    sed -n '/^[0-9]/p' | sort -rn | fzf | cut -d' ' -f1
end

function taskwarrior::fuzzy::find
    set -g TASK_ID (task ready | taskwarrior::fuzzy::_select)
end

function taskwarrior::select
    set -l task_id "$argv[1]"
    if test -n "$task_id"
        set -g TASK_ID "$task_id"
    end
    if test "$TASK_ID" = - -o -z "$TASK_ID"
        taskwarrior::fuzzy::find
    end
end

function taskwarrior::due::count
    set -l due_count (task status:pending due.before:now count)

    if test $due_count -gt 0
        echo "There are $due_count tasks due!"
    end
end

function taskwarrior::add::track
    if test (count $argv) -gt 0
        task add priority:L +personal +track $argv
    else
        tasksamurai +track
    end
end

function taskwarrior::add::standup
    if test (count $argv) -gt 0
        task add priority:L +work +standup +sre +nosched $argv
        task add priority:L +work +standup +storage +nosched $argv

        if test -f ~/git/helpers/jira/jira.rb
            echo "Do you want to raise a Jira ticket? (y/n)"
            read -l user_input
            if test "$user_input" = y
                ruby ~/git/helpers/jira/jira.rb --raise "$argv"
            end
        end

    else
        tasksamurai +standup
    end
end

function taskwarrior::add::standup::editor
    set -l tmpfile (mktemp /tmp/standup.XXXXXX.txt)
    $EDITOR $tmpfile
    taskwarrior::add::standup (cat $tmpfile)
end

function _taskwarrior::set_import_export_tags
    if test (uname) = Darwin
        set -gx TASK_IMPORT_TAG work
        set -gx TASK_EXPORT_TAG personal
    else
        set -gx TASK_IMPORT_TAG personal
        set -gx TASK_EXPORT_TAG work
    end
end

function taskwarrior::export::bd
    # Export bulgarian dumi
    set -l outfile ~/Notes/Bulgarian/bd-(date +%s).txt
    task +bd status:pending export | jq -r '.[].description' >$outfile
    yes | task +bd status:pending delete
    # Delete it if it is empty
    if test -f $outfile; and test ! -s $outfile
        rm $outfile
    end
end

function taskwarrior::export::gos
    task +share status:pending export >"$WORKTIME_DIR/tw-gos-export-$(date +%s).json"
    yes | task +share status:pending delete
end

function taskwarrior::export
    _taskwarrior::set_import_export_tags
    set -l count (task +$TASK_EXPORT_TAG status:pending count)

    if test $count -eq 0
        return
    end

    echo "Exporting $count tasks to $TASK_EXPORT_TAG"
    task +$TASK_EXPORT_TAG status:pending export >"$WORKTIME_DIR/tw-$TASK_EXPORT_TAG-export-$(date +%s).json"
    yes | task +$TASK_EXPORT_TAG status:pending delete
end

function taskwarrior::import
    _taskwarrior::set_import_export_tags

    find $WORKTIME_DIR -name "tw-$TASK_IMPORT_TAG-export-*.json" | while read -l import
        task import $import
        rm $import
    end

    find $WORKTIME_DIR -name "tw-(hostname)-export-*.json" | while read -l import
        task import $import
        rm $import
    end
end

abbr -a t task
abbr -a L 'task add +log'
abbr -a tlog 'task add +log'
abbr -a log 'task add +log'
abbr -a tdue 'tasksamurai status:pending due.before:now'
abbr -a thome 'tasksamurai +home'
abbr -a tasks 'tasksamurai -track'
abbr -a tread 'tasksamurai +read'
abbr -a track 'taskwarrior::add::track'
abbr -a tra 'taskwarrior::add::track'
abbr -a trat 'timr track'
abbr -a tfind 'taskwarrior::fuzzy::find'
abbr -a ts tasksamurai

# Virtual standup abbrs
abbr -a V 'taskwarrior::add::standup'
abbr -a Vstorage 'tasksamurai +standup +storage'
abbr -a Vsre 'tasksamurai +standup +sre'
abbr -a Ved 'taskwarrior::add::standup::editor'

taskwarrior::due::count
