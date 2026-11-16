# Logging and observability

The web UI and token manager configure Go's `log/slog` JSON handler at startup.
Existing standard-library `log` calls are redirected through that handler, so
application logs are JSON records on stdout. The web UI also emits one structured
request summary with the method, matched route, response status, and duration. It
does not record query strings or authenticated user names.

Azure Container Apps collects container stdout and stderr. The destination
(Log Analytics, Azure Monitor, or none) is selected on the Container Apps
environment; this repository does not configure it. See the
[Container Apps logging options](https://learn.microsoft.com/azure/container-apps/log-options).

OpenTelemetry trace export is not configured by this change. Enabling it requires
an OTLP endpoint and an exporter/SDK; logs continue to be collected through the
existing container console path.
