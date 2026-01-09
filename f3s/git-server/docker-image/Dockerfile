FROM alpine:3.19

# Install OpenSSH server and git
RUN apk add --no-cache openssh git

# Create git user with UID 1000 and set git-shell as login shell
# This restricts the user to git operations only
RUN adduser -D -u 1000 -s /usr/bin/git-shell git && \
    mkdir -p /home/git/.ssh /repos && \
    chown -R git:git /home/git /repos

# Generate SSH host keys
# These will be regenerated if not persisted via volume mount
RUN ssh-keygen -A

# Copy sshd configuration
COPY sshd_config /etc/ssh/sshd_config

# Expose SSH port
EXPOSE 22

# Run SSH daemon in foreground with error logging to stderr
CMD ["/usr/sbin/sshd", "-D", "-e"]
