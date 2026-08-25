# Strata MCP k8s server — Phase 1 build
FROM python:3.12-slim AS base

ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    PIP_NO_CACHE_DIR=1 \
    PIP_DISABLE_PIP_VERSION_CHECK=1

WORKDIR /app

# Install uv for fast dependency resolution
COPY --from=ghcr.io/astral-sh/uv:latest /uv /usr/local/bin/uv

# Copy project metadata first for layer caching
COPY pyproject.toml ./
COPY strata_mcp_k8s ./strata_mcp_k8s

# Install dependencies (no dev extras in production image)
RUN uv sync --no-dev

ENV PATH="/app/.venv/bin:$PATH"

EXPOSE 8000

# Streamable-HTTP transport. The orchestrator talks to us at /mcp via
# Envoy Gateway. The MCP host (port 8000) is internal-only.
ENTRYPOINT ["python", "-m", "strata_mcp_k8s.server"]