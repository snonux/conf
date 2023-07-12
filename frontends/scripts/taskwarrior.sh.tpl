PATH=$PATH:/usr/local/bin

echo "Any tasks due before the next 14 days?"
# Using git user, as ssh keys are already there to sync the task db!
su - git -c '/usr/local/bin/task rc:/etc/taskrc due.before:14day minimal 2>/dev/null'
