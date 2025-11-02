function fullest_h
    df -h | sort -n -k 5
end

function fullest_i
    df -i | sort -n -k 5
end

function usortn
    sort | uniq -c | sort -n
end

function asum
    awk '{ sum += $1 } END { print sum }'
end

function stop
    set -l service $argv[1]
    sudo service $service stop $argv
end

function start
    set -l service $argv[1]
    sudo service $service start $argv
end

function restart
    set -l service $argv[1]
    sudo service $service restart $argv
end

function statuss
    set -l service $argv[1]
    sudo service $service status $argv
end

function loop
    set -l sleep 10
    if set -q SLEEP
        set sleep $SLEEP
    end
    echo "sleep is $sleep" 1>&2
    while true
        $argv
        sleep $sleep
    end
end

function f
    find . -iname "*$argv*"
end

function random
    set -l upto $argv[1]
    set -l random (math $RANDOM % $upto)
    echo "Sleeping $random seconds"
    sleep $random
end

function dedup
    set -l file $argv[1]
    if test -z $file
        awk '{ if (line[$0] != 42) { print $0 }; line[$0] = 42; }'
    else
        awk '{ if (line[$0] != 42) { print $0 }; line[$0] = 42; }' $file | sudo tee $file.dedup >/dev/null
        if test ! -f $file.dedupbak
            sudo mv $file $file.dedupbak
        end
        sudo mv $file.dedup $file
        wc -l $file $file.dedupbak
        sudo gzip --best $file.dedupbak &
    end
end

function dedup_no_bak
    set -l file $argv[1]
    if test -z $file
        awk '{ if (line[$0] != 42) { print $0 }; line[$0] = 42; }'
    else
        awk '{ if (line[$0] != 42) { print $0 }; line[$0] = 42; }' $file | sudo tee $file.dedup >/dev/null
        if test ! -f $file.dedupbak
            sudo mv $file $file.dedupbak
        end
        sudo mv $file.dedup $file
        wc -l $file $file.dedupbak
        sudo rm -v $file.dedupbak &
    end
end

function drop_caches
    echo 3 | sudo tee /proc/sys/vm/drop_caches
end

function ssl_connect
    set -l address $argv[1]
    openssl s_client -connect $address
end

function ssl_dates
    ssl_connect $argv | openssl x509 -noout -dates
end

function lastu
    last | grep -E -v '(root|cron|nagios)'
end

function lastl
    lastu | less
end

abbr wetter 'curl http://wttr.in'

abbr tf terraform

function touchtype
    tt --noskip --noreport --showwpm --bold --theme (tt -list themes | sort -R | head -n1) $argv
end

function touchtype::quote
    while true
        touchtype -quotes en
        sleep 0.2
    end
end

function checkcert
    set host $argv[1]
    set port $argv[2]
    openssl s_client \
        -connect $host:$port \
        -servername $host \
        -showcerts </dev/null 2>/dev/null | openssl x509 -noout -dates -subjectend
end

abbr typing 'touchtype::quote'

function sway_config_view
    less /etc/sway/config
end

function ssh::force
    set -l server $argv[1]
    ssh-keygen -R $server
    ssh -A $server
end

if test -f ~/git/geheim/geheim.rb
    function geheim
        ruby ~/git/geheim/geheim.rb $argv
    end
end
