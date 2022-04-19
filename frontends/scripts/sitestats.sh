#!/bin/sh

# This is a quick and dirty script to get some stats for my site.
# Yes, this could be programmed cleaner, but I wanted to do something quick
# and dirty and this also with only tools available on the OpenBSD base install.

STATSFILE=/tmp/sitestats.csv
BOTSFILE=/tmp/sitebots.txt
TOP=20

header () {
	echo "proto,host,ip,day,month,time,path"
}

http_stats () {
	zgrep -h . /var/www/logs/access.log* |
	perl -l -n -e 's/\.html/.suffix/; @s=split / +/; next if @s!=11;
	$s[4]=~s|\[(\d\d)/(...)/\d{4}:(.*)|$1,$2,$3|; print "http,".join ",",@s[0,1,4,7];' 
}

gemini_stats () {
	zgrep -h . /var/log/daemon* |
	perl -l -n -e '@s=split / +/; @v=@s and next if $s[4] eq "vger:";
	next if !/relayd.*gemini/; ($path) = $v[-1] =~ m|gemini://.*?(/.*)|;
	next if $path eq ""; $path =~ s/\.gmi/.suffix/;
	print "gemini,".(split("/", $v[6]))[2].",$s[12],$s[1],$s[0],$s[2],$path"'
}

parse_logs () {
	header > $STATSFILE.tmp
	http_stats >> $STATSFILE.tmp
	gemini_stats >> $STATSFILE.tmp
	mv $STATSFILE.tmp $STATSFILE
}

filter () {
	# Collect some 'you are a bot' scores.
	# 1. You visit 2 sites within one single second
	# 2. You try to call an odd file or path
	cut -d, -f2,3,6,7 $STATSFILE |
	perl -l -n -e '($k)=m/(.*?,.*?,.*?),/; $s{$k}++ if /\.suffix/;
	$s{$k}+=1000 if /(?:target\.suffix|\.php|wordpress|\/wp|\.asp|\.\.|robots\.txt|\.env|\?|\+|%|\*|HNAP1|\/admin\/|\.git\/|microsoft\.exchange|\.lua|\/owa\/)/;
	END { while (($k,$v) = each %s) { print $k =~ /.*?,(.*?),/ if $v > 1 } }' |
	sort -u > $BOTSFILE

	# Filte out all bot IPs, also only filter out all known file "types".
	grep -F -v -f $BOTSFILE $STATSFILE > $STATSFILE.clean1
	grep -v -E '(proto,host|\.suffix|atom\.xml|\.gif|\.png|\.jpg|,,)' $STATSFILE.clean1 > $STATSFILE.dirt
	#grep -E '(proto,host|\.suffix|atom\.xml|\.gif|\.png|\.jpg)' $STATSFILE.clean1 > $STATSFILE.clean2
	mv $STATSFILE.clean1 $STATSFILE
}

stats () {
	sed 1d $STATSFILE
}

top_n () {
	fields="$1"
	descr="$2"

	echo "Top $TOP `head -n 1 $STATSFILE | cut -d, -f"$fields"`$descr:"
 	cut -d, -f"$fields" | sort | uniq -c | sort -nr | head -n $TOP | sed 's/^/    /'
	echo
}

ip_stats () {
	for proto in http gemini; do 
		echo -n "Unique $proto IPv4 IPs:\t"
 		stats | grep "^$proto," | cut -d, -f3 | grep -F -v : | sort -u | wc -l
		echo -n "Unique $proto IPv6 IPs:\t" 
		stats | grep "^$proto," | cut -d, -f3 | grep -F : | sort -u | wc -l
	done
}

ip_daily_stats () {
	echo "Unique IPs by day"
	for back in $(jot 14); do
		now=$(date +%s)
		date=$(date -r $(echo "$now - 86400 * $back" | bc) +%d,%b)
		echo -n "\t $date:"
		stats | grep $date | cut -d, -f3 | sort -u | wc -l		
	done
}

ip_daily_subscribers () {
	echo "Unique atom.xml subscribers by day"
	for back in $(jot 14); do
		now=$(date +%s)
		date=$(date -r $(echo "$now - 86400 * $back" | bc) +%d,%b)
		echo -n "\t $date:"
		stats | grep $date | grep atom.xml | cut -d, -f3 | sort -u | wc -l		
	done
}

main () {
	date
	echo
	parse_logs
	filter 
	stats | grep -F .suffix | top_n '1,2,4,5,7' ' (Only content)'
	stats | top_n 2
	stats | top_n '4,5'
	stats | top_n 7
	stats | grep -F .suffix | top_n 7 ' (Only content)'
	stats | top_n '1,2,7'
	ip_stats
	ip_daily_stats
	ip_daily_subscribers
}

main | sed 's/\.suffix//'
