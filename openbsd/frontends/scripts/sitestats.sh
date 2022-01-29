#!/bin/sh

zgrep -h . /var/www/logs/access.log* | perl -l -n -e '@s=split / +/; next if @s!=11; $s[4]=~s|\[(\d\d)/(...)/(\d{4}):(.*)|$1 $2 $3 $4|; print join " ",@s[0,1,4,7];'
