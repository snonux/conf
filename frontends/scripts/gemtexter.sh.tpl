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
    if [ ! -d $basename ]; then
        git clone $repo -b $branch --single-branch $basename
    else
        cd $basename
        git pull
    fi
}

for site in foo.zone snonux.land paul.buetow.org; do
    ensure_site \
        /var/gemini/$site \
        https://codeberg.org/snonux/$site \
        content-gemtext
    ensure_site \
        /var/www/htdocs/gemtexter/$site \
        https://codeberg.org/snonux/$site \
        content-html
done

