set -gx WORKTIME_DIR ~/git/worktime

if test (uname) = Darwin -a ! -f ~/.wtloggedin
    echo "Warn: Not logged in, run wtlogin"
end

function worktime
    ruby $WORKTIME_DIR/worktime.rb $argv
end

function worktime::sync
    cd $WORKTIME_DIR
    git commit -a -m sync
    git pull
    git push
    cd -
end

function worktime::wisdom_reminder
    if test -f $WORKTIME_DIR/work-wisdoms.md
        sed -n '/^\* / { s/\* //; p; }' $WORKTIME_DIR/work-wisdoms.md | sort -R | head -n 1
    end
end

function worktime::report
    if test -f ~/.wtloggedin
        if test -f ~/.wtmaster
            worktime --report | tee $WORKTIME_DIR/report.txt
        else
            worktime --report
        end
        worktime::wisdom_reminder
    end
end

function worktime::add
    set -l seconds $argv[1]
    set -l what $argv[2]
    set -l descr $argv[3]
    set -l epoch (date +%s)

    if test -z "$what"
        set what work
    end

    if test -z "$descr"
        worktime --add $seconds --epoch $epoch --what $what
    else
        worktime --add $seconds --epoch $epoch --what $what --descr "$descr"
    end

    worktime::report
end

function worktime::log
    set -l seconds $argv[1]
    set -l what $argv[2]
    set -l epoch (date +%s)

    if test -z "$what"
        set what work
    end

    worktime --log --epoch $epoch --what $what
    worktime::report
end

function worktime::login
    set -l what $argv[1]
    if test -z "$what"
        set what work
    end
    touch ~/.wtloggedin
    worktime --login --what $what
    worktime::wisdom_reminder
end

function worktime::logout
    set -l what $argv[1]

    if test -z "$what"
        set what work
    end

    if test -f ~/.wtloggedin
        rm ~/.wtloggedin
    end

    worktime --logout --what $what
    worktime::report
end

function worktime::status
    worktime::report

    if test -f ~/.wtloggedin
        echo "You are logged in"
        set -l num_worklog (ls $WORKTIME_DIR | grep wl- | wc -l)
        if test $num_worklog -gt 0
            echo "$num_worklog entries in the worklog in $WORKTIME_DIR/wl-*"
        end
    else
        echo "You are not logged in"
    end
end

abbr -a cdworktime "cd $WORKTIME_DIR"
abbr -a wt worktime
abbr -a wtedit 'worktime --edit'
abbr -a wtreport 'worktime --report'
abbr -a wtadd 'worktime::add'
abbr -a wtlog 'worktime::log'
abbr -a wtlogin 'worktime::login'
abbr -a wtlogout 'worktime::logout'
abbr -a wtstatus 'worktime::status'
abbr -a wtsync 'worktime::sync'
abbr -a wtf 'worktime --report'
abbr -a random_exercise "sort -R $WORKTIME_DIR/exercises.md | head -n 1"
abbr -a random_exercises "sort -R $WORKTIME_DIR/exercises.md | head -n 10"
abbr -a wl 'task add +work'
abbr -a ql 'task add +personal'
abbr -a pl 'task add +personal'
