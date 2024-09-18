#!/bin/sh

PATH=$PATH:/usr/local/bin

function ensure_site {
    dir=$1
    repo=$2
    branch=$3

    basename=$(basename $dir)
    parent=$(dirname $dir)

    if [ ! -d $parent ]; then
        mkdir -p $parent
    fi

    cd $parent
    if [ ! -e www.$basename ]; then
        ln -s $basename www.$basename
    fi

    if [ ! -e standby.$basename ]; then
        ln -s $basename standby.$basename
    fi

    if [ ! -d $basename ]; then
        git clone $repo -b $branch --single-branch $basename
    else
        cd $basename
        git pull
    fi
}

function ensure_links {
    dir=$1
    target=$2

    basename=$(basename $dir)
    parent=$(dirname $dir)

    cd $parent

    if [ ! -e $target ]; then
        ln -s $basename $target
    fi

    if [ ! -e www.$target ]; then
        ln -s $basename www.$target
    fi

    if [ ! -e standby.$target ]; then
        ln -s $basename standby.$target
    fi
}

for site in foo.zone; do
    ensure_site \
        /var/gemini/$site \
        https://codeberg.org/snonux/$site \
        content-gemtext
    ensure_site \
        /var/www/htdocs/gemtexter/$site \
        https://codeberg.org/snonux/$site \
        content-html
done
