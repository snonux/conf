{
  "EmailTo": "paul",
  "EmailFrom": "gogios@mx.buetow.org",
  "CheckTimeoutS": 4,
  "CheckConcurrency": 10,
  "StateDir": "/var/run/gogios",
  "Checks": {
    <% for my $host (@$acme_hosts) { -%>
      <% for my $prefix ('', 'www.') { -%>
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
    "Check ICMP localhost": {
      "Plugin": "/usr/local/libexec/nagios/check_ping",
      "Args": ["-H", "localhost", "-w", "50,10%", "-c", "100,15%"]
    }
  }
}
