package startup

import (
	"errors"
	"fmt"
	"log"
	"net"
	"strings"

	"go.uber.org/zap"
)

// DependencyError describes a missing or unreachable required service.
type DependencyError struct {
	Service string
	Detail  string
	Fix     string
}

func (e *DependencyError) Error() string {
	return fmt.Sprintf("%s: %s", e.Service, e.Detail)
}

// Format returns a multi-line message suitable for terminal output.
func Format(err error) string {
	var de *DependencyError
	if errors.As(err, &de) {
		return fmt.Sprintf(
			"Thiếu thành phần: %s\n  • %s\n\nCách xử lý:\n  %s",
			de.Service,
			de.Detail,
			strings.ReplaceAll(de.Fix, "\n", "\n  "),
		)
	}
	return err.Error()
}

// Fatal logs a clear startup failure and exits.
func Fatal(logger *zap.Logger, headline string, err error) {
	msg := Format(err)
	if logger != nil {
		logger.Fatal(headline, zap.String("detail", msg))
	}
	log.Fatalf("\n[FATAL] %s\n\n%s\n", headline, msg)
}

func isConnRefused(err error) bool {
	if err == nil {
		return false
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return strings.Contains(strings.ToLower(opErr.Err.Error()), "connection refused")
	}
	return strings.Contains(strings.ToLower(err.Error()), "connection refused")
}

func hostPort(addr, fallbackHost string, fallbackPort string) (host, port string) {
	host = fallbackHost
	port = fallbackPort
	if addr == "" {
		return host, port
	}
	if h, p, err := net.SplitHostPort(addr); err == nil {
		if h != "" {
			host = h
		}
		if p != "" {
			port = p
		}
	}
	return host, port
}

// WrapPostgres turns pgx dial/ping errors into actionable messages.
func WrapPostgres(err error, dsn string) error {
	if err == nil {
		return nil
	}
	_, port := hostPort("", "localhost", "15432")
	if strings.Contains(dsn, ":") {
		// best-effort parse postgres://user:pass@host:port/db
		if i := strings.LastIndex(dsn, "@"); i >= 0 {
			rest := dsn[i+1:]
			if j := strings.Index(rest, "/"); j >= 0 {
				rest = rest[:j]
			}
			if h, p, e := net.SplitHostPort(rest); e == nil {
				if h != "" {
					_ = h
				}
				if p != "" {
					port = p
				}
			}
		}
	}
	detail := err.Error()
	if isConnRefused(err) {
		detail = fmt.Sprintf("không kết nối được Postgres (localhost:%s — connection refused)", port)
	}
	return &DependencyError{
		Service: "PostgreSQL",
		Detail:  detail,
		Fix: strings.TrimSpace(`
Chạy hạ tầng Docker: make infra-up
Đợi Postgres sẵn sàng rồi migrate: make migrate-up
Hoặc một lệnh: make dev-full (infra + migrate + gateway)
Kiểm tra container: docker ps | grep mediahub-postgres
Logs: docker logs mediahub-postgres`),
	}
}

// WrapRedis turns Redis ping errors into actionable messages.
func WrapRedis(err error, addr string) error {
	if err == nil {
		return nil
	}
	host, port := hostPort(addr, "localhost", "16379")
	detail := err.Error()
	if isConnRefused(err) {
		detail = fmt.Sprintf("không kết nối được Redis (%s:%s — connection refused)", host, port)
	}
	return &DependencyError{
		Service: "Redis",
		Detail:  detail,
		Fix: strings.TrimSpace(`
Chạy hạ tầng Docker: make infra-up
Kiểm tra container: docker ps | grep mediahub-redis
Logs: docker logs mediahub-redis`),
	}
}

// WrapMinIO turns S3/MinIO errors into actionable messages.
func WrapMinIO(err error, endpoint string) error {
	if err == nil {
		return nil
	}
	detail := err.Error()
	if isConnRefused(err) {
		detail = fmt.Sprintf("không kết nối được MinIO (%s — connection refused)", endpoint)
	}
	return &DependencyError{
		Service: "MinIO (object storage)",
		Detail:  detail,
		Fix: strings.TrimSpace(`
Chạy hạ tầng Docker: make infra-up
Kiểm tra container: docker ps | grep mediahub-minio
Console: http://localhost:9001
Logs: docker logs mediahub-minio`),
	}
}
