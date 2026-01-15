0 7 * * * <%= $gogios_path %> -renotify >/dev/null 2>&1
*/5 8-22 * * * <%= $gogios_path %> >/dev/null 2>&1
0 3 * * 0 <%= $gogios_path %> -force >/dev/null 2>&1
