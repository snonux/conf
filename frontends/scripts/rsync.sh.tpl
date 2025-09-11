#!/bin/sh

PATH=$PATH:/usr/local/bin

# Sync Joern's content over to Fishfinger!
if [ `hostname -s` = fishfinger ]; then
	rsync -av --delete rsync://blowfish.wg0.wan.buetow.org/joernshtdocs/ /var/www/htdocs/joern/
fi
