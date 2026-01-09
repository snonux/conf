FROM alpine:3.19

# Install OpenSSH server and git
RUN apk add --no-cache openssh git

# Create git user with UID 1000 and set git-shell as login shell
# This restricts the user to git operations only
RUN adduser -D -u 1000 -s /usr/bin/git-shell git && \
    mkdir -p /home/git/.ssh /repos && \
    chown -R git:git /home/git /repos

# Copy sshd configuration
COPY sshd_config /etc/ssh/sshd_config

# Create entrypoint script to generate host keys at runtime
RUN echo '#!/bin/sh' > /entrypoint.sh && \
    echo 'if [ ! -f /etc/ssh/ssh_host_ed25519_key ]; then' >> /entrypoint.sh && \
    echo '  ssh-keygen -A' >> /entrypoint.sh && \
    echo 'fi' >> /entrypoint.sh && \
    echo 'exec /usr/sbin/sshd -D -e' >> /entrypoint.sh && \
    chmod +x /entrypoint.sh

# Expose SSH port
EXPOSE 22

# Run entrypoint script
CMD ["/entrypoint.sh"]
