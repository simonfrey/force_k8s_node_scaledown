#
# Builder
#

FROM golang:1.22-alpine AS builder

# Create a workspace for the app
WORKDIR /app

# Copy over the files
COPY . ./

# Build
RUN go build -o scaledown

#
# Runner
#

FROM alpine AS runner

WORKDIR /


# Copy from builder the final binary
COPY --from=builder /app/scaledown /scaledown

ENV MIN_NODE_AGE_MIN=5
ENV SLEEP_S=20
ENV IGNORE_NAMESPACES="gke-managed-cim,gmp-system,kube-system"

ENV HEALTH_PORT=9200
EXPOSE 9200

ENTRYPOINT ["/scaledown"]