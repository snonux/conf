{
  "EmailTo": "paul",
  "EmailFrom": "gogios@mx.buetow.org",
  "CheckTimeoutS": 10,
  "CheckConcurrency": 2,
  "StateDir": "/var/run/gogios",
  "Checks": {
    <% for my $host (@$acme_hosts) { -%>
      <% for my $prefix ('', 'www.') { -%>
    "<%= $prefix . $host %> TLS Certificate": {
      "Plugin": "/usr/local/libexec/nagios/check_http",
      "Args": ["--sni", "-H", "<%= $prefix . $host %>", "-C", "30" ]
    },
    "<%= $prefix . $host %> HTTP IPv4": {
      "Plugin": "/usr/local/libexec/nagios/check_http",
      "Args": ["<%= $prefix . $host %>", "-4"]
    },
    "<%= $prefix . $host %> HTTP IPv6": {
      "Plugin": "/usr/local/libexec/nagios/check_http",
      "Args": ["<%= $prefix . $host %>", "-6"]
    },
      <% } -%>
    <% } -%>
    <% for my $host (qw(cloud anki wallabag)) { -%>
    "<%= $host %>.buetow.org TLS Certificate": {
      "Plugin": "/usr/local/libexec/nagios/check_http",
      "Args": ["--sni", "-H", "<%= $host %>.buetow.org", "-C", "30" ]
    },
    "<%= $host %>.buetow.org HTTP IPv4": {
      "Plugin": "/usr/local/libexec/nagios/check_http",
      "Args": ["<%= $host %>.buetow.org", "-4"]
    },
    "<%= $host %>.buetow.org HTTP IPv6": {
      "Plugin": "/usr/local/libexec/nagios/check_http",
      "Args": ["<%= $host %>.buetow.org", "-6"]
    },
    <% } -%>
    <% for my $host (qw(fishfinger blowfish vulcan babylon)) { %>
    "Check ICMP4 <%= $host %>.buetow.org": {
      "Plugin": "/usr/local/libexec/nagios/check_ping",
      "Args": ["-H", "<%= $host %>.buetow.org", "-4", "-w", "50,10%", "-c", "100,15%"]
    },
    "Check ICMP6 <%= $host %>.buetow.org": {
      "Plugin": "/usr/local/libexec/nagios/check_ping",
      "Args": ["-H", "<%= $host %>.buetow.org", "-6", "-w", "50,10%", "-c", "100,15%"]
    },
    <% } -%>
    <% for my $host (qw(fishfinger blowfish)) { %>
      <% for my $proto (4, 6) { -%>
    "Check Dig <%= $host %>.buetow.org IPv<%= $proto %>": {
      "Plugin": "/usr/local/libexec/nagios/check_dig",
      "Args": ["-H", "<%= $host %>.buetow.org", "-l", "buetow.org", "-<%= $proto %>"]
    },
    "Check SMTP <%= $host %>.buetow.org IPv<%= $proto %>": {
      "Plugin": "/usr/local/libexec/nagios/check_smtp",
      "Args": ["-H", "<%= $host %>.buetow.org", "-<%= $proto %>"]
    },
    "Check Gemini TCP <%= $host %>.buetow.org IPv<%= $proto %>": {
      "Plugin": "/usr/local/libexec/nagios/check_tcp",
      "Args": ["-H", "<%= $host %>.buetow.org", "-p", "1965", "-<%= $proto %>"]
    },
      <% } -%>
    <% } -%>
    "Check Users <%= $hostname %>": {
      "Plugin": "/usr/local/libexec/nagios/check_users",
      "Args": ["-w", "2", "-c", "3"]
    },
    "Check SWAP <%= $hostname %>": {
      "Plugin": "/usr/local/libexec/nagios/check_swap",
      "Args": ["-w", "99%", "-c", "95%"]
    },
    "Check Procs <%= $hostname %>": {
      "Plugin": "/usr/local/libexec/nagios/check_procs",
      "Args": ["-w", "80", "-c", "100"]
    },
    "Check Disk <%= $hostname %>": {
      "Plugin": "/usr/local/libexec/nagios/check_disk",
      "Args": ["-w", "30%", "-c", "10%"]
    },
    "Check Load <%= $hostname %>": {
      "Plugin": "/usr/local/libexec/nagios/check_load",
      "Args": ["-w", "2,1,1", "-c", "4,3,3"]
    }
  }
}
