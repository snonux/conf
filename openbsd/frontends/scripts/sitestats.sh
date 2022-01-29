#!/bin/sh

STATSFILE=/tmp/sitestats.csv
BOTSFILE=/tmp/sitebots.txt
TOP=20

header () {
	echo "proto,host,ip,day,month,time,path"
}

indent () {
	sed 's/^/    /'
}

http_stats () {
	zgrep -h . /var/www/logs/access.log* |
	perl -l -n -e 's/\.html/.suffix/; @s=split / +/; next if @s!=11;
	$s[4]=~s|\[(\d\d)/(...)/\d{4}:(.*)|$1,$2,$3|;
	print "http,".join ",",@s[0,1,4,7];' 
}

gemini_stats () {
	zgrep -h . /var/log/daemon* |
	perl -l -n -e '@s=split / +/; @v=@s and next if $s[4] eq "vger:";
	next if !/relayd.*gemini/;
	($path) = $v[-1] =~ m|gemini://.*?(/.*)|;
	next if $path eq "";
	$path =~ s/\.gmi/.suffix/;
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
	perl -l -n -e '$s{$_}++ if /\.suffix/;
	$s{$_}+=1000 if /(?:\.php|\.env|robots\.txt|\/wp|\/wordpress\/|\/\.git\/|HNAP)/;
	END { while (($k,$v) = each %s) { print $k =~ /.*?,(.*?),/ if $v > 1 } }' |
	sort -u > $BOTSFILE
	grep -F -v -f $BOTSFILE $STATSFILE > $STATSFILE.clean
	mv $STATSFILE.clean $STATSFILE
}

stats () {
	sed 1d $STATSFILE
}

top_n () {
	fields="$1"
	descr="$2"

	echo "Top $TOP `head -n 1 $STATSFILE | cut -d, -f"$fields"`$descr:"
 	cut -d, -f"$fields" | sort | uniq -c | sort -nr | head -n $TOP | indent
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

main () {
	parse_logs
	filter 
	stats | grep -F .suffix | top_n '1,2,4,5,7' ' (Only .suffix)'
	stats | grep -F atom.xml | top_n '1,2,4,5,7' ' (Only atom.xml)'
	stats | top_n 1
	stats | top_n 2
	stats | top_n '4,5'
	stats | top_n 7
	stats | top_n '1,7'
	stats | top_n '1,2,7'
	ip_stats
}

main
