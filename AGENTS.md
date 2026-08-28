# Repository Guidelines

## Workspace Layout

This repository is a multi-module Go monorepo under `github.com/mulan-go/mulan`, managed locally by the root `go.work`. Each directory publishes independently with tags in the form `<module>/vX.Y.Z`.

- `app/`: application lifecycle and CLI framework.
- `web/`, `rdb/`, `req/`, `rsp/`: HTTP, Redis, request binding, and response packages.
- `auth/`, `captcha/`, `dbx/`, `heartbeat/`, `log/`, `passwd/`, `vault/`: framework extensions.

## Build and Verification

Run commands from each changed module. To verify all modules from the root:

```bash
for module in app auth captcha dbx heartbeat log passwd rdb req rsp vault web; do
  (cd "$module" && go test ./...)
done
```

## Coding and Testing Conventions

- Format Go with `gofmt`; exported identifiers use `PascalCase`, internal identifiers use `camelCase`, and packages use short lowercase names.
- Bind SQL parameters; never concatenate user input into SQL.
- Use structured Zap fields and preserve request context.
- Use fake repositories for service tests, in-memory SQLite for repository tests, and `httptest` with fake services for handlers.
- Cover success, validation, authorization, persistence errors, and concurrency-sensitive behavior as applicable. Every behavior change requires focused regression coverage.

## Configuration and Security

- Start from `web-be/config.example.yaml` or `web-be/config.example.json`.
- Never commit `.env`, runtime configuration, credentials, tokens, or database files.
- Keep Header-only Bearer sessions enabled.
- Bind metrics and pprof only to loopback interfaces.
- Keep Swagger disabled unless explicitly required.
- Treat `/health` as liveness and `/ready` as dependency readiness.
