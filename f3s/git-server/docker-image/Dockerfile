FROM alpine:3.19

# Install OpenSSH server and git
RUN apk add --no-cache openssh git

# Create git user with UID 1001, GID 33 (www-data) and set git-shell as login shell
# UID 1001 and GID 33 match the NFS file ownership
# This restricts the user to git operations only
# GID 33 is the existing www-data group in Alpine
RUN adduser -D -u 1001 -G www-data -s /usr/bin/git-shell git && \
    mkdir -p /home/git/.ssh /repos && \
    chown -R git:www-data /home/git /repos && \
    echo "/usr/bin/git-shell" >> /etc/shells && \
    passwd -u git

# Copy sshd configuration to /tmp (will be copied to /etc/ssh by entrypoint)
COPY sshd_config /tmp/sshd_config

# Create entrypoint script to setup SSH and run sshd
RUN echo '#!/bin/sh' > /entrypoint.sh && \
    echo '# Copy sshd_config if not exists' >> /entrypoint.sh && \
    echo 'if [ ! -f /etc/ssh/sshd_config ]; then' >> /entrypoint.sh && \
    echo '  cp /tmp/sshd_config /etc/ssh/sshd_config' >> /entrypoint.sh && \
    echo 'fi' >> /entrypoint.sh && \
    echo '# Generate host keys if not exists' >> /entrypoint.sh && \
    echo 'if [ ! -f /etc/ssh/ssh_host_ed25519_key ]; then' >> /entrypoint.sh && \
    echo '  ssh-keygen -A' >> /entrypoint.sh && \
    echo 'fi' >> /entrypoint.sh && \
    echo 'exec /usr/sbin/sshd -D -e' >> /entrypoint.sh && \
    chmod +x /entrypoint.sh

# Expose SSH port
EXPOSE 22

# Run entrypoint script
CMD ["/entrypoint.sh"]
