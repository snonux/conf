set -x SUPERSYNC_STAMP_FILE ~/.supersync.last

# Only sync the HabitsAndQuotes when it's asked for via function parameter
function supersync::worktime
    set -l worktime_dir ~/git/worktime

    if not test -d $worktime_dir
        echo "Warning: Directory $worktime_dir does not exist"
        return 1
    end
    cd $worktime_dir

    if test (count $argv) -gt 0 -a $argv[1] = sync_quotes
        if test -d ~/Notes/HabitsAndQuotes
            echo "" >work-wisdoms.md.tmp
            for notes in ~/Notes/HabitsAndQuotes/{Productivity,Mentoring}.md
                grep '^\* ' $notes >>work-wisdoms.md.tmp
            end
            sort -u work-wisdoms.md.tmp >work-wisdoms.md
            rm work-wisdoms.md.tmp
            git add work-wisdoms.md
            grep '^\* ' ~/Notes/HabitsAndQuotes/Exercise.md >exercises.md
            git add exercises.md
        end
    end

    find . -name '*.txt' -exec git add {} \;
    find . -name '*.json' -exec git add {} \;
    git commit -a -m sync

    git pull origin master
    git push origin master

    cd -
end

function supersync::uprecords
    set -l uprecords_dir ~/git/uprecords
    set -l uprecords_repo git@codeberg.org:snonux/uprecords.git

    if not test -d $uprecords_dir
        git clone $uprecords_repo $uprecords_dir
        cd $uprecords_dir
    else
        cd $uprecords_dir
        git pull
    end

    make update
    git commit -a -m Update
    git push
    cd -
end

function supersync::taskwarrior
    if test -f ~/scripts/taskwarriorfeeder.rb
        ruby ~/scripts/taskwarriorfeeder.rb
    else
        echo "No taskwarrior feeder script, skipping"
    end

    taskwarrior::export
    taskwarrior::export::gos
    taskwarrior::import
end

function supersync::gitsyncer
    if test -f ~/.gitsyncer_enable
        ~/go/bin/gitsyncer sync bidirectional
    end
end

function supersync
    supersync::worktime sync_quotes
    supersync::taskwarrior
    supersync::worktime no_sync_quotes
    supersync::uprecords
    supersync::gitsyncer

    if test -f ~/.gos_enable
        gos
    end

    date +%s >$SUPERSYNC_STAMP_FILE.tmp
    mv $SUPERSYNC_STAMP_FILE.tmp $SUPERSYNC_STAMP_FILE
end

function supersync::is_it_time_to_sync
    set -l max_age 86400
    set -l now (date +%s)
    if test -f $SUPERSYNC_STAMP_FILE
        set -l diff (math $now - (cat $SUPERSYNC_STAMP_FILE))
        if test $diff -lt $max_age
            return 0
        end
    end
    read -P "It's time to run supersync! Run it? (y/n) " answer; and test "$answer" = y; and supersync
end
