0 7 * * * <%= $gogios_path %> -renotify >/dev/null
*/5 8-22 * * * -s <%= $gogios_path %> >/dev/null
0 3 * * 0 <%= $gogios_path %> -force >/dev/null
