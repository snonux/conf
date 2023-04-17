{
  "EmailTo": "paul",
  "EmailFrom": "gogios@mx.buetow.org",
  "CheckTimeoutS": 10,
  "CheckConcurrency": 2,
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
    "hasn foo.zone HTTP IPv6": {
      "Plugin": "/usr/local/libexec/nagios/check_http",
      "Args": ["foo.zone", "-6"]
    }
  }
}
