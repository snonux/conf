<%
  our $plugin_dir = '/usr/local/libexec/nagios';
-%>
{
  "EmailTo": "paul",
  "EmailFrom": "gogios@mx.buetow.org",
  "CheckTimeoutS": 10,
  "CheckConcurrency": 3,
  "StateDir": "/var/run/gogios",
  "Checks": {
    <% for my $host (qw(fishfinger blowfish)) { %>
    "Check DTail <%= $host %>.buetow.org": {
      "Plugin": "/usr/local/bin/dtailhealth",
      "Args": ["--server", "<%= $host %>.buetow.org:2222"],
      "DependsOn": ["Check Ping4 <%= $host %>.buetow.org", "Check Ping6 <%= $host %>.buetow.org"]
    },
    <% } -%>
    <% for my $host (qw(fishfinger blowfish)) { %>
    "Check Ping4 <%= $host %>.buetow.org": {
      "Plugin": "<%= $plugin_dir %>/check_ping",
      "Args": ["-H", "<%= $host %>.buetow.org", "-4", "-w", "100,10%", "-c", "200,15%"],
      "Retries": 3,
      "RetryInterval": 3
    },
    "Check Ping6 <%= $host %>.buetow.org": {
      "Plugin": "<%= $plugin_dir %>/check_ping",
      "Args": ["-H", "<%= $host %>.buetow.org", "-6", "-w", "100,10%", "-c", "200,15%"],
      "Retries": 3,
      "RetryInterval": 3
    },
    <% } -%>
    <% for my $host (@$acme_hosts) { -%>
      <% for my $prefix ('', 'mirror.', 'www.') { -%>
    "Check TLS Certificate <%= $prefix . $host %>": {
      "Plugin": "<%= $plugin_dir %>/check_http",
      "Args": ["--sni", "-H", "<%= $prefix . $host %>", "-C", "20" ],
      "DependsOn": ["Check Ping4 <%= $prefix eq '' ? 'blowfish.buetow.org' : 'fishfinger.buetow.org' %>"]
    },
        <% for my $proto (4, 6) { -%>
    "Check HTTP IPv<%= $proto %> <%= $prefix . $host %>": {
      "Plugin": "<%= $plugin_dir %>/check_http",
      "Args": ["<%= $prefix . $host %>", "-<%= $proto %>"],
      "DependsOn": ["Check Ping<%= $proto %> <%= $prefix eq '' ? 'blowfish.buetow.org' : 'fishfinger.buetow.org' %>"]
    },
        <% } -%>
      <% } -%>
    <% } -%>
    <% for my $host (qw(fishfinger blowfish)) { %>
      <% for my $proto (4, 6) { -%>
    "Check Dig <%= $host %>.buetow.org IPv<%= $proto %>": {
      "Plugin": "<%= $plugin_dir %>/check_dig",
      "Args": ["-H", "<%= $host %>.buetow.org", "-l", "buetow.org", "-<%= $proto %>"],
      "DependsOn": ["Check Ping<%= $proto %> <%= $host %>.buetow.org"]
    },
    "Check SMTP <%= $host %>.buetow.org IPv<%= $proto %>": {
      "Plugin": "<%= $plugin_dir %>/check_smtp",
      "Args": ["-H", "<%= $host %>.buetow.org", "-<%= $proto %>"],
      "DependsOn": ["Check Ping<%= $proto %> <%= $host %>.buetow.org"]
    },
    "Check Gemini TCP <%= $host %>.buetow.org IPv<%= $proto %>": {
      "Plugin": "<%= $plugin_dir %>/check_tcp",
      "Args": ["-H", "<%= $host %>.buetow.org", "-p", "1965", "-<%= $proto %>"],
      "DependsOn": ["Check Ping<%= $proto %> <%= $host %>.buetow.org"]
    },
      <% } -%>
    <% } -%>
    "Check Users <%= $hostname %>": {
      "Plugin": "<%= $plugin_dir %>/check_users",
      "Args": ["-w", "2", "-c", "3"]
    },
    "Check SWAP <%= $hostname %>": {
      "Plugin": "<%= $plugin_dir %>/check_swap",
      "Args": ["-w", "95%", "-c", "90%"]
    },
    "Check Procs <%= $hostname %>": {
      "Plugin": "<%= $plugin_dir %>/check_procs",
      "Args": ["-w", "80", "-c", "100"]
    },
    "Check Disk <%= $hostname %>": {
      "Plugin": "<%= $plugin_dir %>/check_disk",
      "Args": ["-w", "30%", "-c", "10%"]
    },
    "Check Load <%= $hostname %>": {
      "Plugin": "<%= $plugin_dir %>/check_load",
      "Args": ["-w", "2,1,1", "-c", "4,3,3"]
    }
  }
}
