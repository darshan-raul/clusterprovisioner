# Strata orchestrator — Phase 1 build
#
# Build context must be the repo root so we can copy the shared
# Go module:
#   docker build -f backend/services/orchestrator/Dockerfile -t strata-orchestrator:dev .

FROM golang:1.25-alpine AS build

WORKDIR /src
COPY backend/services/orchestrator/go.mod backend/services/orchestrator/go.sum ./backend/services/orchestrator/
COPY backend/services/shared/ ./backend/services/shared/
WORKDIR /src/backend/services/orchestrator
RUN go mod download

COPY backend/services/orchestrator/ ./
RUN CGO_ENABLED=0 go build -o /out/orchestrator ./cmd/orchestrator

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/orchestrator /orchestrator
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/orchestrator"]