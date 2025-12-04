set -gx TMPUTILS_DIR ~/data/tmp
set -gx TMPUTILS_TMPFILE ~/.tmpfile

function tmpdir
    set -l name $argv[1]
    set -l dir "$TMPUTILS_DIR/$name"
    if not test -d $dir
        mkdir -p $dir
    end
    cd $dir
end

function tmpls
    if not test -d $TMPUTILS_DIR
        return
    end
    ls $TMPUTILS_DIR
end

function tmptee
    set -l name $argv[1]
    if test -z "$name"
        set name (date +%s)
    else
        set -e argv[1]
    end
    set -l file "$TMPUTILS_DIR/$name"
    if not test -d $TMPUTILS_DIR
        mkdir -p $TMPUTILS_DIR
    end
    tee $argv $file
    echo $file >$TMPUTILS_TMPFILE
end

function tmpcat
    set -l name $argv[1]
    if test -z "$name"
        cat (tmpfile)
        return
    end
    cat "$TMPUTILS_DIR/$name"
end

function tmpedit
    set -l name $argv[1]
    if test -z "$name"
        $EDITOR (tmpfile)
        return
    end
    $EDITOR "$TMPUTILS_DIR/$name"
end

function tmpgrep
    set -l name $argv[1]
    set -e argv[1]
    tmcpat $name | grep $argv
end

function tmpfile
    cat $TMPUTILS_TMPFILE
end

abbr -a cdtmp "cd $TMPUTILS_DIR"
