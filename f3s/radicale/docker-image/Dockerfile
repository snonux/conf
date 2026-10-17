FROM alpine:3.24.2
RUN adduser -D -u 1000 radicale \
    && apk add --no-cache python3 py3-pip \
    && python3 -m venv /opt/venv \
    && /opt/venv/bin/pip install --no-cache-dir radicale==3.8.3 bcrypt \
    && mkdir /auth /collections /etc/radicale \
    && chown radicale /auth /collections
COPY config /etc/radicale/config
ENV PATH="/opt/venv/bin:$PATH"
EXPOSE 8080
USER radicale
CMD ["python3", "-m", "radicale", "--debug"]
