# This is the smtpd server system-wide configuration file.
# See smtpd.conf(5) for more information.

# I used https://www.checktls.com/TestReceiver for testing.

pki "buetow_org_tls" cert "/etc/ssl/<%= "$hostname.$domain" %>.fullchain.pem"
pki "buetow_org_tls" key "/etc/ssl/private/<%= "$hostname.$domain" %>.key"

table aliases file:/etc/mail/aliases
table virtualdomains file:/etc/mail/virtualdomains
table virtualusers file:/etc/mail/virtualusers

# Reject lists for blocking unwanted senders/domains/recipients
table reject-senders file:/etc/mail/reject-senders
table reject-domains file:/etc/mail/reject-domains
table reject-recipients file:/etc/mail/reject-recipients

listen on socket
listen on all tls pki "buetow_org_tls" hostname "<%= "$hostname.$domain" %>"
#listen on all

action localmail mbox alias <aliases>
action receive mbox virtual <virtualusers>
action outbound relay

# Reject rules (processed before accept rules)
# reject-senders: full addresses, reject-domains: patterns like *@domain.com
match from any mail-from <reject-senders> reject
match from any mail-from <reject-domains> reject
match from any for rcpt-to <reject-recipients> reject

match from any for domain <virtualdomains> action receive
match from local for local action localmail
match from local for any action outbound
