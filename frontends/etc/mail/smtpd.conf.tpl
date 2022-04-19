# This is the smtpd server system-wide configuration file.
# See smtpd.conf(5) for more information.

# I used https://www.checktls.com/TestReceiver for testing.

pki "buetow_org_tls" cert "/etc/ssl/buetow.org.fullchain.pem"
pki "buetow_org_tls" key "/etc/ssl/private/buetow.org.key"

table aliases file:/etc/mail/aliases
table virtualdomains file:/etc/mail/virtualdomains
table virtualusers file:/etc/mail/virtualusers

listen on socket
listen on all tls pki "buetow_org_tls" hostname "<%= $mail_hostname->($hostname) %>"
#listen on all

action localmail mbox alias <aliases>
action receive mbox virtual <virtualusers>
action outbound relay

match from any for domain <virtualdomains> action receive
match from local for local action localmail
match from local for any action outbound
