# Build frontend
FROM node:20-alpine AS frontend-builder
WORKDIR /build
RUN apk add --no-cache git
RUN git clone https://github.com/Ludvigaman/MinVid.git .
WORKDIR /build/MinVid
RUN npm install && npm run build

# Build API
FROM mcr.microsoft.com/dotnet/sdk:9.0-alpine AS api-builder
WORKDIR /build
RUN apk add --no-cache git
RUN git clone https://github.com/Ludvigaman/MinVid.git .
WORKDIR /build/MinVid-API
RUN dotnet publish -c Release -o /app/publish

# Final image
FROM mcr.microsoft.com/dotnet/aspnet:9.0-alpine

ENV PUID=1000
ENV PGID=1000

RUN apk add --no-cache bash su-exec ffmpeg

WORKDIR /app

# Copy API
COPY --from=api-builder /app/publish .

# Copy frontend dist
COPY --from=frontend-builder /build/MinVid/dist ./wwwroot

EXPOSE 80

ENV ASPNETCORE_URLS=http://+:80

COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]
CMD ["dotnet", "MinVid.dll"]

HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget -qO- http://127.0.0.1:80/health || exit 0

VOLUME /app/data

LABEL maintainer="MinVid Contributors"
