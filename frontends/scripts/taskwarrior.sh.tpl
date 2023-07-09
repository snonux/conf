PATH=$PATH:/usr/local/bin

echo "Any tasks due before the next 14 days?"
su - git -c '/usr/local/bin/task rc:/etc/taskrc due.before:14day long 2>/dev/null'
