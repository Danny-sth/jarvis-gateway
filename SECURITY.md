# Security Policy

## Supported Versions

| Version | Supported          |
| ------- | ------------------ |
| 1.0.x   | :white_check_mark: |

## Reporting a Vulnerability

If you discover a security vulnerability, please report it privately:

1. **Do NOT** create a public GitHub issue
2. Email the maintainer directly with details
3. Include steps to reproduce the vulnerability
4. Allow up to 48 hours for initial response

## Security Features

### Authentication
- Keycloak OIDC (RS256 + JWKS)
- JWT Auth (HS256)
- Mobile session tokens
- BasicAuth for admin endpoints

### Rate Limiting
- Telegram webhooks: 60 req/min per IP
- General webhooks: 30 req/min per IP
- Auth endpoints: 10 req/min per IP

### CSRF Protection
- Token-based CSRF for forms
- Bypass for Bearer token APIs and webhooks

### Security Headers
- HSTS, X-Frame-Options, CSP, X-Content-Type-Options

### Webhook Security
- GitHub: HMAC-SHA256 signature verification

## Audit History

- 2026-04: Added Security Headers and CSRF middleware
