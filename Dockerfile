FROM alpine:3.21

# Install dependencies
RUN apk add --no-cache \
    bash \
    curl \
    ca-certificates \
    aws-cli \
    postgresql-client

# Install dbmate
ARG DBMATE_VERSION=2.29.3
RUN curl -fsSL -o /usr/local/bin/dbmate \
    https://github.com/amacneil/dbmate/releases/download/v${DBMATE_VERSION}/dbmate-linux-amd64 \
    && chmod +x /usr/local/bin/dbmate

# Create working directory
WORKDIR /migrations

# Copy entrypoint script
COPY entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod +x /usr/local/bin/entrypoint.sh

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
