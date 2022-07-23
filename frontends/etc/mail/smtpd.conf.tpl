# This is the smtpd server system-wide configuration file.
# See smtpd.conf(5) for more information.

# I used https://www.checktls.com/TestReceiver for testing.
#
<%
  our $primary = $is_primary->($vio0_ip);
  our $prefix = $primary ? '' : 'www.';
%>

pki "buetow_org_tls" cert "/etc/ssl/<%= $prefix %>buetow.org.fullchain.pem"
pki "buetow_org_tls" key "/etc/ssl/private/<%= $prefix %>buetow.org.key"

table aliases file:/etc/mail/aliases
table virtualdomains file:/etc/mail/virtualdomains
table virtualusers file:/etc/mail/virtualusers

listen on socket
listen on all tls pki "buetow_org_tls" hostname "<%= $prefix %>buetow.org"
#listen on all

action localmail mbox alias <aliases>
action receive mbox virtual <virtualusers>
action outbound relay

match from any for domain <virtualdomains> action receive
match from local for local action localmail
match from local for any action outbound
