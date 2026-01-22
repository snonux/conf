<% our $plugin_dir = '/usr/local/libexec/nagios'; -%>
{
  "EmailTo": "paul",
  "EmailFrom": "gogios@mx.buetow.org",
  "CheckTimeoutS": 10,
  "CheckConcurrency": 3,
  "StateDir": "/var/run/gogios",
  "HTMLStatusFile": "/var/www/htdocs/buetow.org/self/gogios/index.html",
  "PrometheusHosts": ["r0.wg0:30090", "r1.wg0:30090", "r2.wg0:30090"],
  "PrometheusOnlyIfNotExists": "/tmp/f3s_taken_down",
  "Checks": {
    <% for my $host (qw(master standby)) { -%>
    <%   for my $proto (4, 6) { -%>
    "Check Ping<%= $proto %> <%= $host %>.buetow.org": {
      "Plugin": "<%= $plugin_dir %>/check_ping",
      "Args": ["-H", "<%= $host %>.buetow.org", "-<%= $proto %>", "-w", "100,10%", "-c", "200,15%"],
      "RandomSpread": 10,
      "Retries": 3,
      "RetryInterval": 3
    },
    <%   } -%>
    <% } -%>
    <% for my $host (qw(blowfish fishfinger)) { -%>
    <%   for my $proto (4, 6) { -%>
    "Check Ping<%= $proto %> <%= $host %>.wg0.wan.buetow.org": {
      "Plugin": "<%= $plugin_dir %>/check_ping",
      "Args": ["-H", "<%= $wg0_ips->{$host}->{$proto} %>", "-<%= $proto %>", "-w", "100,20%", "-c", "200,30%"],
      "RandomSpread": 10,
      "Retries": 5,
      "RetryInterval": 3
    },
    <%   } -%>
    <% } -%>
    <% for my $host (qw(f0 f1 f2 r0 r1 r2)) { -%>
    <%   for my $proto (4, 6) { -%>
    "Check Ping<%= $proto %> <%= $host %>.wg0.wan.buetow.org": {
      "Plugin": "<%= $plugin_dir %>/check_ping",
      "Args": ["-H", "<%= $wg0_ips->{$host}->{$proto} %>", "-<%= $proto %>", "-w", "100,20%", "-c", "200,30%"],
      "OnlyIfNotExists": "/tmp/f3s_taken_down",
      "RandomSpread": 10,
      "Retries": 5,
      "RetryInterval": 3
    },
    <%   } -%>
    <% } -%>
    <% for my $host (qw(fishfinger blowfish)) { -%>
    "Check DTail <%= $host %>.buetow.org": {
      "Plugin": "/usr/local/bin/dtailhealth",
      "RunInterval": 3600,
      "RandomSpread": 10,
      "Args": ["--server", "<%= $host %>.buetow.org:2222"],
      "DependsOn": ["Check Ping4 <%= $host %>.buetow.org", "Check Ping6 <%= $host %>.buetow.org"]
    },
    <% } -%>
    <% for my $host (qw(fishfinger blowfish)) { -%>
    <%   for my $proto (4, 6) { -%>
    "Check Ping<%= $proto %> <%= $host %>.buetow.org": {
      "Plugin": "<%= $plugin_dir %>/check_ping",
      "RandomSpread": 10,
      "Args": ["-H", "<%= $host %>.buetow.org", "-<%= $proto %>", "-w", "100,10%", "-c", "200,15%"],
      "Retries": 3,
      "RetryInterval": 3
    },
    <%   } -%>
    "Check TLS Certificate <%= $host %>.buetow.org": {
      "Plugin": "<%= $plugin_dir %>/check_http",
      "RandomSpread": 10,
      "RunInterval": 3600,
      "Args": ["--sni", "-H", "<%= $host %>.buetow.org", "-C", "20" ],
      "DependsOn": ["Check Ping4 <%= $host %>.buetow.org", "Check Ping6 <%= $host %>.buetow.org"]
    },
    <% } -%>
    <% for my $host (@$acme_hosts) {
         # Skip server hostnames - they have dedicated checks above without www/standby variants
         next if $host eq 'blowfish.buetow.org' or $host eq 'fishfinger.buetow.org'; -%>
    <%   for my $prefix ('', 'standby.', 'www.') { -%>
    <%     my $depends_on = $prefix eq 'standby.' ? 'standby.buetow.org' : 'master.buetow.org'; -%>
    "Check TLS Certificate <%= $prefix . $host %>": {
      "Plugin": "<%= $plugin_dir %>/check_http",
      "RandomSpread": 10,
      "RunInterval": 3600,
      "Args": ["--sni", "-H", "<%= $prefix . $host %>", "-C", "20" ],
      "DependsOn": ["Check Ping4 <%= $depends_on %>", "Check Ping6 <%= $depends_on %>"]
    },
    <%     for my $proto (4, 6) { -%>
    "Check HTTP IPv<%= $proto %> <%= $prefix . $host %>": {
      "Plugin": "<%= $plugin_dir %>/check_http",
      "RandomSpread": 10,
      "Args": ["<%= $prefix . $host %>", "-<%= $proto %>"],
      "DependsOn": ["Check Ping<%= $proto %> <%= $depends_on %>"]
    },
    <%     } -%>
    <%   } -%>
    <% } -%>
    <% for my $host (qw(fishfinger blowfish)) { -%>
    <%   for my $proto (4, 6) { -%>
    "Check Dig <%= $host %>.buetow.org IPv<%= $proto %>": {
      "Plugin": "<%= $plugin_dir %>/check_dig",
      "RandomSpread": 10,
      "Args": ["-H", "<%= $host %>.buetow.org", "-l", "buetow.org", "-<%= $proto %>"],
      "DependsOn": ["Check Ping<%= $proto %> <%= $host %>.buetow.org"]
    },
    "Check SMTP <%= $host %>.buetow.org IPv<%= $proto %>": {
      "Plugin": "<%= $plugin_dir %>/check_smtp",
      "RandomSpread": 10,
      "Args": ["-H", "<%= $host %>.buetow.org", "-<%= $proto %>"],
      "DependsOn": ["Check Ping<%= $proto %> <%= $host %>.buetow.org"]
    },
    "Check Gemini TCP <%= $host %>.buetow.org IPv<%= $proto %>": {
      "Plugin": "<%= $plugin_dir %>/check_tcp",
      "RandomSpread": 10,
      "Args": ["-H", "<%= $host %>.buetow.org", "-p", "1965", "-<%= $proto %>"],
      "DependsOn": ["Check Ping<%= $proto %> <%= $host %>.buetow.org"]
    },
    <%   } -%>
    <% } -%>
    "Check Users <%= $hostname %>": {
      "Plugin": "<%= $plugin_dir %>/check_users",
      "RandomSpread": 10,
      "RunInterval": 600,
      "Args": ["-w", "2", "-c", "3"]
    },
    "Check SWAP <%= $hostname %>": {
      "Plugin": "<%= $plugin_dir %>/check_swap",
      "RandomSpread": 10,
      "RunInterval": 300,
      "Args": ["-w", "95%", "-c", "90%"]
    },
    "Check Procs <%= $hostname %>": {
      "Plugin": "<%= $plugin_dir %>/check_procs",
      "RandomSpread": 10,
      "RunInterval": 300,
      "Args": ["-w", "100", "-c", "150"]
    },
    "Check Disk <%= $hostname %>": {
      "Plugin": "<%= $plugin_dir %>/check_disk",
      "RandomSpread": 10,
      "RunInterval": 300,
      "Args": ["-w", "30%", "-c", "10%"]
    },
    "Check Load <%= $hostname %>": {
      "Plugin": "<%= $plugin_dir %>/check_load",
      "RandomSpread": 10,
      "RunInterval": 300,
      "Args": ["-w", "2,1,1", "-c", "4,3,3"]
    }
  }
}
